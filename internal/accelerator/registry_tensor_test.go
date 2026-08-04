package accelerator

import (
	"testing"

	"github.com/loreste/weft/internal/tensor"
)

type fakeTensorNative struct {
	inputs []*tensor.Tensor
}

func (f *fakeTensorNative) manifest() ([]byte, error) { return []byte(`{}`), nil }
func (f *fakeTensorNative) run(string, []byte) ([]byte, error) {
	return []byte(`{"ok":true}`), nil
}
func (f *fakeTensorNative) close() error { return nil }
func (f *fakeTensorNative) runTensor(_ string, inputs []*tensor.Tensor) (*tensor.Tensor, error) {
	f.inputs = inputs
	return tensor.FromFloat64([]int{2, 2}, []float64{19, 22, 43, 50})
}

func TestRunTensorReleasesTemporaryContiguousInputs(t *testing.T) {
	fake := &fakeTensorNative{}
	p := &Plugin{
		manifest: Manifest{
			Name:       "test",
			Version:    "1",
			ABI:        ABI,
			Vendors:    []string{"cpu"},
			Operations: []string{"tensor_matmul"},
		},
		native: fake,
	}
	leftBase, err := tensor.FromFloat64([]int{5}, []float64{99, 1, 2, 3, 4})
	if err != nil {
		t.Fatal(err)
	}
	defer tensor.Release(leftBase)
	rightBase, err := tensor.FromFloat64([]int{5}, []float64{98, 5, 6, 7, 8})
	if err != nil {
		t.Fatal(err)
	}
	defer tensor.Release(rightBase)
	left, err := leftBase.View([]int{2, 2}, []int64{2, 1}, 1)
	if err != nil {
		t.Fatal(err)
	}
	right, err := rightBase.View([]int{2, 2}, []int64{2, 1}, 1)
	if err != nil {
		t.Fatal(err)
	}

	result, info, err := p.RunTensor("tensor_matmul", left, right)
	if err != nil {
		t.Fatal(err)
	}
	defer tensor.Release(result)
	if info.Reported || info.Status != StatusUnreported {
		t.Fatalf("fake provider without exec_info export must be unreported, got %+v", info)
	}
	if len(fake.inputs) != 2 {
		t.Fatalf("provider received %d inputs, want 2", len(fake.inputs))
	}
	for i, input := range fake.inputs {
		if got := len(input.Bytes()); got != 0 {
			t.Fatalf("temporary input %d still owns %d bytes after RunTensor", i, got)
		}
	}
}
