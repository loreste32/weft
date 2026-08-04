// Package accelerator owns the small, optional native accelerator ABI.
//
// The main Weft binary deliberately does not link CUDA, ROCm, or MLX. Native
// providers are loaded only after an application grants the accelerator
// capability and explicitly names a shared library.
package accelerator

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/loreste/weft/internal/tensor"
)

const (
	// ABI is the stable version of the shared-library contract.
	ABI = 1
	// MaxResultBytes prevents a faulty provider from allocating unbounded data.
	MaxResultBytes = 256 << 20
	// MaxTensorInputs bounds descriptor allocation in native providers.
	MaxTensorInputs = 1024

	// EnvAllowlist is a colon/semicolon/comma-separated list of absolute
	// directories or exact shared-library paths that may be loaded.
	// When set, every load path must resolve under an allowlisted entry.
	EnvAllowlist = "WEFT_ACCELERATOR_ALLOWLIST"
	// EnvRequireChecksum forces SHA-256 verification for every load.
	// Accepts "1", "true", or "yes".
	EnvRequireChecksum = "WEFT_ACCELERATOR_REQUIRE_CHECKSUM"
	// EnvChecksum is the expected lowercase hex SHA-256 of the plugin file.
	// Used with a single load; prefer sidecar "<path>.sha256" for multiple.
	EnvChecksum = "WEFT_ACCELERATOR_CHECKSUM"
	// EnvDisablePlugins hard-disables all native accelerator loads.
	EnvDisablePlugins = "WEFT_ACCELERATOR_DISABLE"
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

type nativeTensorPlugin interface {
	runTensor(operation string, inputs []*tensor.Tensor) (*tensor.Tensor, error)
}

// nativeExecInfoPlugin is the optional additive ABI v1 export
// weft_accel_exec_info: a JSON document describing the most recent run.
// Providers that omit it load and run, but their tensor results report
// StatusUnreported and they fail conformance.
type nativeExecInfoPlugin interface {
	execInfo() ([]byte, error)
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
//
// Trust model (fail-closed when configured):
//   - WEFT_ACCELERATOR_DISABLE=1 rejects every load.
//   - WEFT_ACCELERATOR_ALLOWLIST restricts paths to listed files/directories.
//   - WEFT_ACCELERATOR_REQUIRE_CHECKSUM=1 (or a provided checksum/sidecar)
//     verifies the plugin bytes before dlopen.
//
// Native providers are trusted host code: they bypass the language sandbox.
// Registry packages cannot silently activate plugins; loads require an
// explicit path from application code with the accelerator capability.
func Load(path string) (*Plugin, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("accelerator plugin path is required")
	}
	if envTruthy(os.Getenv(EnvDisablePlugins)) {
		return nil, errors.New("accelerator plugins are disabled (WEFT_ACCELERATOR_DISABLE=1)")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve accelerator plugin: %w", err)
	}
	abs, err = filepath.EvalSymlinks(abs)
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
	if err := enforceAllowlist(abs); err != nil {
		return nil, err
	}
	if err := enforceChecksum(abs); err != nil {
		return nil, err
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

func envTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// enforceAllowlist rejects paths outside WEFT_ACCELERATOR_ALLOWLIST when set.
func enforceAllowlist(abs string) error {
	raw := strings.TrimSpace(os.Getenv(EnvAllowlist))
	if raw == "" {
		return nil
	}
	entries := splitPathList(raw)
	if len(entries) == 0 {
		return errors.New("WEFT_ACCELERATOR_ALLOWLIST is set but empty after parsing")
	}
	plugin := filepath.Clean(abs)
	if resolved, err := filepath.EvalSymlinks(plugin); err == nil {
		plugin = resolved
	}
	for _, entry := range entries {
		candidate, err := filepath.Abs(entry)
		if err != nil {
			continue
		}
		if resolved, err := filepath.EvalSymlinks(candidate); err == nil {
			candidate = resolved
		} else {
			// Fall back to cleaned absolute path when the entry does not exist yet.
			candidate = filepath.Clean(candidate)
		}
		info, err := os.Stat(candidate)
		if err != nil {
			if pathAllowed(plugin, candidate, false) {
				return nil
			}
			continue
		}
		if pathAllowed(plugin, candidate, info.IsDir()) {
			return nil
		}
	}
	return fmt.Errorf("accelerator plugin %q is outside WEFT_ACCELERATOR_ALLOWLIST", plugin)
}

func pathAllowed(plugin, allowed string, allowedIsDir bool) bool {
	if plugin == allowed {
		return true
	}
	if allowedIsDir {
		prefix := allowed
		if !strings.HasSuffix(prefix, string(os.PathSeparator)) {
			prefix += string(os.PathSeparator)
		}
		return strings.HasPrefix(plugin, prefix)
	}
	return false
}

func splitPathList(raw string) []string {
	replacer := strings.NewReplacer(";", string(os.PathListSeparator), ",", string(os.PathListSeparator))
	normalized := replacer.Replace(raw)
	parts := filepath.SplitList(normalized)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// enforceChecksum verifies plugin bytes when required or when a checksum is
// supplied via env or a "<path>.sha256" sidecar file.
func enforceChecksum(abs string) error {
	expected, source, err := expectedChecksum(abs)
	if err != nil {
		return err
	}
	require := envTruthy(os.Getenv(EnvRequireChecksum))
	if expected == "" {
		if require {
			return fmt.Errorf("accelerator checksum required (set %s or provide %s.sha256)", EnvChecksum, abs)
		}
		return nil
	}
	actual, err := fileSHA256(abs)
	if err != nil {
		return err
	}
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("accelerator plugin checksum mismatch (%s): got %s, want %s", source, actual, expected)
	}
	return nil
}

func expectedChecksum(abs string) (checksum, source string, err error) {
	if env := strings.TrimSpace(os.Getenv(EnvChecksum)); env != "" {
		sum, err := normalizeHexChecksum(env)
		if err != nil {
			return "", EnvChecksum, err
		}
		return sum, EnvChecksum, nil
	}
	sidecar := abs + ".sha256"
	data, err := os.ReadFile(sidecar)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", nil
		}
		return "", sidecar, fmt.Errorf("read accelerator checksum sidecar: %w", err)
	}
	line := strings.TrimSpace(string(data))
	if line == "" {
		return "", sidecar, fmt.Errorf("accelerator checksum sidecar %s is empty", sidecar)
	}
	// Accept "hex", "hex  filename", or "hex *filename".
	fields := strings.Fields(line)
	sum, err := normalizeHexChecksum(fields[0])
	if err != nil {
		return "", sidecar, err
	}
	return sum, sidecar, nil
}

func normalizeHexChecksum(value string) (string, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if len(value) != 64 {
		return "", fmt.Errorf("accelerator checksum must be 64 hex characters")
	}
	if _, err := hex.DecodeString(value); err != nil {
		return "", fmt.Errorf("accelerator checksum is not valid hex: %w", err)
	}
	return value, nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open accelerator plugin for checksum: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash accelerator plugin: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
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
	if len(m.Operations) == 0 {
		return errors.New("accelerator manifest must declare at least one operation")
	}
	if err := validateNames("vendor", m.Vendors); err != nil {
		return err
	}
	if err := validateNames("operation", m.Operations); err != nil {
		return err
	}
	return nil
}

func validateNames(kind string, values []string) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return fmt.Errorf("accelerator manifest contains an empty %s", kind)
		}
		if _, ok := seen[value]; ok {
			return fmt.Errorf("accelerator manifest contains duplicate %s %q", kind, value)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func (p *Plugin) Path() string { return p.path }

func (p *Plugin) Manifest() Manifest { return p.manifest }

// RunJSON executes one provider operation. The provider must return one JSON
// value; providers must not return pointers into caller-owned memory.
func (p *Plugin) RunJSON(op string, input []byte) ([]byte, error) {
	out, _, err := p.RunJSONEx(op, input)
	return out, err
}

// RunJSONEx is RunJSON plus execution reporting: the device, requested
// device, and fallback fields are parsed from the provider result. A result
// without those fields yields ExecInfo with Reported=false and
// StatusUnreported; a contradictory report (device other than requested
// without fallback) is an error.
func (p *Plugin) RunJSONEx(op string, input []byte) ([]byte, ExecInfo, error) {
	if p == nil || p.native == nil {
		return nil, UnavailableExecInfo(), errors.New("accelerator plugin is not loaded")
	}
	if strings.TrimSpace(op) == "" {
		return nil, ExecInfo{Status: StatusUnreported}, errors.New("accelerator operation is required")
	}
	if !supportsOperation(p.manifest.Operations, op) {
		return nil, ExecInfo{Status: StatusUnreported}, fmt.Errorf("accelerator operation %q is not declared by the provider", op)
	}
	if len(input) > MaxResultBytes {
		return nil, ExecInfo{Status: StatusUnreported}, fmt.Errorf("accelerator input exceeds %d bytes", MaxResultBytes)
	}
	out, err := p.native.run(op, input)
	if err != nil {
		return nil, ExecInfo{Status: StatusUnreported}, fmt.Errorf("accelerator %s: %w", op, err)
	}
	if len(out) > MaxResultBytes {
		return nil, ExecInfo{Status: StatusUnreported}, fmt.Errorf("accelerator result exceeds %d bytes", MaxResultBytes)
	}
	if !json.Valid(out) {
		return nil, ExecInfo{Status: StatusUnreported}, errors.New("accelerator returned invalid JSON")
	}
	info := parseExecInfo(out)
	if err := validateExecInfo(p.manifest.Name, info); err != nil {
		return nil, info, err
	}
	return out, info, nil
}

// RunTensor executes an optional binary tensor operation. Providers must
// advertise the operation in their manifest and implement the binary ABI;
// JSON-only providers fail explicitly instead of silently copying large
// tensors through the compatibility path.
//
// The returned ExecInfo comes from the provider's additive
// weft_accel_exec_info export. A provider without the export (or with a
// malformed document) yields Reported=false and StatusUnreported; a
// contradictory report is an error.
func (p *Plugin) RunTensor(op string, inputs ...*tensor.Tensor) (output *tensor.Tensor, info ExecInfo, err error) {
	unreported := ExecInfo{Status: StatusUnreported}
	if p == nil || p.native == nil {
		return nil, UnavailableExecInfo(), errors.New("accelerator plugin is not loaded")
	}
	if strings.TrimSpace(op) == "" {
		return nil, unreported, errors.New("accelerator operation required")
	}
	if !supportsOperation(p.manifest.Operations, op) {
		return nil, unreported, fmt.Errorf("accelerator operation %q is not declared by provider", op)
	}
	if len(inputs) == 0 {
		return nil, unreported, errors.New("accelerator tensor operation requires at least one input")
	}
	if len(inputs) > MaxTensorInputs {
		return nil, unreported, fmt.Errorf("accelerator tensor operation accepts at most %d inputs", MaxTensorInputs)
	}
	prepared := make([]*tensor.Tensor, len(inputs))
	temporary := make([]*tensor.Tensor, 0, len(inputs))
	// Contiguous views are pooled allocations owned by this call. Release them
	// on every exit path, while preserving an output that a provider may
	// deliberately return as an input alias.
	defer func() {
		for _, candidate := range temporary {
			if candidate != output {
				tensor.Release(candidate)
			}
		}
	}()
	for i, input := range inputs {
		if input == nil {
			return nil, unreported, errors.New("accelerator tensor operation received a nil input")
		}
		contiguous, err := input.Contiguous()
		if err != nil {
			return nil, unreported, fmt.Errorf("prepare accelerator tensor input %d: %w", i, err)
		}
		if contiguous != input {
			temporary = append(temporary, contiguous)
		}
		if contiguous.ByteLen() > MaxResultBytes {
			return nil, unreported, fmt.Errorf("accelerator tensor input exceeds %d bytes", MaxResultBytes)
		}
		prepared[i] = contiguous
	}
	provider, ok := p.native.(nativeTensorPlugin)
	if !ok {
		return nil, unreported, errors.New("accelerator provider does not implement binary tensor ABI")
	}
	output, err = provider.runTensor(op, prepared)
	if err != nil {
		return nil, unreported, fmt.Errorf("accelerator tensor %s: %w", op, err)
	}
	if output == nil {
		return nil, unreported, errors.New("accelerator tensor operation returned no result")
	}
	if output.ByteLen() > MaxResultBytes {
		tensor.Release(output)
		return nil, unreported, fmt.Errorf("accelerator tensor result exceeds %d bytes", MaxResultBytes)
	}
	info = unreported
	if reporter, ok := p.native.(nativeExecInfoPlugin); ok {
		if raw, rerr := reporter.execInfo(); rerr == nil {
			info = parseExecInfo(raw)
		}
	}
	if verr := validateExecInfo(p.manifest.Name, info); verr != nil {
		tensor.Release(output)
		return nil, info, verr
	}
	return output, info, nil
}

func supportsOperation(operations []string, operation string) bool {
	for _, declared := range operations {
		if declared == operation {
			return true
		}
	}
	return false
}

func (p *Plugin) Close() error {
	if p == nil || p.native == nil {
		return nil
	}
	err := p.native.close()
	p.native = nil
	return err
}
