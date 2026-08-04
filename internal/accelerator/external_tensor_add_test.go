package accelerator

import (
	"math"
	"os"
	"testing"

	"github.com/loreste/weft/internal/tensor"
)

func TestExternalProviderTensorAdd(t *testing.T) {
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
	if !supportsOperation(manifest.Operations, "tensor_add") {
		t.Fatalf("provider %q must declare tensor_add: %+v", manifest.Name, manifest.Operations)
	}

	dtype := tensor.Float32
	if manifest.Metadata["dtype"] == "float64" {
		dtype = tensor.Float64
	}
	left, err := tensor.FromList(dtype, []int{2, 3},
		[]any{float32(1), float32(2), float32(3), float32(4), float32(5), float32(6)})
	if dtype == tensor.Float64 {
		left, err = tensor.FromFloat64([]int{2, 3}, []float64{1, 2, 3, 4, 5, 6})
	}
	if err != nil {
		t.Fatal(err)
	}
	right, err := tensor.FromList(dtype, []int{2, 3},
		[]any{float32(10), float32(20), float32(30), float32(40), float32(50), float32(60)})
	if dtype == tensor.Float64 {
		right, err = tensor.FromFloat64([]int{2, 3}, []float64{10, 20, 30, 40, 50, 60})
	}
	if err != nil {
		t.Fatal(err)
	}
	result, err := plugin.RunTensor("tensor_add", left, right)
	if err != nil {
		t.Fatal("external tensor add: ", err)
	}
	values, err := result.Float64Values()
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []float64{11, 22, 33, 44, 55, 66} {
		if math.Abs(values[i]-want) > 1e-5 {
			t.Fatalf("external tensor add result[%d] = %v, want %v", i, values[i], want)
		}
	}
}
