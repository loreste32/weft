package accelerator

import (
	"errors"
	"strings"
	"testing"

	"github.com/loreste/weft/internal/tensor"
)

// fakeExecTensorNative is a tensor provider with a configurable
// weft_accel_exec_info document, used to adversarially probe the host's
// parsing and honesty validation.
type fakeExecTensorNative struct {
	info    []byte
	infoErr error
}

func (f *fakeExecTensorNative) manifest() ([]byte, error) { return []byte(`{}`), nil }
func (f *fakeExecTensorNative) run(string, []byte) ([]byte, error) {
	return []byte(`{"ok":true}`), nil
}
func (f *fakeExecTensorNative) close() error { return nil }
func (f *fakeExecTensorNative) runTensor(_ string, _ []*tensor.Tensor) (*tensor.Tensor, error) {
	return tensor.FromFloat64([]int{1}, []float64{1})
}
func (f *fakeExecTensorNative) execInfo() ([]byte, error) { return f.info, f.infoErr }

func newExecTensorPlugin(native nativePlugin) *Plugin {
	return &Plugin{
		manifest: Manifest{
			Name:       "fake",
			Version:    "1",
			ABI:        ABI,
			Vendors:    []string{"fake"},
			Operations: []string{"tensor_matmul"},
		},
		native: native,
	}
}

