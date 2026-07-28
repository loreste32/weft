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

	// Prevent version overwrite — immutable once published
	pkgDir := s.packagesDir()
	os.MkdirAll(pkgDir, 0o755)
	archiveName := fmt.Sprintf("%s-%s.tar.gz", meta.Name, meta.Version)
	archivePath := filepath.Join(pkgDir, archiveName)
	metaPath := filepath.Join(pkgDir, fmt.Sprintf("%s-%s.json", meta.Name, meta.Version))

	if _, err := os.Stat(metaPath); err == nil {
		http.Error(w, fmt.Sprintf("%s@%s already published — versions are immutable", meta.Name, meta.Version), 409)
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
