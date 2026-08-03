package accelerator

import (
	"encoding/json"
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
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(health, &healthValue); err != nil || !healthValue.OK {
		t.Fatalf("health result = %s, %v", health, err)
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
		Data  []float64 `json:"data"`
		Shape []int     `json:"shape"`
	}
	if err := json.Unmarshal(matmul, &matrix); err != nil {
		t.Fatalf("matmul JSON = %s: %v", matmul, err)
	}
	if len(matrix.Shape) != 2 || matrix.Shape[0] != 2 || matrix.Shape[1] != 2 ||
		len(matrix.Data) != 4 || matrix.Data[0] != 19 || matrix.Data[1] != 22 ||
		matrix.Data[2] != 43 || matrix.Data[3] != 50 {
		t.Fatalf("matmul result = %+v", matrix)
	}
}
