package pkgman

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var validPkgName = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// ScaffoldOptions configures weft new module|app.
type ScaffoldOptions struct {
	// Dir is the parent directory (default cwd). Module is created at Dir/Name.
	Dir string
	// Name is the package/app identifier (import name).
	Name string
	// Kind: "module" or "app"
	Kind string
	// Force overwrite existing non-empty dir
	Force bool
}

// Scaffold creates a new Weft module or application for third-party authors.
func Scaffold(opts ScaffoldOptions) (string, error) {
	name := strings.TrimSpace(opts.Name)
	if name == "" {
		return "", fmt.Errorf("name required")
	}
	// allow dashes in folder; normalize import name
	importName := strings.ReplaceAll(strings.ToLower(name), "-", "_")
	importName = strings.ReplaceAll(importName, " ", "_")
	if !validPkgName.MatchString(importName) {
		return "", fmt.Errorf("invalid name %q — use lowercase letters, digits, underscore (e.g. greeter, http_util)", importName)
	}
	kind := strings.ToLower(opts.Kind)
	if kind == "" {
		kind = "module"
	}
	if kind == "package" || kind == "lib" {
		kind = "module"
	}
	if kind == "tool" || kind == "ctl" {
		kind = "cli"
	}
	if kind != "module" && kind != "app" && kind != "cli" {
		return "", fmt.Errorf("kind must be module, app, or cli")
	}

	parent := opts.Dir
	if parent == "" {
		var err error
		parent, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	root := filepath.Join(parent, name)
	if st, err := os.Stat(root); err == nil {
		if !opts.Force {
			ents, _ := os.ReadDir(root)
			if st.IsDir() && len(ents) > 0 {
				return "", fmt.Errorf("%s already exists (use --force)", root)
			}
		}
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}

	switch kind {
	case "module":
		if err := scaffoldModule(root, importName, name); err != nil {
			return "", err
		}
	case "app":
		if err := scaffoldApp(root, importName, name); err != nil {
			return "", err
		}
	case "cli":
		if err := scaffoldCLI(root, importName, name); err != nil {
			return "", err
		}
	}
	return root, nil
}

func scaffoldModule(root, importName, display string) error {
	m := &Manifest{
		Name:        importName,
		Version:     "0.1.0",
		Description: display + " — Weft module",
		Type:        "module",
		Entry:       "lib.weft",
		License:     "Apache-2.0",
		Exports:     []string{"hello", "greet"},
		Deps:        map[string]DepSpec{},
	}
	if err := SaveManifest(root, m); err != nil {
		return err
	}
	lib := fmt.Sprintf(`// %s — Weft module
// Consumers: use %s   then  %s.hello("world")
// Publish: git tag v0.1.0 → weft get %s github.com/you/%s@v0.1.0

// Only pub symbols are exported when any pub is present.
pub fn hello(name) {
    "hello, " + name + " from %s"
}

// Private helper (not imported by consumers)
fn shout(s) {
    s + "!"
}

pub fn greet(name) {
    shout(hello(name))
}
`, importName, importName, importName, importName, display, importName)

	if err := os.WriteFile(filepath.Join(root, "lib.weft"), []byte(lib), 0o644); err != nil {
		return err
	}

	// Multi-file example: optional util
	util := `// Internal helper module — import from lib.weft with:
//   use "./util.weft" as util
pub fn trim(s) {
    s
}
`
	if err := os.WriteFile(filepath.Join(root, "util.weft"), []byte(util), 0o644); err != nil {
		return err
	}

	readme := fmt.Sprintf(`# %s

Weft **module** — installable library for other Weft apps.

## Use it

`+"```bash"+`
# from a path (local development)
weft get %s %s

# from git (after you push + tag)
weft get %s github.com/you/%s@v0.1.0
weft install
`+"```"+`

`+"```weft"+`
use %s

fn main {
    say(%s.hello("weft"))
    say(%s.greet("devs"))
}
`+"```"+`

## Author checklist

1. Edit `+"`lib.weft`"+` — mark public API with `+"`pub fn`"+`
2. Multi-file: `+"`use \"./util.weft\" as util`"+` (only entry exports are the package surface)
3. Depend on other modules: add `+"`deps`"+` in `+"`weft.json`"+` (consumers get them transitively)
4. Set `+"`name`"+` / `+"`version`"+` / `+"`exports`"+` in `+"`weft.json`"+`
5. `+"`weft mod check`"+` — validate (parses all .weft files)
6. `+"`weft mod check --tests`"+` — validate + run tests
7. `+"`weft mod pack`"+` — zip for distribution
8. Tag a release and share the git URL (or add to monorepo packages/index.json)

Modules **expand Weft** for your users: they `+"`use "+importName+"`"+` and call your API — no binary plugins.

See [docs/modules.md](https://github.com/loreste/weft/blob/main/docs/modules.md).
`, display, importName, root, importName, display, importName, importName, importName)

	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte(readme), 0o644); err != nil {
		return err
	}
	// Unit tests: weft test (path-import the entry so tests resolve)
	testSrc := fmt.Sprintf(`// %s tests — run: weft test
use "./lib.weft" as mod

fn test_hello {
    test.eq(mod.hello("weft"), "hello, weft from %s")
}

fn test_greet {
    test.contains(mod.greet("x"), "hello")
}
`, importName, importName)
	if err := os.WriteFile(filepath.Join(root, importName+"_test.weft"), []byte(testSrc), 0o644); err != nil {
		return err
	}
	_ = os.WriteFile(filepath.Join(root, ".gitignore"), []byte("vendor/\n*.weftpkg\n"), 0o644)
	return nil
}

