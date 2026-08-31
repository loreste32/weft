//go:build !js

package stdlib_test

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/loreste/weft/pkg/weft"
)

func TestAcceleratorRunTensorThroughWeft(t *testing.T) {
	compiler, err := exec.LookPath("cc")
	if err != nil {
		t.Skip("C compiler unavailable")
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
	pluginPath := filepath.Join(t.TempDir(), "weft-example"+ext)
	args := append(flags, "-I", nativeDir, filepath.Join(nativeDir, "example.c"), "-o", pluginPath)
	if output, err := exec.Command(compiler, args...).CombinedOutput(); err != nil {
		t.Fatalf("compile example provider: %v\n%s", err, output)
	}
	source := fmt.Sprintf(`
use accelerator

fn main -> Result {
    plugin := accelerator.load(%q)?
    health := accelerator.run(plugin, "health", {})?
    ensure(health.ok == true, "health failed")?
    hinfo := accelerator.last_exec_info(plugin)?
    ensure(hinfo.reported == true, "health exec info unreported")?
    ensure(hinfo.device == "cpu", "health exec info device")?
    ensure(hinfo.requested_device == "cpu", "health exec info requested device")?
    ensure(hinfo.fallback == false, "health exec info fallback")?
    ensure(hinfo.status == "device", "health exec info status")?
    result := accelerator.run_tensor(plugin, "tensor_matmul", [
        {"dtype": "float64", "shape": [2, 2], "data": [1.0, 2.0, 3.0, 4.0]},
        {"dtype": "float64", "shape": [2, 2], "data": [5.0, 6.0, 7.0, 8.0]},
    ])?
    ensure(result.dtype == "float64", "unexpected dtype")?
    ensure(result.shape == [2, 2], "unexpected shape")?
    ensure(result.data[0] == 19.0 && result.data[1] == 22.0, "unexpected first row")?
    ensure(result.data[2] == 43.0 && result.data[3] == 50.0, "unexpected second row")?
    f16 := accelerator.run_tensor(plugin, "tensor_add", [
        {"dtype": "float16", "shape": [4], "data": [1.5, 2.5, 3.5, 4.5]},
        {"dtype": "float16", "shape": [4], "data": [0.5, 0.5, 2.0, 1.5]},
    ])?
    ensure(f16.dtype == "float16", "unexpected float16 dtype")?
    ensure(f16.shape == [4], "unexpected float16 shape")?
    ensure(f16.data[0] == 2.0 && f16.data[1] == 3.0, "unexpected float16 first values")?
    ensure(f16.data[2] == 5.5 && f16.data[3] == 6.0, "unexpected float16 last values")?
    info := accelerator.last_exec_info(plugin)?
    ensure(info.reported == true, "tensor exec info unreported")?
    ensure(info.device == "cpu", "tensor exec info device")?
    ensure(info.fallback == false, "tensor exec info fallback")?
    ensure(info.status == "device", "tensor exec info status")?
    accelerator.close(plugin)?
    gone := accelerator.last_exec_info(plugin)?
    ensure(gone.status == "unavailable", "closed plugin must report unavailable")?
    ensure(gone.reported == false, "closed plugin must not claim a report")?
}
`, pluginPath)
	var output bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &output})
	if err := ctx.RunSource(context.Background(), "accelerator_tensor.weft", source); err != nil {
		t.Fatal(err)
	}
}
