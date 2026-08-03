package accelerator

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/loreste/weft/internal/tensor"
)

func TestExampleBinaryTensor(t *testing.T) {
	if !Supported() {
		t.Skip("shared-library loading unavailable on platform")
	}
	compiler, err := exec.LookPath("cc")
	if err != nil {
		t.Fatal("native accelerator CI requires C compiler: ", err)
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
	defer func() { _ = plugin.Close() }()
	if !supportsOperation(plugin.Manifest().Operations, "tensor_matmul") {
		t.Fatalf("reference provider does not declare tensor_matmul: %+v", plugin.Manifest())
	}
	leftBase, err := tensor.FromFloat64([]int{5}, []float64{99, 1, 2, 3, 4})
	if err != nil {
		t.Fatal(err)
	}
	left, err := leftBase.View([]int{2, 2}, []int64{2, 1}, 1)
	if err != nil {
		t.Fatal(err)
	}
	rightBase, err := tensor.FromFloat64([]int{5}, []float64{98, 5, 6, 7, 8})
	if err != nil {
		t.Fatal(err)
	}
	right, err := rightBase.View([]int{2, 2}, []int64{2, 1}, 1)
	if err != nil {
		t.Fatal(err)
	}
	result, err := plugin.RunTensor("tensor_matmul", left, right)
	if err != nil {
		t.Fatal("tensor matmul: ", err)
	}
	values, err := result.Float64Values()
	if err != nil {
		t.Fatal(err)
	}
	if result.DType() != tensor.Float64 || result.Shape()[0] != 2 || result.Shape()[1] != 2 {
		t.Fatalf("tensor result metadata: dtype=%s shape=%v", result.DType(), result.Shape())
	}
	for i, want := range []float64{19, 22, 43, 50} {
		if values[i] != want {
			t.Fatalf("tensor result[%d] = %v, want %v", i, values[i], want)
		}
	}
}