func scaffoldApp(root, importName, display string) error {
	m := &Manifest{
		Name:        importName,
		Version:     "0.1.0",
		Description: display + " — Weft app",
		Type:        "app",
		Deps:        map[string]DepSpec{},
	}
	if err := SaveManifest(root, m); err != nil {
		return err
	}
	main := fmt.Sprintf(`// %s — Weft application
fn main {
    say("hello from %s")
    // Add modules:  weft get greeter ./path/or/github.com/org/repo@v0.1
    // use greeter
    // say(greeter.hello("world"))
}
`, display, importName)
	if err := os.WriteFile(filepath.Join(root, "main.weft"), []byte(main), 0o644); err != nil {
		return err
	}
	readme := fmt.Sprintf(`# %s

Weft application.

`+"```bash"+`
weft run main.weft
weft get <module> <path|git>
weft install
`+"```"+`
`, display)
	return os.WriteFile(filepath.Join(root, "README.md"), []byte(readme), 0o644)
}

func scaffoldCLI(root, importName, display string) error {
	m := &Manifest{
		Name:        importName,
		Version:     "0.1.0",
		Description: display + " — Weft CLI tool",
		Type:        "app",
		Deps:        map[string]DepSpec{},
	}
	if err := SaveManifest(root, m); err != nil {
		return err
	}
	main := fmt.Sprintf(`// %s — devops/data CLI
//   weft run main.weft -- --help
//   weft run main.weft -- greet Ada -e prod
//   weft run main.weft -- lines path/to/file

fn main -> Result {
    p := cli.parse({
        "about": "%s — Weft CLI",
        "flags": {
            "env": {"short": "e", "default": "dev", "help": "environment"},
            "verbose": {"short": "v", "bool": true, "help": "verbose logs on stderr"},
            "output": {"short": "o", "default": "", "help": "write result to file"},
        },
    })?

    if p.help {
        say(p.usage)
        cli.exit(0)
    }

    args := p.args
    if len(args) == 0 {
        io.eprintln("commands: greet | lines | shell | version")
        say(p.usage)
        cli.exit(2)
    }

    cmd := args[0]
    if p.verbose {
        io.eprintln("[" + time.iso() + "] env=" + p.env + " cmd=" + cmd)
    }

    if cmd == "version" {
        say("%s 0.1.0")
        return Ok(unit)
    }

    if cmd == "greet" {
        mut name := "world"
        if len(args) > 1 {
            name = args[1]
        }
        msg := "hello, " + name + " (" + p.env + ")"
        say(msg)
        if p.output != "" {
            fs.write(p.output, msg + "\n")?
        }
        return Ok(unit)
    }

    if cmd == "lines" {
        if len(args) < 2 {
            cli.die("usage: lines <file>")
        }
        ls := fs.lines(args[1])?
        say(args[1] + ": " + len(ls) + " lines")
        return Ok(unit)
    }

    if cmd == "shell" {
        if len(args) < 2 {
            cli.die("usage: shell <command…>")
        }
        mut line := args[1]
        mut i := 2
        while i < len(args) {
            line = line + " " + args[i]
            i = i + 1
        }
        r := sh.shell(line)?
        if r.stdout != "" { print(r.stdout) }
        if r.stderr != "" { io.eprint(r.stderr) }
        if !r.ok { cli.exit(r.code) }
        return Ok(unit)
    }

    cli.die("unknown command: " + cmd)
}
`, display, importName, importName)
	if err := os.WriteFile(filepath.Join(root, "main.weft"), []byte(main), 0o644); err != nil {
		return err
	}
	readme := fmt.Sprintf(`# %s

Weft **CLI tool** for devops / data work.

`+"```bash"+`
weft run main.weft -- --help
weft run main.weft -- greet Ada -e prod
weft run main.weft -- lines README.md
weft run main.weft -- shell uname -s
`+"```"+`

See [docs/cli.md](https://github.com/loreste/weft/blob/main/docs/cli.md).
`, display)
	_ = os.WriteFile(filepath.Join(root, ".gitignore"), []byte("vendor/\n"), 0o644)
	return os.WriteFile(filepath.Join(root, "README.md"), []byte(readme), 0o644)
}
