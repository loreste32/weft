package pkgman

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// RegistryServer is a simple HTTP package registry.
// Stores packages as files on disk: dataDir/packages/<name>-<version>.tar.gz
// Index rebuilt from metadata files: dataDir/packages/<name>-<version>.json
type RegistryServer struct {
	DataDir string
	Token   string // if non-empty, required for publish (Bearer token)
	mu      sync.RWMutex
	index   *RegistryIndex // cached
}

var validVersion = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.]+)?$`)

// NewRegistryServer creates a registry backed by dataDir.
func NewRegistryServer(dataDir, token string) *RegistryServer {
	return &RegistryServer{DataDir: dataDir, Token: token}
}

// Handler returns an http.Handler for the registry API.
func (s *RegistryServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/index.json", s.handleIndex)
	mux.HandleFunc("/v1/namespaces.json", s.handleNamespaces)
	mux.HandleFunc("/v1/namespaces/", s.handleNamespaceKeys) // GET list / POST add key
	mux.HandleFunc("/v1/publish", s.handlePublish)
	mux.HandleFunc("/v1/packages/", s.handleDownload)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok\n"))
	})
	return mux
}

func (s *RegistryServer) packagesDir() string {
	return filepath.Join(s.DataDir, "packages")
}

func (s *RegistryServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", 405)
		return
	}
	idx, err := s.loadIndex()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(idx)
}

func (s *RegistryServer) handlePublish(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}

	// Auth — always required
	if s.Token == "" {
		http.Error(w, "registry not configured for publishing (no token set)", 503)
		return
	}
	auth := r.Header.Get("Authorization")
	if auth != "Bearer "+s.Token {
		http.Error(w, "unauthorized — set WEFT_REGISTRY_TOKEN or pass --token", 401)
		return
	}

	// Limit upload size
	r.Body = http.MaxBytesReader(w, r.Body, MaxArchiveBytes+4<<20)

	if err := r.ParseMultipartForm(MaxArchiveBytes + 4<<20); err != nil {
		http.Error(w, "bad request: "+err.Error(), 400)
		return
	}

	metaStr := r.FormValue("metadata")
	if metaStr == "" {
		http.Error(w, "missing metadata field", 400)
		return
	}
	var meta RegistryPackage
	if err := json.Unmarshal([]byte(metaStr), &meta); err != nil {
		http.Error(w, "bad metadata: "+err.Error(), 400)
		return
	}

	// Validate name format
	if !validPkgName.MatchString(meta.Name) {
		http.Error(w, "invalid package name: must match [a-z][a-z0-9_-]{0,63}", 400)
		return
	}
	// Validate version format
	if !validVersion.MatchString(meta.Version) {
		http.Error(w, "invalid version: must be semver (e.g. 1.0.0)", 400)
		return
	}

	// Reject stdlib name collisions
	if IsPackageReserved(meta.Name) {
		http.Error(w, fmt.Sprintf("package name %q is reserved (conflicts with stdlib or prelude)", meta.Name), 409)
		return
	}

	// Require signature — unsigned packages are rejected
	if meta.PublicKey == "" || meta.Signature == "" {
		http.Error(w, "signature required: publish with --key <name> (run weft registry keygen first)", 400)
		return
	}

	// Read archive
	file, _, err := r.FormFile("archive")
	if err != nil {
		http.Error(w, "missing archive file: "+err.Error(), 400)
		return
	}
	defer file.Close()

	archiveData, err := io.ReadAll(io.LimitReader(file, MaxArchiveBytes))
	if err != nil {
		http.Error(w, "read archive failed: "+err.Error(), 400)
		return
	}

	// Verify signature matches the uploaded archive
	if err := Verify(meta.PublicKey, archiveData, meta.Signature); err != nil {
		http.Error(w, "signature verification failed: "+err.Error(), 403)
		return
	}

	// Verify checksum if provided
	if meta.Sum != "" {
		got := "sha256:" + hex.EncodeToString(sha256sum(archiveData))
		if got != meta.Sum {
			http.Error(w, fmt.Sprintf("checksum mismatch: expected %s, got %s", meta.Sum, got), 400)
			return
		}
	}

	// Prevent version overwrite — immutable once published (before key checks so 409 wins)
	pkgDir := s.packagesDir()
	os.MkdirAll(pkgDir, 0o755)
	archiveName := fmt.Sprintf("%s-%s.tar.gz", meta.Name, meta.Version)
	archivePath := filepath.Join(pkgDir, archiveName)
	metaPath := filepath.Join(pkgDir, fmt.Sprintf("%s-%s.json", meta.Name, meta.Version))

	if _, err := os.Stat(metaPath); err == nil {
		http.Error(w, fmt.Sprintf("%s@%s already published — versions are immutable", meta.Name, meta.Version), 409)
		return
	}

	// Namespace ownership: first publisher pins the namespace key; later packages must match.
	if err := s.enforceNamespaceKey(meta.Name, meta.PublicKey); err != nil {
		http.Error(w, err.Error(), 403)
		return
	}

	// Save archive
	if err := os.WriteFile(archivePath, archiveData, 0o644); err != nil {
		http.Error(w, "save failed: "+err.Error(), 500)
		return
	}

	// Set archive URL and timestamp
	meta.ArchiveURL = "v1/packages/" + archiveName
	if meta.Published == "" {
		meta.Published = time.Now().UTC().Format(time.RFC3339)
	}

	// Compute checksum if not provided
	if meta.Sum == "" {
		meta.Sum = "sha256:" + hex.EncodeToString(sha256sum(archiveData))
	}

	// Save metadata
	mb, _ := json.MarshalIndent(meta, "", "  ")
	if err := os.WriteFile(metaPath, append(mb, '\n'), 0o644); err != nil {
		os.Remove(archivePath) // clean up
		http.Error(w, "save metadata failed: "+err.Error(), 500)
		return
	}

	// Invalidate cached index
	s.mu.Lock()
	s.index = nil
	s.mu.Unlock()

	w.WriteHeader(201)
	fmt.Fprintf(w, "published %s@%s (signed, verified)\n", meta.Name, meta.Version)
}

func (s *RegistryServer) handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", 405)
		return
	}
	name := strings.TrimPrefix(r.URL.Path, "/v1/packages/")
	// Sanitize
	if strings.Contains(name, "..") || strings.Contains(name, "/") {
		http.Error(w, "bad path", 400)
		return
	}
	path := filepath.Join(s.packagesDir(), name)
	if _, err := os.Stat(path); err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, path)
}

// NamespaceRecord is one owned namespace in /v1/namespaces.json.
type NamespaceRecord struct {
	Namespace  string   `json:"namespace"`
	PublicKeys []string `json:"public_keys"`
	Packages   []string `json:"packages,omitempty"`
}

// namespaceFile is the on-disk pin for allowed signing keys (supports rotation).
type namespaceFile struct {
	Namespace  string   `json:"namespace"`
	PublicKeys []string `json:"public_keys"`
	Updated    string   `json:"updated,omitempty"`
}

func (s *RegistryServer) namespacesDir() string {
	return filepath.Join(s.DataDir, "namespaces")
}

func (s *RegistryServer) loadNamespaceFile(ns string) (*namespaceFile, error) {
	path := filepath.Join(s.namespacesDir(), ns+".json")
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var nf namespaceFile
	if err := json.Unmarshal(b, &nf); err != nil {
		return nil, err
	}
	return &nf, nil
}

func (s *RegistryServer) saveNamespaceFile(nf *namespaceFile) error {
	if err := os.MkdirAll(s.namespacesDir(), 0o755); err != nil {
		return err
	}
	nf.Updated = time.Now().UTC().Format(time.RFC3339)
	b, err := json.MarshalIndent(nf, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.namespacesDir(), nf.Namespace+".json"), append(b, '\n'), 0o644)
}

// handleNamespaces lists namespace → keys (disk pins + packages).
func (s *RegistryServer) handleNamespaces(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", 405)
		return
	}
	idx, err := s.loadIndex()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	type acc struct {
		keys map[string]bool
		pkgs map[string]bool
	}
	m := map[string]*acc{}
	// disk pins first
	if ents, err := os.ReadDir(s.namespacesDir()); err == nil {
		for _, e := range ents {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			ns := strings.TrimSuffix(e.Name(), ".json")
			nf, err := s.loadNamespaceFile(ns)
			if err != nil || nf == nil {
				continue
			}
			a := &acc{keys: map[string]bool{}, pkgs: map[string]bool{}}
			for _, k := range nf.PublicKeys {
				a.keys[strings.ToLower(k)] = true
			}
			m[ns] = a
		}
	}
	for _, p := range idx.Packages {
		ns := PackageNamespace(p.Name)
		a := m[ns]
		if a == nil {
			a = &acc{keys: map[string]bool{}, pkgs: map[string]bool{}}
			m[ns] = a
		}
		if p.PublicKey != "" {
			a.keys[strings.ToLower(p.PublicKey)] = true
		}
		a.pkgs[p.Name] = true
	}
	out := make([]NamespaceRecord, 0, len(m))
	for ns, a := range m {
		rec := NamespaceRecord{Namespace: ns}
		for k := range a.keys {
			rec.PublicKeys = append(rec.PublicKeys, k)
		}
		for p := range a.pkgs {
			rec.Packages = append(rec.Packages, p)
		}
		out = append(out, rec)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"namespaces": out})
}

// handleNamespaceKeys: POST /v1/namespaces/<ns>/keys  body {"public_key":"..."}
// Adds a signing key for rotation (requires registry token).
func (s *RegistryServer) handleNamespaceKeys(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/namespaces/")
	path = strings.Trim(path, "/")
	// expect "<ns>/keys" or "<ns>"
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "namespace required", 400)
		return
	}
	ns := parts[0]
	if strings.Contains(ns, "..") || strings.ContainsAny(ns, `/\`) {
		http.Error(w, "bad namespace", 400)
		return
	}

	if r.Method == "GET" {
		nf, err := s.loadNamespaceFile(ns)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		if nf == nil {
			// synthesize from packages
			idx, _ := s.loadIndex()
			nf = &namespaceFile{Namespace: ns}
			seen := map[string]bool{}
			for _, p := range idx.Packages {
				if PackageNamespace(p.Name) == ns && p.PublicKey != "" {
					k := strings.ToLower(p.PublicKey)
					if !seen[k] {
						nf.PublicKeys = append(nf.PublicKeys, k)
						seen[k] = true
					}
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(nf)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	if len(parts) < 2 || parts[1] != "keys" {
		http.Error(w, "use POST /v1/namespaces/<ns>/keys", 400)
		return
	}
	// Auth required for rotation
	if s.Token == "" {
		http.Error(w, "registry not configured for publishing (no token set)", 503)
		return
	}
	if r.Header.Get("Authorization") != "Bearer "+s.Token {
		http.Error(w, "unauthorized", 401)
		return
	}
	var body struct {
		PublicKey string `json:"public_key"`
		Retire    string `json:"retire,omitempty"` // optional old key to remove
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	pub := strings.ToLower(strings.TrimSpace(body.PublicKey))
	if pub == "" {
		http.Error(w, "public_key required", 400)
		return
	}
	if _, err := decodePubHex(pub); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	nf, err := s.loadNamespaceFile(ns)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if nf == nil {
		nf = &namespaceFile{Namespace: ns}
	}
	// add new key
	found := false
	for _, k := range nf.PublicKeys {
		if strings.EqualFold(k, pub) {
			found = true
			break
		}
	}
	if !found {
		nf.PublicKeys = append(nf.PublicKeys, pub)
	}
	// optional retire
	if body.Retire != "" {
		retire := strings.ToLower(strings.TrimSpace(body.Retire))
		var keys []string
		for _, k := range nf.PublicKeys {
			if !strings.EqualFold(k, retire) {
				keys = append(keys, k)
			}
		}
		if len(keys) == 0 {
			http.Error(w, "cannot retire last remaining key", 400)
			return
		}
		nf.PublicKeys = keys
	}
	if err := s.saveNamespaceFile(nf); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(200)
	json.NewEncoder(w).Encode(nf)
}

// enforceNamespaceKey allows any key listed for the namespace (disk pin and/or prior packages).
// First publish creates the pin. Rotation: POST /v1/namespaces/<ns>/keys then publish with new key.
func (s *RegistryServer) enforceNamespaceKey(pkgName, pubKey string) error {
	ns := PackageNamespace(pkgName)
	pubKey = strings.ToLower(strings.TrimSpace(pubKey))

	nf, err := s.loadNamespaceFile(ns)
	if err != nil {
		return err
	}
	allowed := map[string]bool{}
	if nf != nil {
		for _, k := range nf.PublicKeys {
			allowed[strings.ToLower(k)] = true
		}
	} else {
		// seed from existing packages
		idx, err := s.loadIndex()
		if err != nil {
			return err
		}
		for _, p := range idx.Packages {
			if PackageNamespace(p.Name) != ns || p.PublicKey == "" {
				continue
			}
			allowed[strings.ToLower(p.PublicKey)] = true
		}
	}

	if len(allowed) == 0 {
		// first key for namespace — pin it
		nf = &namespaceFile{Namespace: ns, PublicKeys: []string{pubKey}}
		return s.saveNamespaceFile(nf)
	}
	if allowed[pubKey] {
		// ensure disk pin exists
		if nf == nil {
			keys := make([]string, 0, len(allowed))
			for k := range allowed {
				keys = append(keys, k)
			}
			nf = &namespaceFile{Namespace: ns, PublicKeys: keys}
			_ = s.saveNamespaceFile(nf)
		}
		return nil
	}
	return fmt.Errorf("namespace %q does not allow this signing key — rotate with POST /v1/namespaces/%s/keys (Bearer token)", ns, ns)
}

func (s *RegistryServer) loadIndex() (*RegistryIndex, error) {
	s.mu.RLock()
	if s.index != nil {
		idx := s.index
		s.mu.RUnlock()
		return idx, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.index != nil {
		return s.index, nil
	}

	idx := &RegistryIndex{}
	pkgDir := s.packagesDir()
	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		if os.IsNotExist(err) {
			s.index = idx
			return idx, nil
		}
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(pkgDir, e.Name()))
		if err != nil {
			continue
		}
		var pkg RegistryPackage
		if err := json.Unmarshal(b, &pkg); err != nil {
			continue
		}
		if pkg.Name != "" && pkg.Version != "" {
			idx.Packages = append(idx.Packages, pkg)
		}
	}
	s.index = idx
	return idx, nil
}

// ListenAndServe starts the registry server.
func (s *RegistryServer) ListenAndServe(addr string) error {
	os.MkdirAll(s.packagesDir(), 0o755)
	fmt.Printf("registry listening on %s (data: %s)\n", addr, s.DataDir)
	return http.ListenAndServe(addr, s.Handler())
}

func sha256sum(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

// IsPackageReserved checks if a name conflicts with stdlib or language builtins.
func IsPackageReserved(name string) bool {
	reserved := map[string]bool{
		"map": true, "filter": true, "reduce": true, "say": true,
		"print": true, "len": true, "push": true, "range": true,
		"spawn": true, "channel": true, "send": true, "recv": true,
		"close": true, "parallel": true, "race": true, "timeout": true,
		"ok": true, "err": true, "ensure": true, "bail": true,
		"sort": true, "reverse": true, "unique": true, "find": true,
		"any": true, "all": true, "zip": true, "flatten": true,
		"each": true, "group": true, "gather": true, "enumerate": true,
	}
	if reserved[name] {
		return true
	}
	// Check against stdlib names
	stdlibNames := []string{
		"env", "json", "jsonl", "table", "pipe", "fs", "http", "web", "ws",
		"webrtc", "viz", "cli", "sh", "shlex", "signal", "binstruct", "difflib",
		"copy", "functools", "traceback", "io", "str", "csv", "time", "math",
		"random", "uuid", "base64", "url", "archive", "decimal", "xml", "html",
		"ini", "toml", "yaml", "mime", "email", "socket", "pickle", "iter",
		"collections", "platform", "ip", "bisect", "heap", "db", "redis",
		"nats", "amqp", "mongo", "graphql", "pcap", "tokenizer", "metrics",
		"dataset", "ratelimit", "migrate", "log", "crypto", "re", "test",
		"secrets", "llm", "ollama", "vllm", "sysinfo", "proc", "netutil",
	}
	for _, s := range stdlibNames {
		if name == s {
			return true
		}
	}
	return false
}
