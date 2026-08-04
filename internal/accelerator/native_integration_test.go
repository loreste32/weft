package accelerator

import (
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/loreste/weft/internal/tensor"
)

func TestExampleSharedLibrary(t *testing.T) {
	if !Supported() {
		t.Skip("shared-library loading is unavailable on this platform")
	}
	compiler, err := exec.LookPath("cc")
	if err != nil {
		t.Fatal("native accelerator CI requires a C compiler: ", err)
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	nativeDir := filepath.Join(root, "native", "accelerator")
	ext := ".so"
	flags := []string{"-shared", "-fPIC"}
	if runtime.GOOS == "darwin" {
		ext = ".dylib"
		flags = []string{"-dynamiclib", "-fPIC"}
	}
	output := filepath.Join(t.TempDir(), "weft-example"+ext)
	args := append(flags, "-I", nativeDir, filepath.Join(nativeDir, "example.c"), "-o", output)
	if result := exec.Command(compiler, args...).Run(); result != nil {
		t.Fatalf("compile example provider: %v", result)
	}

	plugin, err := Load(output)
	if err != nil {
		t.Fatal("load example provider: ", err)
	}
	defer plugin.Close()
	if plugin.Manifest().Name != "weft-example" || plugin.Manifest().ABI != ABI {
		t.Fatalf("unexpected manifest: %+v", plugin.Manifest())
	}
	health, healthInfo, err := plugin.RunJSONEx("health", []byte(`{}`))
	if err != nil {
		t.Fatal("health: ", err)
	}
	var healthValue struct {
		OK       bool   `json:"ok"`
		Backend  string `json:"backend"`
		Device   string `json:"device"`
		Fallback *bool  `json:"fallback"`
	}
	if err := json.Unmarshal(health, &healthValue); err != nil || !healthValue.OK {
		t.Fatalf("health result = %s, %v", health, err)
	}
	// Fallback reporting is required: CPU reference must declare device + no silent fallback.
	if healthValue.Fallback == nil {
		t.Fatalf("health missing fallback flag: %s", health)
	}
	if *healthValue.Fallback {
		t.Fatalf("CPU reference health reported fallback=true: %s", health)
	}
	if healthValue.Device != "cpu" {
		t.Fatalf("CPU reference health device = %q, want cpu: %s", healthValue.Device, health)
	}
	if !healthInfo.Reported || healthInfo.Status != StatusDevice || healthInfo.Device != "cpu" ||
		healthInfo.Fallback {
		t.Fatalf("health exec info = %+v, want reported cpu device run", healthInfo)
	}
	identity, err := plugin.RunJSON("identity", []byte(`{"value":7}`))
	if err != nil || string(identity) != `{"value":7}` {
		t.Fatalf("identity result = %s, %v", identity, err)
	}
	matmul, matmulInfo, err := plugin.RunJSONEx("matmul", []byte(`{"a":[1,2,3,4],"a_shape":[2,2],"b":[5,6,7,8],"b_shape":[2,2]}`))
	if err != nil {
		t.Fatal("matmul: ", err)
	}
	var matrix struct {
		Data     []float64 `json:"data"`
		Shape    []int     `json:"shape"`
		Device   string    `json:"device"`
		Fallback *bool     `json:"fallback"`
	}
	if err := json.Unmarshal(matmul, &matrix); err != nil {
		t.Fatalf("matmul JSON = %s: %v", matmul, err)
	}
	if len(matrix.Shape) != 2 || matrix.Shape[0] != 2 || matrix.Shape[1] != 2 ||
		len(matrix.Data) != 4 || matrix.Data[0] != 19 || matrix.Data[1] != 22 ||
		matrix.Data[2] != 43 || matrix.Data[3] != 50 {
		t.Fatalf("matmul result = %+v", matrix)
	}
	if matrix.Fallback == nil {
		t.Fatalf("matmul missing fallback flag: %s", matmul)
	}
	if *matrix.Fallback {
		t.Fatalf("CPU reference matmul reported fallback=true: %s", matmul)
	}
	if !matmulInfo.Reported || matmulInfo.Status != StatusDevice || matmulInfo.Device != "cpu" ||
		matmulInfo.RequestedDevice != "cpu" || matmulInfo.Fallback {
		t.Fatalf("matmul exec info = %+v, want reported cpu device run", matmulInfo)
	}
}

// TestExternalProvider is the provider conformance test used by hardware
// runners. It exercises the same dlopen ABI as applications, while keeping
// vendor SDKs out of ordinary CI and allowing the provider path to be chosen
// by the workflow environment.
func TestExternalProvider(t *testing.T) {
	path := os.Getenv("WEFT_ACCELERATOR_PLUGIN")
	if path == "" {
		t.Skip("WEFT_ACCELERATOR_PLUGIN is not configured")
	}
	if !Supported() {
		t.Skip("shared-library loading unavailable on platform")
	}
	plugin, err := Load(path)
	if err != nil {
		t.Fatal("load external provider: ", err)
	}
	defer func() {
		if err := plugin.Close(); err != nil {
			t.Errorf("close external provider: %v", err)
		}
	}()
	manifest := plugin.Manifest()
	if !supportsOperation(manifest.Operations, "health") || !supportsOperation(manifest.Operations, "matmul") {
		t.Fatalf("provider %q must declare health and matmul operations: %+v", manifest.Name, manifest.Operations)
	}
	health, err := plugin.RunJSON("health", []byte(`{}`))
	if err != nil {
		t.Fatal("external health: ", err)
	}
	var healthValue struct {
		OK       bool   `json:"ok"`
		Device   string `json:"device"`
		Fallback *bool  `json:"fallback"`
		Backend  string `json:"backend"`
	}
	if err := json.Unmarshal(health, &healthValue); err != nil {
		t.Fatalf("decode external health %s: %v", health, err)
	}
	if !healthValue.OK {
		t.Fatalf("external provider reported unhealthy: %s", health)
	}
	// Device and fallback reporting are mandatory: a provider that omits them
	// fails conformance instead of passing silently.
	if healthValue.Device == "" {
		t.Fatalf("health missing device field: %s", health)
	}
	if healthValue.Fallback == nil {
		t.Fatalf("health missing fallback flag: %s", health)
	}
	if *healthValue.Fallback && healthValue.Device != "cpu" {
		t.Fatalf("health claims non-cpu device with fallback=true (silent fallback): %s", health)
	}
	if supportsOperation(manifest.Operations, "identity") {
		identity, err := plugin.RunJSON("identity", []byte(`{"probe":1}`))
		if err != nil || string(identity) != `{"probe":1}` {
			t.Fatalf("external identity = %s, %v", identity, err)
		}
	}
	result, err := plugin.RunJSON("matmul", []byte(`{"a":[1,2,3,4],"a_shape":[2,2],"b":[5,6,7,8],"b_shape":[2,2]}`))
	if err != nil {
		t.Fatal("external matmul: ", err)
	}
	var matrix struct {
		Data     []float64 `json:"data"`
		Shape    []int     `json:"shape"`
		Device   string    `json:"device"`
		Fallback *bool     `json:"fallback"`
	}
	if err := json.Unmarshal(result, &matrix); err != nil {
		t.Fatalf("decode external matmul %s: %v", result, err)
	}
	if len(matrix.Shape) != 2 || matrix.Shape[0] != 2 || matrix.Shape[1] != 2 || len(matrix.Data) != 4 {
		t.Fatalf("external matmul shape/data = %+v", matrix)
	}
	want := []float64{19, 22, 43, 50}
	for i := range want {
		if math.Abs(matrix.Data[i]-want[i]) > 1e-5 {
			t.Fatalf("external matmul data[%d] = %v, want %v", i, matrix.Data[i], want[i])
		}
	}
	// Device and fallback reporting are mandatory for every provider under test.
	if matrix.Device == "" {
		t.Fatalf("matmul missing device field: %s", result)
	}
	if matrix.Fallback == nil {
		t.Fatalf("matmul missing fallback flag: %s", result)
	}
	if *matrix.Fallback && matrix.Device != "cpu" {
		t.Fatalf("matmul claims non-cpu device with fallback=true: %s", result)
	}
}

// TestExternalProviderReporting is the adversarial reporting conformance
// gate: every provider must truthfully report where each operation ran, on
// both the JSON and the binary tensor path. Failure messages carry
// REPORTING_UNREPORTED / REPORTING_CONTRADICTORY markers so the conformance
// script can classify the provider as "unreported" or "contradictory"
// instead of merely "failed".
func TestExternalProviderReporting(t *testing.T) {
	path := os.Getenv("WEFT_ACCELERATOR_PLUGIN")
	if path == "" {
		t.Skip("WEFT_ACCELERATOR_PLUGIN is not configured")
	}
	if !Supported() {
		t.Skip("shared-library loading unavailable on platform")
	}
	plugin, err := Load(path)
	if err != nil {
		t.Fatal("load external provider: ", err)
	}
	defer func() { _ = plugin.Close() }()
	manifest := plugin.Manifest()

	check := func(op string, info ExecInfo, err error) {
		t.Helper()
		if err != nil {
			if strings.Contains(err.Error(), "no fallback") || strings.Contains(err.Error(), "fallback=") {
				t.Fatalf("REPORTING_CONTRADICTORY: %s: %v", op, err)
			}
			t.Fatalf("%s: %v", op, err)
		}
		if !info.Reported {
			t.Fatalf("REPORTING_UNREPORTED: %s returned no device/fallback fields", op)
		}
		if info.Fallback && info.Device != "cpu" {
			t.Fatalf("REPORTING_CONTRADICTORY: %s claims fallback on non-cpu device %q", op, info.Device)
		}
		if !info.Fallback && info.RequestedDevice != "" && info.Device != info.RequestedDevice {
			t.Fatalf("REPORTING_CONTRADICTORY: %s device %q != requested %q without fallback",
				op, info.Device, info.RequestedDevice)
		}
	}

	_, healthInfo, err := plugin.RunJSONEx("health", []byte(`{}`))
	check("health", healthInfo, err)
	_, matmulInfo, err := plugin.RunJSONEx("matmul",
		[]byte(`{"a":[1,2,3,4],"a_shape":[2,2],"b":[5,6,7,8],"b_shape":[2,2]}`))
	check("matmul", matmulInfo, err)

	if supportsOperation(manifest.Operations, "tensor_matmul") {
		dtype := tensor.Float32
		values := []any{float32(1), float32(2), float32(3), float32(4)}
		other := []any{float32(5), float32(6), float32(7), float32(8)}
		if manifest.Metadata["dtype"] == "float64" {
			dtype = tensor.Float64
			values = []any{float64(1), float64(2), float64(3), float64(4)}
			other = []any{float64(5), float64(6), float64(7), float64(8)}
		}
		left, err := tensor.FromList(dtype, []int{2, 2}, values)
		if err != nil {
			t.Fatal(err)
		}
		right, err := tensor.FromList(dtype, []int{2, 2}, other)
		if err != nil {
			t.Fatal(err)
		}
		output, tensorInfo, err := plugin.RunTensor("tensor_matmul", left, right)
		if output != nil {
			defer tensor.Release(output)
		}
		check("tensor_matmul", tensorInfo, err)
	}
}
