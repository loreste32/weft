package weft_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loreste/weft/pkg/weft"
)

func TestModuleDeniedShWithoutCapability(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(root, "sheller")
	_ = os.MkdirAll(pkg, 0o755)
	_ = os.WriteFile(filepath.Join(pkg, "lib.weft"), []byte(`
use sh
pub fn run() -> Result {
    sh.run("true")?
}
`), 0o644)
	_ = os.WriteFile(filepath.Join(pkg, "weft.json"), []byte(`{
  "name": "sheller",
  "type": "module",
  "entry": "lib.weft",
  "exports": ["run"]
}`), 0o644)
	app := filepath.Join(root, "app")
	_ = os.MkdirAll(app, 0o755)
	_ = os.WriteFile(filepath.Join(app, "weft.json"), []byte(`{"name":"app","deps":{"sheller":{"path":"../sheller"}}}`), 0o644)
	_ = os.WriteFile(filepath.Join(app, "main.weft"), []byte(`
use sheller
fn main -> Result {
    sheller.run()?
}
`), 0o644)
	if err := weft.PkgInstall(app); err != nil {
		t.Fatal(err)
	}
	ctx := weft.New(weft.Options{})
	err := ctx.RunFile(context.Background(), filepath.Join(app, "main.weft"))
	if err == nil || !strings.Contains(err.Error(), "capability denied") {
		t.Fatalf("want capability denied, got %v", err)
	}
}

func TestModuleGrantedShWithCapability(t *testing.T) {
	root := t.TempDir()
	pkg := filepath.Join(root, "sheller")
	_ = os.MkdirAll(pkg, 0o755)
	_ = os.WriteFile(filepath.Join(pkg, "lib.weft"), []byte(`
use sh
pub fn run() -> Result {
    r := sh.run("true")?
    Ok(r.ok)
}
`), 0o644)
	_ = os.WriteFile(filepath.Join(pkg, "weft.json"), []byte(`{
  "name": "sheller",
  "type": "module",
  "entry": "lib.weft",
  "exports": ["run"],
  "capabilities": ["sh"]
}`), 0o644)
	app := filepath.Join(root, "app")
	_ = os.MkdirAll(app, 0o755)
	_ = os.WriteFile(filepath.Join(app, "weft.json"), []byte(`{"name":"app","deps":{"sheller":{"path":"../sheller"}}}`), 0o644)
	_ = os.WriteFile(filepath.Join(app, "main.weft"), []byte(`
use sheller
fn main -> Result {
    say(sheller.run()?)
}
`), 0o644)
	if err := weft.PkgInstall(app); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	ctx := weft.New(weft.Options{Stdout: &out})
	if err := ctx.RunFile(context.Background(), filepath.Join(app, "main.weft")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "true") {
		t.Fatalf("out %q", out.String())
	}
}
