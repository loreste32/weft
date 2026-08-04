package accelerator

import (
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/loreste/weft/internal/tensor"
)

// compileExampleProvider builds the CPU reference provider into a temporary
// shared library and returns its path.
func compileExampleProvider(t *testing.T) string {
	t.Helper()
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
	return output
}

func loadExampleProvider(t *testing.T) *Plugin {
	t.Helper()
	plugin, err := Load(compileExampleProvider(t))
	if err != nil {
		t.Fatal("load example provider: ", err)
	}
	t.Cleanup(func() { _ = plugin.Close() })
	return plugin
}

// makeProbeTensor builds a rank-len(shape) tensor of the requested float dtype.
func makeProbeTensor(t *testing.T, dtype tensor.DType, shape []int, values []float64) *tensor.Tensor {
	t.Helper()
	if dtype == tensor.Float32 {
		list := make([]any, len(values))
		for i, value := range values {
			list[i] = float32(value)
		}
		result, err := tensor.FromList(tensor.Float32, shape, list)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	result, err := tensor.FromFloat64(shape, values)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

// checkTensorExecInfo enforces the reporting contract on a binary tensor call:
// the provider must report, must not claim a fallback on a non-cpu device, and
// must not contradict the requested device. Markers let the conformance script
// classify dishonest providers.
func checkTensorExecInfo(t *testing.T, op string, info ExecInfo) {
	t.Helper()
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

var (
	probeLeft   = []float64{1, 2, 3, 4, 5, 6}
	probeRight  = []float64{10, 20, 30, 40, 50, 60}
	probeShape  = []int{2, 3}
	probeExpect = map[string][]float64{
		"tensor_add": {11, 22, 33, 44, 55, 66},
		"tensor_sub": {-9, -18, -27, -36, -45, -54},
		"tensor_mul": {10, 40, 90, 160, 250, 360},
		"tensor_div": {0.1, 0.1, 0.1, 0.1, 0.1, 0.1},
	}
)

// probeElementwise runs one same-shape binary op and checks values, output
// metadata, and execution reporting.
func probeElementwise(t *testing.T, plugin *Plugin, op string, dtype tensor.DType, shape []int,
	left, right, want []float64) {
	t.Helper()
	leftTensor := makeProbeTensor(t, dtype, shape, left)
	rightTensor := makeProbeTensor(t, dtype, shape, right)
	result, info, err := plugin.RunTensor(op, leftTensor, rightTensor)
	if err != nil {
		t.Fatalf("%s: %v", op, err)
	}
	if result != nil {
		defer tensor.Release(result)
	}
	checkTensorExecInfo(t, op, info)
	if result.DType() != dtype {
		t.Fatalf("%s dtype = %s, want %s", op, result.DType(), dtype)
	}
	if len(result.Shape()) != len(shape) {
		t.Fatalf("%s shape = %v, want %v", op, result.Shape(), shape)
	}
	for axis := range shape {
		if result.Shape()[axis] != shape[axis] {
			t.Fatalf("%s shape = %v, want %v", op, result.Shape(), shape)
		}
	}
	values, err := result.Float64Values()
	if err != nil {
		t.Fatalf("%s values: %v", op, err)
	}
	if len(values) != len(want) {
		t.Fatalf("%s returned %d values, want %d", op, len(values), len(want))
	}
	for i := range want {
		if math.Abs(values[i]-want[i]) > 1e-4 {
			t.Fatalf("%s result[%d] = %v, want %v", op, i, values[i], want[i])
		}
	}
}

// tensorOpProbes maps every binary tensor op the reference provider can
// declare to a happy-path probe. A manifest entry without a probe is a
// declared-but-missing lie and fails the coverage tests.
var tensorOpProbes = map[string]func(t *testing.T, plugin *Plugin, dtype tensor.DType){
	"tensor_add": func(t *testing.T, plugin *Plugin, dtype tensor.DType) {
		probeElementwise(t, plugin, "tensor_add", dtype, probeShape, probeLeft, probeRight, probeExpect["tensor_add"])
	},
	"tensor_sub": func(t *testing.T, plugin *Plugin, dtype tensor.DType) {
		probeElementwise(t, plugin, "tensor_sub", dtype, probeShape, probeLeft, probeRight, probeExpect["tensor_sub"])
	},
	"tensor_mul": func(t *testing.T, plugin *Plugin, dtype tensor.DType) {
		probeElementwise(t, plugin, "tensor_mul", dtype, probeShape, probeLeft, probeRight, probeExpect["tensor_mul"])
	},
	"tensor_div": func(t *testing.T, plugin *Plugin, dtype tensor.DType) {
		probeElementwise(t, plugin, "tensor_div", dtype, probeShape, probeLeft, probeRight, probeExpect["tensor_div"])
	},
	"tensor_matmul": func(t *testing.T, plugin *Plugin, dtype tensor.DType) {
		left := makeProbeTensor(t, dtype, []int{2, 2}, []float64{1, 2, 3, 4})
		right := makeProbeTensor(t, dtype, []int{2, 2}, []float64{5, 6, 7, 8})
		result, info, err := plugin.RunTensor("tensor_matmul", left, right)
		if err != nil {
			t.Fatalf("tensor_matmul: %v", err)
		}
		if result != nil {
			defer tensor.Release(result)
		}
		checkTensorExecInfo(t, "tensor_matmul", info)
		values, err := result.Float64Values()
		if err != nil {
			t.Fatal(err)
		}
		for i, want := range []float64{19, 22, 43, 50} {
			if math.Abs(values[i]-want) > 1e-4 {
				t.Fatalf("tensor_matmul result[%d] = %v, want %v", i, values[i], want)
			}
		}
	},
	"tensor_sum": func(t *testing.T, plugin *Plugin, dtype tensor.DType) {
		input := makeProbeTensor(t, dtype, probeShape, probeLeft)
		result, info, err := plugin.RunTensor("tensor_sum", input)
		if err != nil {
			t.Fatalf("tensor_sum: %v", err)
		}
		if result != nil {
			defer tensor.Release(result)
		}
		checkTensorExecInfo(t, "tensor_sum", info)
		if result.DType() != dtype {
			t.Fatalf("tensor_sum dtype = %s, want %s", result.DType(), dtype)
		}
		// Full reduction is NumPy-consistent: the result is rank-0.
		if len(result.Shape()) != 0 {
			t.Fatalf("tensor_sum shape = %v, want rank-0", result.Shape())
		}
		values, err := result.Float64Values()
		if err != nil {
			t.Fatal(err)
		}
		if len(values) != 1 || math.Abs(values[0]-21) > 1e-4 {
			t.Fatalf("tensor_sum = %v, want [21]", values)
		}
	},
}

func TestExampleTensorElementwiseOps(t *testing.T) {
	plugin := loadExampleProvider(t)

	// Happy paths for every elementwise op in both float dtypes.
	for _, op := range []string{"tensor_add", "tensor_sub", "tensor_mul", "tensor_div"} {
		probeElementwise(t, plugin, op, tensor.Float64, probeShape, probeLeft, probeRight, probeExpect[op])
	}
	probeElementwise(t, plugin, "tensor_sub", tensor.Float32, []int{4},
		[]float64{1.5, 2.5, 3.5, 4.5}, []float64{0.5, 0.5, 2.0, 1.5}, []float64{1, 2, 1.5, 3})
	probeElementwise(t, plugin, "tensor_mul", tensor.Float32, []int{4},
		[]float64{1.5, 2.5, 3.5, 4.5}, []float64{0.5, 0.5, 2.0, 1.5}, []float64{0.75, 1.25, 7, 6.75})
	probeElementwise(t, plugin, "tensor_div", tensor.Float32, []int{4},
		[]float64{1.5, 2.5, 3.5, 4.5}, []float64{0.5, 0.5, 2.0, 1.5}, []float64{3, 5, 1.75, 3})
	probeElementwise(t, plugin, "tensor_add", tensor.Float32, []int{4},
		[]float64{1.5, 2.5, 3.5, 4.5}, []float64{0.5, 0.5, 2.0, 1.5}, []float64{2, 3, 5.5, 6})

	left := makeProbeTensor(t, tensor.Float64, probeShape, probeLeft)
	mismatched := makeProbeTensor(t, tensor.Float64, []int{3, 2}, probeLeft)
	vector := makeProbeTensor(t, tensor.Float64, []int{6}, probeLeft)
	rank3 := makeProbeTensor(t, tensor.Float64, []int{2, 3, 1}, probeLeft)
	float32Tensor := makeProbeTensor(t, tensor.Float32, probeShape, probeLeft)
	intTensor, err := tensor.FromList(tensor.Int64, probeShape,
		[]any{int64(1), int64(2), int64(3), int64(4), int64(5), int64(6)})
	if err != nil {
		t.Fatal(err)
	}

	for _, op := range []string{"tensor_sub", "tensor_mul", "tensor_div"} {
		// Broadcasting is not a coverage claim: mismatched shapes must error.
		if _, _, err := plugin.RunTensor(op, left, mismatched); err == nil {
			t.Fatalf("%s accepted mismatched shapes", op)
		}
		// Same element count, different rank: still not same-shape.
		if _, _, err := plugin.RunTensor(op, left, vector); err == nil {
			t.Fatalf("%s accepted mismatched ranks", op)
		}
		if _, _, err := plugin.RunTensor(op, rank3, rank3); err == nil {
			t.Fatalf("%s accepted rank-3 inputs", op)
		}
		if _, _, err := plugin.RunTensor(op, left, intTensor); err == nil {
			t.Fatalf("%s accepted an unsupported dtype", op)
		}
		if _, _, err := plugin.RunTensor(op, left, float32Tensor); err == nil {
			t.Fatalf("%s accepted mixed dtypes", op)
		}
		// Elementwise ops are binary: a single input must error, not crash.
		if _, _, err := plugin.RunTensor(op, left); err == nil {
			t.Fatalf("%s accepted one input", op)
		}
	}
}

func TestExampleTensorSum(t *testing.T) {
	plugin := loadExampleProvider(t)
	if !supportsOperation(plugin.Manifest().Operations, "tensor_sum") {
		t.Fatalf("reference provider does not declare tensor_sum: %+v", plugin.Manifest())
	}

	// float64 matrix: NumPy-consistent full reduction to a rank-0 result.
	input := makeProbeTensor(t, tensor.Float64, probeShape, probeLeft)
	result, info, err := plugin.RunTensor("tensor_sum", input)
	if err != nil {
		t.Fatal("tensor_sum: ", err)
	}
	defer tensor.Release(result)
	if !info.Reported || info.Status != StatusDevice || info.Device != "cpu" || info.Fallback {
		t.Fatalf("reference tensor_sum exec info = %+v", info)
	}
	if result.DType() != tensor.Float64 || len(result.Shape()) != 0 {
		t.Fatalf("tensor_sum metadata: dtype=%s shape=%v, want float64 rank-0",
			result.DType(), result.Shape())
	}
	values, err := result.Float64Values()
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values[0] != 21 {
		t.Fatalf("tensor_sum = %v, want [21]", values)
	}

	// float32 vector.
	vector := makeProbeTensor(t, tensor.Float32, []int{4}, []float64{1.5, 2.5, 3.5, 4.5})
	f32Result, info, err := plugin.RunTensor("tensor_sum", vector)
	if err != nil {
		t.Fatal("tensor_sum float32: ", err)
	}
	defer tensor.Release(f32Result)
	checkTensorExecInfo(t, "tensor_sum", info)
	if f32Result.DType() != tensor.Float32 || len(f32Result.Shape()) != 0 {
		t.Fatalf("tensor_sum float32 metadata: dtype=%s shape=%v", f32Result.DType(), f32Result.Shape())
	}
	f32Values, err := f32Result.Float64Values()
	if err != nil {
		t.Fatal(err)
	}
	if len(f32Values) != 1 || math.Abs(f32Values[0]-12) > 1e-4 {
		t.Fatalf("tensor_sum float32 = %v, want [12]", f32Values)
	}

	// Rejections: wrong dtype, wrong rank, wrong arity.
	intTensor, err := tensor.FromList(tensor.Int64, probeShape,
		[]any{int64(1), int64(2), int64(3), int64(4), int64(5), int64(6)})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := plugin.RunTensor("tensor_sum", intTensor); err == nil {
		t.Fatal("tensor_sum accepted an unsupported dtype")
	}
	rank3 := makeProbeTensor(t, tensor.Float64, []int{2, 3, 1}, probeLeft)
	if _, _, err := plugin.RunTensor("tensor_sum", rank3); err == nil {
		t.Fatal("tensor_sum accepted a rank-3 input")
	}
	if _, _, err := plugin.RunTensor("tensor_sum", input, input); err == nil {
		t.Fatal("tensor_sum accepted two inputs")
	}
	if _, _, err := plugin.RunTensor("tensor_neg", input); err == nil {
		t.Fatal("undeclared tensor_neg was not rejected")
	}
}

// TestExampleManifestTensorOpCoverage is the manifest-driven coverage gate for
// the reference provider: every declared tensor op must have a probe and must
// pass it. A manifest entry whose implementation is removed (declared but
// missing) fails here.
func TestExampleManifestTensorOpCoverage(t *testing.T) {
	plugin := loadExampleProvider(t)
	declared := 0
	for _, op := range plugin.Manifest().Operations {
		if !strings.HasPrefix(op, "tensor_") {
			continue
		}
		declared++
		probe, ok := tensorOpProbes[op]
		if !ok {
			t.Fatalf("OP_FAILED: manifest declares %q but no coverage probe exists (declared-but-missing)", op)
		}
		probe(t, plugin, tensor.Float64)
		t.Logf("OP_COVERED %s", op)
	}
	if declared == 0 {
		t.Fatal("reference provider declares no tensor operations")
	}
}

// TestExternalProviderTensorCoverage exercises every tensor op an external
// provider declares, in the provider's advertised dtype. Each passing op logs
// an OP_COVERED line that the conformance script collects into the per-op
// coverage report; any declared op that errors fails the provider.
func TestExternalProviderTensorCoverage(t *testing.T) {
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
	dtype := tensor.Float32
	if manifest.Metadata["dtype"] == "float64" {
		dtype = tensor.Float64
	}
	declared := 0
	for _, op := range manifest.Operations {
		if !strings.HasPrefix(op, "tensor_") {
			continue
		}
		declared++
		probe, ok := tensorOpProbes[op]
		if !ok {
			t.Fatalf("OP_FAILED: provider %q declares %q but no coverage probe exists (declared-but-missing)",
				manifest.Name, op)
		}
		// A declared op whose probe errors (or panics) fails this test; the
		// conformance script then classifies the provider as failed.
		probe(t, plugin, dtype)
		t.Logf("OP_COVERED %s", op)
	}
	if declared == 0 {
		t.Log("provider declares no tensor operations")
	}
}
