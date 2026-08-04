//go:build !js

package stdlib_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/loreste/weft/pkg/weft"
)

func compileMLReferenceProvider(t *testing.T) string {
	t.Helper()
	compiler, err := exec.LookPath("cc")
	if err != nil {
		t.Skip("native ML dispatch test requires a C compiler")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal("resolve repository root: ", err)
	}
	nativeDir := filepath.Join(root, "native", "accelerator")
	ext := ".so"
	flags := []string{"-shared", "-fPIC"}
	if runtime.GOOS == "darwin" {
		ext = ".dylib"
		flags = []string{"-dynamiclib", "-fPIC"}
	}
	path := filepath.Join(t.TempDir(), "weft-reference"+ext)
	args := append(flags, "-I", nativeDir, filepath.Join(nativeDir, "example.c"), "-o", path)
	output, err := exec.Command(compiler, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("compile reference accelerator: %v\n%s", err, output)
	}
	return path
}

func TestMLMatmulDispatchesThroughBoundPlugin(t *testing.T) {
	pluginPath := compileMLReferenceProvider(t)
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal("resolve test working directory: ", err)
	}
	projectDir := t.TempDir()
	manifest := fmt.Sprintf(`{"name":"ml-dispatch-fixture","version":"0.0.0","entry":"main.weft","deps":{"ml":{"path":%q},"warp":{"path":%q}}}
`, filepath.Join(root, "packages", "ml"), filepath.Join(root, "packages", "warp"))
	if err := os.WriteFile(filepath.Join(projectDir, "weft.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal("write fixture manifest: ", err)
	}
	source := fmt.Sprintf(`
use accelerator
use ml
use warp

fn main -> Result {
    device := ml.device_with_plugin("cpu", %q)?
    left := ml.to_device(warp.array([1.0, 2.0, 3.0, 4.0], [2, 2]), device)?
    right := ml.to_device(warp.array([5.0, 6.0, 7.0, 8.0], [2, 2]), device)?
    product := ml.matmul(left, right)?
    values := warp.to_list(ml.value(product)?)
    ensure(values[0] == 19.0, "native matmul result[0]")?
    ensure(values[1] == 22.0, "native matmul result[1]")?
    ensure(values[2] == 43.0, "native matmul result[2]")?
    ensure(values[3] == 50.0, "native matmul result[3]")?
    ensure(ml.device_of(ml.value(product)?)? == "cpu", "reference provider device")?
    info := ml.exec_info(device)?
    ensure(info.reported == true, "native dispatch must report execution")?
    ensure(info.status == "device", "native dispatch status")?
    ensure(info.device == "cpu", "native dispatch device")?
    ensure(info.fallback == false, "native dispatch fallback")?
    accelerator.close(device._plugin)?
    Ok(null)
}
`, pluginPath)

	mainPath := filepath.Join(projectDir, "main.weft")
	if err := os.WriteFile(mainPath, []byte(source), 0o644); err != nil {
		t.Fatal("write ML accelerator fixture: ", err)
	}
	ctx := weft.New(weft.Options{})
	if err := ctx.RunFile(context.Background(), mainPath); err != nil {
		t.Fatal("run ML accelerator dispatch fixture: ", err)
	}
}
