package accelerator

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/loreste/weft/internal/tensor"
)

func TestExampleBinaryTensorAdd(t *testing.T) {
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
	if !supportsOperation(plugin.Manifest().Operations, "tensor_add") {
		t.Fatalf("reference provider does not tensor_add: %+v", plugin.Manifest())
	}

	left, err := tensor.FromFloat64([]int{2, 3}, []float64{1, 2, 3, 4, 5, 6})
	if err != nil {
		t.Fatal(err)
	}
	right, err := tensor.FromFloat64([]int{2, 3}, []float64{10, 20, 30, 40, 50, 60})
	if err != nil {
		t.Fatal(err)
	}
	result, err := plugin.RunTensor("tensor_add", left, right)
	if err != nil {
		t.Fatal("tensor add: ", err)
	}
	values, err := result.Float64Values()
	if err != nil {
		t.Fatal(err)
	}
	if result.DType() != tensor.Float64 || len(result.Shape()) != 2 ||
		result.Shape()[0] != 2 || result.Shape()[1] != 3 {
		t.Fatalf("tensor add metadata: dtype=%s shape=%v", result.DType(), result.Shape())
	}
	for i, want := range []float64{11, 22, 33, 44, 55, 66} {
		if values[i] != want {
			t.Fatalf("tensor add result[%d] = %v, want %v", i, values[i], want)
		}
	}

	wrongShape, err := tensor.FromFloat64([]int{3}, []float64{1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plugin.RunTensor("tensor_add", left, wrongShape); err == nil {
		t.Fatal("tensor_add accepted mismatched shapes")
	}
	wrongDType, err := tensor.FromList(tensor.Int64, []int{2, 3},
		[]any{int64(1), int64(2), int64(3), int64(4), int64(5), int64(6)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plugin.RunTensor("tensor_add", left, wrongDType); err == nil {
		t.Fatal("tensor_add accepted unsupported dtype")
	}
}