func mustTensor(t *testing.T) *tensor.Tensor {
	t.Helper()
	value, err := tensor.FromFloat64([]int{1}, []float64{1})
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestParseExecInfoMissingFieldsIsUnreported(t *testing.T) {
	cases := map[string]string{
		"no fields at all":   `{"data":[1,2,3],"shape":[3]}`,
		"device only":        `{"device":"cpu"}`,
		"fallback only":      `{"fallback":false}`,
		"empty device":       `{"device":"","fallback":false}`,
		"wrong fallbacktype": `{"device":"cpu","fallback":"no"}`,
		"not an object":      `[1,2,3]`,
		"not json":           `garbage`,
	}
	for name, payload := range cases {
		info := parseExecInfo([]byte(payload))
		if info.Reported || info.Status != StatusUnreported {
			t.Errorf("%s: parseExecInfo(%s) = %+v, want unreported", name, payload, info)
		}
	}
}

func TestRunJSONExParsesHonestReport(t *testing.T) {
	fake := &fakeNative{output: []byte(`{"device":"cuda:0","requested_device":"cuda:0","fallback":false}`)}
	p := &Plugin{manifest: Manifest{
		Name: "fake", Version: "1", ABI: ABI, Vendors: []string{"fake"}, Operations: []string{"matmul"},
	}, native: fake}
	_, info, err := p.RunJSONEx("matmul", []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if !info.Reported || info.Status != StatusDevice || info.Device != "cuda:0" ||
		info.RequestedDevice != "cuda:0" || info.Fallback {
		t.Fatalf("honest report parsed as %+v", info)
	}
}

func TestRunJSONExMissingFieldsIsUnreportedNotDevice(t *testing.T) {
	fake := &fakeNative{output: []byte(`{"data":[1],"shape":[1]}`)}
	p := &Plugin{manifest: Manifest{
		Name: "fake", Version: "1", ABI: ABI, Vendors: []string{"fake"}, Operations: []string{"matmul"},
	}, native: fake}
	_, info, err := p.RunJSONEx("matmul", []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if info.Reported || info.Status != StatusUnreported {
		t.Fatalf("field-less result must be unreported, got %+v", info)
	}
}

func TestRunJSONExContradictoryReportIsError(t *testing.T) {
	// The lie: claims it ran on cuda while acknowledging the request was cpu,
	// without setting the fallback flag.
	fake := &fakeNative{output: []byte(`{"device":"cuda:0","requested_device":"cpu","fallback":false}`)}
	p := &Plugin{manifest: Manifest{
		Name: "fake", Version: "1", ABI: ABI, Vendors: []string{"fake"}, Operations: []string{"matmul"},
	}, native: fake}
	out, info, err := p.RunJSONEx("matmul", []byte(`{}`))
	if err == nil {
		t.Fatalf("contradictory report accepted: out=%s info=%+v", out, info)
	}
	if !strings.Contains(err.Error(), "no fallback") {
		t.Fatalf("contradiction error = %v", err)
	}
	// RunJSON must surface the same honesty error.
	if _, _, err := p.RunJSONEx("matmul", []byte(`{}`)); err == nil {
		t.Fatal("RunJSONEx accepted contradictory report")
	}
}

func TestRunJSONExHonestFallbackReport(t *testing.T) {
	// Honest fallback: requested cuda, ran on cpu, fallback flagged.
	fake := &fakeNative{output: []byte(`{"device":"cpu","requested_device":"cuda","fallback":true}`)}
	p := &Plugin{manifest: Manifest{
		Name: "fake", Version: "1", ABI: ABI, Vendors: []string{"fake"}, Operations: []string{"matmul"},
	}, native: fake}
	_, info, err := p.RunJSONEx("matmul", []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if !info.Reported || info.Status != StatusFallback || !info.Fallback || info.Device != "cpu" ||
		info.RequestedDevice != "cuda" {
		t.Fatalf("fallback report parsed as %+v", info)
	}
}

func TestRunJSONExContradictoryStatusFlag(t *testing.T) {
	// Status claims fallback while the fallback flag says otherwise.
	fake := &fakeNative{output: []byte(`{"device":"cpu","requested_device":"cpu","fallback":false,"status":"fallback"}`)}
	p := &Plugin{manifest: Manifest{
		Name: "fake", Version: "1", ABI: ABI, Vendors: []string{"fake"}, Operations: []string{"matmul"},
	}, native: fake}
	if _, _, err := p.RunJSONEx("matmul", []byte(`{}`)); err == nil ||
		!strings.Contains(err.Error(), "fallback") {
		t.Fatalf("status/flag contradiction error = %v", err)
	}
}

func TestRunTensorExecInfoHonest(t *testing.T) {
	p := newExecTensorPlugin(&fakeExecTensorNative{
		info: []byte(`{"device":"cpu","requested_device":"cpu","fallback":false,"status":"device"}`),
	})
	output, info, err := p.RunTensor("tensor_matmul", mustTensor(t), mustTensor(t))
	if err != nil {
		t.Fatal(err)
	}
	defer tensor.Release(output)
	if !info.Reported || info.Status != StatusDevice || info.Device != "cpu" || info.Fallback {
		t.Fatalf("tensor exec info = %+v", info)
	}
}

func TestRunTensorExecInfoMalformedIsUnreported(t *testing.T) {
	p := newExecTensorPlugin(&fakeExecTensorNative{info: []byte(`{not json`)})
	output, info, err := p.RunTensor("tensor_matmul", mustTensor(t), mustTensor(t))
	if err != nil {
		t.Fatal(err)
	}
	defer tensor.Release(output)
	if info.Reported || info.Status != StatusUnreported {
		t.Fatalf("malformed exec info must be unreported, got %+v", info)
	}
}

func TestRunTensorExecInfoExportMissingIsUnreported(t *testing.T) {
	// fakeTensorNative implements no execInfo method, like a provider built
	// before the export existed.
	p := newExecTensorPlugin(&fakeTensorNative{})
	output, info, err := p.RunTensor("tensor_matmul", mustTensor(t), mustTensor(t))
	if err != nil {
		t.Fatal(err)
	}
	defer tensor.Release(output)
	if info.Reported || info.Status != StatusUnreported {
		t.Fatalf("provider without exec_info export must be unreported, got %+v", info)
	}
}

func TestRunTensorExecInfoErrorIsUnreported(t *testing.T) {
	p := newExecTensorPlugin(&fakeExecTensorNative{infoErr: errors.New("boom")})
	output, info, err := p.RunTensor("tensor_matmul", mustTensor(t), mustTensor(t))
	if err != nil {
		t.Fatal(err)
	}
	defer tensor.Release(output)
	if info.Reported || info.Status != StatusUnreported {
		t.Fatalf("failing exec_info must be unreported, got %+v", info)
	}
}

func TestRunTensorExecInfoContradictoryIsError(t *testing.T) {
	p := newExecTensorPlugin(&fakeExecTensorNative{
		info: []byte(`{"device":"cuda:0","requested_device":"cpu","fallback":false}`),
	})
	output, _, err := p.RunTensor("tensor_matmul", mustTensor(t), mustTensor(t))
	if err == nil {
		if output != nil {
			tensor.Release(output)
		}
		t.Fatal("contradictory tensor exec info accepted")
	}
	if output != nil {
		tensor.Release(output)
		t.Fatal("contradictory run must not hand back an output tensor")
	}
	if !strings.Contains(err.Error(), "no fallback") {
		t.Fatalf("contradiction error = %v", err)
	}
}

func TestUnavailableIsNotAnOpFailure(t *testing.T) {
	// Disabled plugins: Load fails with an explicit unavailable-style error,
	// distinct from an operation failure on a loaded plugin.
	t.Setenv(EnvDisablePlugins, "1")
	_, err := Load("/nonexistent/provider.so")
	if err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("disabled load error = %v", err)
	}
	info := UnavailableExecInfo()
	if info.Status != StatusUnavailable || info.Reported {
		t.Fatalf("UnavailableExecInfo() = %+v", info)
	}
	if info.Status == StatusDevice || info.Status == StatusFallback {
		t.Fatalf("unavailable status %q must be distinguishable from execution statuses", info.Status)
	}
}
