// Package accelerator owns the small, optional native accelerator ABI.
//
// The main Weft binary deliberately does not link CUDA, ROCm, or MLX. Native
// providers are loaded only after an application grants the accelerator
// capability and explicitly names a shared library.
package accelerator

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// ABI is the stable version of the shared-library contract.
	ABI = 1
	// MaxResultBytes prevents a faulty provider from allocating unbounded data.
	MaxResultBytes = 256 << 20
)

var ErrUnavailable = errors.New("native accelerator plugins are unavailable in this build")

// Manifest is returned by weft_accel_manifest.
type Manifest struct {
	Name       string            `json:"name"`
	Version    string            `json:"version"`
	ABI        int               `json:"abi"`
	Vendors    []string          `json:"vendors"`
	Operations []string          `json:"operations"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type nativePlugin interface {
	manifest() ([]byte, error)
	run(op string, input []byte) ([]byte, error)
	close() error
}

// Plugin is a loaded, validated native provider.
type Plugin struct {
	path     string
	manifest Manifest
	native   nativePlugin
}

// Supported reports whether this host can load the shared-library ABI.
func Supported() bool { return nativeSupported() }

// Load validates and loads one explicitly selected provider.
func Load(path string) (*Plugin, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("accelerator plugin path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve accelerator plugin: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("stat accelerator plugin: %w", err)
	}
	if info.IsDir() {
		return nil, errors.New("accelerator plugin path must name a shared library")
	}
	native, err := loadNative(abs)
	if err != nil {
		return nil, err
	}
	raw, err := native.manifest()
	if err != nil {
		native.close()
		return nil, fmt.Errorf("read accelerator manifest: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		native.close()
		return nil, fmt.Errorf("accelerator manifest is not valid JSON: %w", err)
	}
	if err := validateManifest(manifest); err != nil {
		native.close()
		return nil, err
	}
	return &Plugin{path: abs, manifest: manifest, native: native}, nil
}

func validateManifest(m Manifest) error {
	if strings.TrimSpace(m.Name) == "" {
		return errors.New("accelerator manifest name is required")
	}
	if m.ABI != ABI {
		return fmt.Errorf("unsupported accelerator ABI %d (want %d)", m.ABI, ABI)
	}
	if strings.TrimSpace(m.Version) == "" {
		return errors.New("accelerator manifest version is required")
	}
	if len(m.Vendors) == 0 {
		return errors.New("accelerator manifest must declare at least one vendor")
	}
	return nil
}

func (p *Plugin) Path() string { return p.path }

func (p *Plugin) Manifest() Manifest { return p.manifest }

// RunJSON executes one provider operation. The provider must return one JSON
// value; providers must not return pointers into caller-owned memory.
func (p *Plugin) RunJSON(op string, input []byte) ([]byte, error) {
	if p == nil || p.native == nil {
		return nil, errors.New("accelerator plugin is not loaded")
	}
	if strings.TrimSpace(op) == "" {
		return nil, errors.New("accelerator operation is required")
	}
	if len(input) > MaxResultBytes {
		return nil, fmt.Errorf("accelerator input exceeds %d bytes", MaxResultBytes)
	}
	out, err := p.native.run(op, input)
	if err != nil {
		return nil, fmt.Errorf("accelerator %s: %w", op, err)
	}
	if len(out) > MaxResultBytes {
		return nil, fmt.Errorf("accelerator result exceeds %d bytes", MaxResultBytes)
	}
	if !json.Valid(out) {
		return nil, errors.New("accelerator returned invalid JSON")
	}
	return out, nil
}

func (p *Plugin) Close() error {
	if p == nil || p.native == nil {
		return nil
	}
	err := p.native.close()
	p.native = nil
	return err
}
