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
    result := accelerator.run_tensor(plugin, "tensor_matmul", [
        {"dtype": "float64", "shape": [2, 2], "data": [1.0, 2.0, 3.0, 4.0]},
        {"dtype": "float64", "shape": [2, 2], "data": [5.0, 6.0, 7.0, 8.0]},
    ])?
    ensure(result.dtype == "float64", "unexpected dtype")?
    ensure(result.shape == [2, 2], "unexpected shape")?
    ensure(result.data[0] == 19.0 && result.data[1] == 22.0, "unexpected first row")?
    ensure(result.data[2] == 43.0 && result.data[3] == 50.0, "unexpected second row")?
    accelerator.close(plugin)?
}
`, pluginPath)
	var output bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &output})
	if err := ctx.RunSource(context.Background(), "accelerator_tensor.weft", source); err != nil {
		t.Fatal(err)
	}
}
