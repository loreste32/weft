package accelerator

import (
	"encoding/json"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
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
	health, err := plugin.RunJSON("health", []byte(`{}`))
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
	if healthValue.Device != "" && healthValue.Device != "cpu" {
		t.Fatalf("CPU reference health device = %q, want cpu: %s", healthValue.Device, health)
	}
	identity, err := plugin.RunJSON("identity", []byte(`{"value":7}`))
	if err != nil || string(identity) != `{"value":7}` {
		t.Fatalf("identity result = %s, %v", identity, err)
	}
	matmul, err := plugin.RunJSON("matmul", []byte(`{"a":[1,2,3,4],"a_shape":[2,2],"b":[5,6,7,8],"b_shape":[2,2]}`))
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
	// Prefer explicit fallback reporting; require it when the key is present so
	// silent fallback cannot hide as a green health check.
	if healthValue.Fallback != nil && *healthValue.Fallback && healthValue.Device != "cpu" {
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
	// When providers report fallback, a true flag must not claim pure device exec.
	if matrix.Fallback != nil && *matrix.Fallback && matrix.Device != "" && matrix.Device != "cpu" {
		t.Fatalf("matmul claims non-cpu device with fallback=true: %s", result)
	}
}
