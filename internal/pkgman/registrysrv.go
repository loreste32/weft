package pkgman

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
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
	// Auth
	if s.Token != "" {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer "+s.Token {
			http.Error(w, "unauthorized", 401)
			return
		}
	}

	// Limit upload size
	r.Body = http.MaxBytesReader(w, r.Body, MaxArchiveBytes+4<<20) // archive + metadata overhead

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
	if meta.Name == "" || meta.Version == "" {
		http.Error(w, "metadata requires name and version", 400)
		return
	}
	// Sanitize name/version
	if strings.ContainsAny(meta.Name, "/\\") || strings.Contains(meta.Name, "..") ||
		strings.ContainsAny(meta.Version, "/\\") || strings.Contains(meta.Version, "..") {
		http.Error(w, "invalid name or version", 400)
		return
	}

	file, _, err := r.FormFile("archive")
	if err != nil {
		http.Error(w, "missing archive file: "+err.Error(), 400)
		return
	}
	defer file.Close()

	// Save archive
	pkgDir := s.packagesDir()
	os.MkdirAll(pkgDir, 0o755)

	archiveName := fmt.Sprintf("%s-%s.tar.gz", meta.Name, meta.Version)
	archivePath := filepath.Join(pkgDir, archiveName)
	out, err := os.Create(archivePath)
	if err != nil {
		http.Error(w, "save failed: "+err.Error(), 500)
		return
	}
	if _, err := io.Copy(out, io.LimitReader(file, MaxArchiveBytes)); err != nil {
		out.Close()
		os.Remove(archivePath)
		http.Error(w, "save failed: "+err.Error(), 500)
		return
	}
	out.Close()

	// Set archive URL
	meta.ArchiveURL = "v1/packages/" + archiveName
	if meta.Published == "" {
		meta.Published = time.Now().UTC().Format(time.RFC3339)
	}

	// Save metadata
	metaPath := filepath.Join(pkgDir, fmt.Sprintf("%s-%s.json", meta.Name, meta.Version))
	mb, _ := json.MarshalIndent(meta, "", "  ")
	if err := os.WriteFile(metaPath, append(mb, '\n'), 0o644); err != nil {
		http.Error(w, "save metadata failed: "+err.Error(), 500)
		return
	}

	// Invalidate cached index
	s.mu.Lock()
	s.index = nil
	s.mu.Unlock()

	w.WriteHeader(201)
	fmt.Fprintf(w, "published %s@%s\n", meta.Name, meta.Version)
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

	// double-check
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
