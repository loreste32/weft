package accelerator

import (
	"math"
	"os"
	"testing"

	"github.com/loreste/weft/internal/tensor"
)

func TestExternalProviderTensor(t *testing.T) {
	path := os.Getenv("WEFT_ACCELERATOR_PLUGIN")
	if path == "" {
		t.Skip("WEFT_ACCELERATOR_PLUGIN is not configured")
	}
	plugin, err := Load(path)
	if err != nil {
		t.Fatal("load external provider: ", err)
	}
	defer func() { _ = plugin.Close() }()
	if !supportsOperation(plugin.Manifest().Operations, "tensor_matmul") {
		t.Fatalf("provider %q does not declare tensor_matmul: %+v", plugin.Manifest().Name, plugin.Manifest())
	}
	left, err := tensor.FromFloat64([]int{2, 2}, []float64{1, 2, 3, 4})
	if err != nil {
		t.Fatal(err)
	}
	right, err := tensor.FromFloat64([]int{2, 2}, []float64{5, 6, 7, 8})
	if err != nil {
		t.Fatal(err)
	}
	result, info, err := plugin.RunTensor("tensor_matmul", left, right)
	if err != nil {
		t.Fatal("external tensor matmul: ", err)
	}
	// Execution reporting is mandatory for conformance: a provider that omits
	// weft_accel_exec_info, or reports a contradiction, fails here.
	if !info.Reported {
		t.Fatalf("REPORTING_UNREPORTED: provider %q omits tensor exec info", plugin.Manifest().Name)
	}
	if info.Fallback && info.Device != "cpu" {
		t.Fatalf("REPORTING_CONTRADICTORY: tensor_matmul claims fallback on non-cpu device: %+v", info)
	}
	if !info.Fallback && info.RequestedDevice != "" && info.Device != info.RequestedDevice {
		t.Fatalf("REPORTING_CONTRADICTORY: tensor_matmul device %q != requested %q without fallback",
			info.Device, info.RequestedDevice)
	}
	values, err := result.Float64Values()
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []float64{19, 22, 43, 50} {
		if math.Abs(values[i]-want) > 1e-4 {
			t.Fatalf("external tensor result[%d] = %v, want %v", i, values[i], want)
		}
	}
}
