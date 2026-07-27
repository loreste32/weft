package stdlib

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/loreste/weft/internal/runtime"
)

func packageFS() runtime.Value {
	p := pkg()
	// fs.read(path) -> Result[str]
	set(p, "read", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("fs.read needs path", "fs"), nil
		}
		b, err := os.ReadFile(args[0].String())
		if err != nil {
			return errRes(err.Error(), "fs"), nil
		}
		return runtime.Ok(runtime.Str(string(b))), nil
	}, 1)
	// fs.write(path, text) -> Result[unit]
	set(p, "write", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("fs.write needs path and text", "fs"), nil
		}
		path := args[0].String()
		if dir := filepath.Dir(path); dir != "" && dir != "." {
			_ = os.MkdirAll(dir, 0o755)
		}
		if err := os.WriteFile(path, []byte(args[1].String()), 0o644); err != nil {
			return errRes(err.Error(), "fs"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	}, 2)
	// fs.stem(path) -> str  filename without extension
	set(p, "stem", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		base := filepath.Base(args[0].String())
		ext := filepath.Ext(base)
		if ext != "" {
			base = base[:len(base)-len(ext)]
		}
		return runtime.Str(base), nil
	}, 1)
	// fs.append(path, text) -> Result
	set(p, "append", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("fs.append(path, text)", "fs"), nil
		}
		path := args[0].String()
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return errRes(err.Error(), "fs"), nil
		}
		_, err = f.WriteString(args[1].String())
		_ = f.Close()
		if err != nil {
			return errRes(err.Error(), "fs"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	}, 2)
	// fs.exists(path) -> bool
	set(p, "exists", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Bool(false), nil
		}
		_, err := os.Stat(args[0].String())
		return runtime.Bool(err == nil), nil
	}, 1)
	// fs.is_dir / fs.is_file
	set(p, "is_dir", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Bool(false), nil
		}
		st, err := os.Stat(args[0].String())
		return runtime.Bool(err == nil && st.IsDir()), nil
	}, 1)
	set(p, "is_file", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Bool(false), nil
		}
		st, err := os.Stat(args[0].String())
		return runtime.Bool(err == nil && !st.IsDir()), nil
	}, 1)
	// fs.list(dir) -> Result[[str]] names
	set(p, "list", func(args []runtime.Value) (runtime.Value, error) {
		dir := "."
		if len(args) >= 1 {
			dir = args[0].String()
		}
		ents, err := os.ReadDir(dir)
		if err != nil {
			return errRes(err.Error(), "fs"), nil
		}
		items := make([]runtime.Value, 0, len(ents))
		for _, e := range ents {
			items = append(items, runtime.Str(e.Name()))
		}
		return runtime.Ok(runtime.List(items...)), nil
	}, 1)
	// fs.mkdir(path) -> Result
	set(p, "mkdir", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("fs.mkdir(path)", "fs"), nil
		}
		if err := os.MkdirAll(args[0].String(), 0o755); err != nil {
			return errRes(err.Error(), "fs"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	}, 1)
	// fs.remove(path) -> Result  (file or empty dir; use remove_all for trees)
	set(p, "remove", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("fs.remove(path)", "fs"), nil
		}
		if err := os.Remove(args[0].String()); err != nil {
			return errRes(err.Error(), "fs"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	}, 1)
	set(p, "remove_all", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("fs.remove_all(path)", "fs"), nil
		}
		if err := os.RemoveAll(args[0].String()); err != nil {
			return errRes(err.Error(), "fs"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	}, 1)
	// fs.join(a, b, ...) -> str
	set(p, "join", func(args []runtime.Value) (runtime.Value, error) {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = a.String()
		}
		return runtime.Str(filepath.Join(parts...)), nil
	}, -1)
	// fs.cwd() -> str
	set(p, "cwd", func(args []runtime.Value) (runtime.Value, error) {
		wd, err := os.Getwd()
		if err != nil {
			return errRes(err.Error(), "fs"), nil
		}
		return runtime.Str(wd), nil
	}, 0)
	// fs.abs(path) -> Result[str]
	set(p, "abs", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("fs.abs(path)", "fs"), nil
		}
		p, err := filepath.Abs(args[0].String())
		if err != nil {
			return errRes(err.Error(), "fs"), nil
		}
		return runtime.Ok(runtime.Str(p)), nil
	}, 1)
	// fs.base / fs.dir / fs.ext
	set(p, "base", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		return runtime.Str(filepath.Base(args[0].String())), nil
	}, 1)
	set(p, "dir", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		return runtime.Str(filepath.Dir(args[0].String())), nil
	}, 1)
	set(p, "ext", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		return runtime.Str(filepath.Ext(args[0].String())), nil
	}, 1)
	// fs.glob(pattern) -> Result[[str]]  (filepath.Glob)
	set(p, "glob", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("fs.glob(pattern)", "fs"), nil
		}
		matches, err := filepath.Glob(args[0].String())
		if err != nil {
			return errRes(err.Error(), "fs"), nil
		}
		items := make([]runtime.Value, len(matches))
		for i, m := range matches {
			items[i] = runtime.Str(m)
		}
		return runtime.Ok(runtime.List(items...)), nil
	}, 1)
	// fs.lines(path) -> Result[[str]]
	set(p, "lines", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("fs.lines(path)", "fs"), nil
		}
		b, err := os.ReadFile(args[0].String())
		if err != nil {
			return errRes(err.Error(), "fs"), nil
		}
		s := strings.ReplaceAll(string(b), "\r\n", "\n")
		s = strings.TrimSuffix(s, "\n")
		if s == "" {
			return runtime.Ok(runtime.List()), nil
		}
		parts := strings.Split(s, "\n")
		items := make([]runtime.Value, len(parts))
		for i, p := range parts {
			items[i] = runtime.Str(p)
		}
		return runtime.Ok(runtime.List(items...)), nil
	}, 1)
	// fs.copy(src, dst) -> Result
	set(p, "copy", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("fs.copy(src, dst)", "fs"), nil
		}
		b, err := os.ReadFile(args[0].String())
		if err != nil {
			return errRes(err.Error(), "fs"), nil
		}
		dst := args[1].String()
		if dir := filepath.Dir(dst); dir != "" && dir != "." {
			_ = os.MkdirAll(dir, 0o755)
		}
		if err := os.WriteFile(dst, b, 0o644); err != nil {
			return errRes(err.Error(), "fs"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	}, 2)

	// fs.temp_dir(prefix?) -> Result[str]
	set(p, "temp_dir", func(args []runtime.Value) (runtime.Value, error) {
		prefix := "weft-"
		if len(args) >= 1 && args[0].String() != "" {
			prefix = args[0].String()
		}
		d, err := os.MkdirTemp("", prefix)
		if err != nil {
			return errRes(err.Error(), "fs"), nil
		}
		return runtime.Ok(runtime.Str(d)), nil
	}, 1)

	// fs.temp_file(prefix?) -> Result[str]  empty file path
	set(p, "temp_file", func(args []runtime.Value) (runtime.Value, error) {
		prefix := "weft-"
		if len(args) >= 1 && args[0].String() != "" {
			prefix = args[0].String()
		}
		f, err := os.CreateTemp("", prefix)
		if err != nil {
			return errRes(err.Error(), "fs"), nil
		}
		name := f.Name()
		_ = f.Close()
		return runtime.Ok(runtime.Str(name)), nil
	}, 1)

	// fs.write_atomic(path, text) -> Result  write via temp + rename
	set(p, "write_atomic", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("fs.write_atomic(path, text)", "fs"), nil
		}
		path := args[0].String()
		dir := filepath.Dir(path)
		if dir == "" {
			dir = "."
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return errRes(err.Error(), "fs"), nil
		}
		tmp, err := os.CreateTemp(dir, ".weft-atomic-*")
		if err != nil {
			return errRes(err.Error(), "fs"), nil
		}
		tmpName := tmp.Name()
		if _, err := tmp.WriteString(args[1].String()); err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
			return errRes(err.Error(), "fs"), nil
		}
		if err := tmp.Close(); err != nil {
			_ = os.Remove(tmpName)
			return errRes(err.Error(), "fs"), nil
		}
		if err := os.Rename(tmpName, path); err != nil {
			// cross-device fallback
			b, err2 := os.ReadFile(tmpName)
			_ = os.Remove(tmpName)
			if err2 != nil {
				return errRes(err.Error(), "fs"), nil
			}
			if err2 := os.WriteFile(path, b, 0o644); err2 != nil {
				return errRes(err2.Error(), "fs"), nil
			}
		}
		return runtime.Ok(runtime.Unit()), nil
	}, 2)

	// fs.replace(old, new) -> Result  os.Rename
	set(p, "replace", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("fs.replace(old, new)", "fs"), nil
		}
		if err := os.Rename(args[0].String(), args[1].String()); err != nil {
			return errRes(err.Error(), "fs"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	}, 2)

	// fs.rename(old, new) — alias of replace (Python os.rename)
	set(p, "rename", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("fs.rename(old, new)", "fs"), nil
		}
		if err := os.Rename(args[0].String(), args[1].String()); err != nil {
			return errRes(err.Error(), "fs"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	}, 2)

	// fs.stat(path) -> Result[{size, mtime, mode, is_dir, is_file, name}]
	set(p, "stat", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("fs.stat(path)", "fs"), nil
		}
		st, err := os.Stat(args[0].String())
		if err != nil {
			return errRes(err.Error(), "fs"), nil
		}
		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		put := func(k string, v runtime.Value) {
			mo.Keys = append(mo.Keys, k)
			mo.Vals[k] = v
		}
		put("name", runtime.Str(st.Name()))
		put("size", runtime.Int(st.Size()))
		put("mtime", runtime.Int(st.ModTime().Unix()))
		put("mode", runtime.Int(int64(st.Mode().Perm())))
		put("is_dir", runtime.Bool(st.IsDir()))
		put("is_file", runtime.Bool(st.Mode().IsRegular()))
		return runtime.Ok(m), nil
	}, 1)

	// fs.chmod(path, mode) mode as int 0o644 style
	set(p, "chmod", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("fs.chmod(path, mode)", "fs"), nil
		}
		mode, err := runtime.AsInt(args[1])
		if err != nil {
			return errRes("fs.chmod: mode must be int", "fs"), nil
		}
		if err := os.Chmod(args[0].String(), os.FileMode(mode)); err != nil {
			return errRes(err.Error(), "fs"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	}, 2)

	// fs.size(path) -> Result[int]
	set(p, "size", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("fs.size(path)", "fs"), nil
		}
		st, err := os.Stat(args[0].String())
		if err != nil {
			return errRes(err.Error(), "fs"), nil
		}
		return runtime.Ok(runtime.Int(st.Size())), nil
	}, 1)

	// fs.walk(root) -> Result[[{path, name, size, is_dir}]]  depth-first, no follow symlink roots
	set(p, "walk", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("fs.walk(root)", "fs"), nil
		}
		root := args[0].String()
		var items []runtime.Value
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			// skip symlink directories to avoid cycles
			if info.Mode()&os.ModeSymlink != 0 && info.IsDir() {
				return filepath.SkipDir
			}
			m := runtime.NewMap()
			mo := m.Obj.(*runtime.MapObj)
			put := func(k string, v runtime.Value) {
				mo.Keys = append(mo.Keys, k)
				mo.Vals[k] = v
			}
			put("path", runtime.Str(path))
			put("name", runtime.Str(info.Name()))
			put("size", runtime.Int(info.Size()))
			put("is_dir", runtime.Bool(info.IsDir()))
			put("is_file", runtime.Bool(info.Mode().IsRegular()))
			items = append(items, m)
			return nil
		})
		if err != nil {
			return errRes(err.Error(), "fs"), nil
		}
		return runtime.Ok(runtime.List(items...)), nil
	}, 1)

	// fs.rel(path, base?) -> Result[str]  path relative to base (default cwd)
	set(p, "rel", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("fs.rel(path, base?)", "fs"), nil
		}
		target := args[0].String()
		base := "."
		if len(args) >= 2 && args[1].String() != "" {
			base = args[1].String()
		}
		rel, err := filepath.Rel(base, target)
		if err != nil {
			return errRes(err.Error(), "fs"), nil
		}
		return runtime.Ok(runtime.Str(rel)), nil
	}, 2)

	// fs.splitext(path) -> [stem, ext]  Python os.path.splitext
	set(p, "splitext", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.List(runtime.Str(""), runtime.Str("")), nil
		}
		ext := filepath.Ext(args[0].String())
		stem := strings.TrimSuffix(args[0].String(), ext)
		return runtime.List(runtime.Str(stem), runtime.Str(ext)), nil
	}, 1)

	// fs.norm(path) -> str  clean separators / dots
	set(p, "norm", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		return runtime.Str(filepath.Clean(args[0].String())), nil
	}, 1)

	// fs.expanduser(path) -> str  ~ and ~/
	set(p, "expanduser", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		s := args[0].String()
		if s == "~" || strings.HasPrefix(s, "~/") || strings.HasPrefix(s, `~\`) {
			home, err := os.UserHomeDir()
			if err != nil {
				return runtime.Str(s), nil
			}
			if s == "~" {
				return runtime.Str(home), nil
			}
			return runtime.Str(filepath.Join(home, s[2:])), nil
		}
		return runtime.Str(s), nil
	}, 1)

	// fs.move(src, dst) -> Result  rename with cross-device copy fallback
	set(p, "move", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("fs.move(src, dst)", "fs"), nil
		}
		src, dst := args[0].String(), args[1].String()
		if err := os.Rename(src, dst); err == nil {
			return runtime.Ok(runtime.Unit()), nil
		}
		// fallback copy+remove (file only)
		b, err := os.ReadFile(src)
		if err != nil {
			return errRes(err.Error(), "fs"), nil
		}
		if dir := filepath.Dir(dst); dir != "" && dir != "." {
			_ = os.MkdirAll(dir, 0o755)
		}
		if err := os.WriteFile(dst, b, 0o644); err != nil {
			return errRes(err.Error(), "fs"), nil
		}
		if err := os.Remove(src); err != nil {
			return errRes(err.Error(), "fs"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	}, 2)

	// fs.copy_tree(src, dst) -> Result  recursive directory copy
	set(p, "copy_tree", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("fs.copy_tree(src, dst)", "fs"), nil
		}
		src, dst := args[0].String(), args[1].String()
		if err := copyTree(src, dst); err != nil {
			return errRes(err.Error(), "fs"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	}, 2)

	// fs.touch(path) -> Result  create empty or update mtime
	set(p, "touch", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("fs.touch(path)", "fs"), nil
		}
		path := args[0].String()
		if _, err := os.Stat(path); err != nil {
			if dir := filepath.Dir(path); dir != "" && dir != "." {
				_ = os.MkdirAll(dir, 0o755)
			}
			f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644)
			if err != nil {
				return errRes(err.Error(), "fs"), nil
			}
			_ = f.Close()
			return runtime.Ok(runtime.Unit()), nil
		}
		now := time.Now()
		if err := os.Chtimes(path, now, now); err != nil {
			return errRes(err.Error(), "fs"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	}, 1)

	// fs.is_link(path) -> bool
	set(p, "is_link", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Bool(false), nil
		}
		st, err := os.Lstat(args[0].String())
		return runtime.Bool(err == nil && st.Mode()&os.ModeSymlink != 0), nil
	}, 1)

	// fs.readlink(path) -> Result[str]
	set(p, "readlink", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("fs.readlink(path)", "fs"), nil
		}
		s, err := os.Readlink(args[0].String())
		if err != nil {
			return errRes(err.Error(), "fs"), nil
		}
		return runtime.Ok(runtime.Str(s)), nil
	}, 1)

	// fs.symlink(target, link) -> Result
	set(p, "symlink", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("fs.symlink(target, link)", "fs"), nil
		}
		if err := os.Symlink(args[0].String(), args[1].String()); err != nil {
			return errRes(err.Error(), "fs"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	}, 2)

	// fs.fnmatch(name, pattern) -> bool  shell-style (filepath.Match)
	set(p, "fnmatch", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.Bool(false), nil
		}
		ok, err := filepath.Match(args[1].String(), args[0].String())
		if err != nil {
			return runtime.Bool(false), nil
		}
		return runtime.Bool(ok), nil
	}, 2)

	// fs.rglob(root, pattern) -> Result[[str]]  recursive fnmatch on basenames
	set(p, "rglob", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("fs.rglob(root, pattern)", "fs"), nil
		}
		root, pat := args[0].String(), args[1].String()
		var items []runtime.Value
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			ok, e := filepath.Match(pat, d.Name())
			if e != nil || !ok {
				return nil
			}
			items = append(items, runtime.Str(path))
			return nil
		})
		if err != nil {
			return errRes(err.Error(), "fs"), nil
		}
		return runtime.Ok(runtime.List(items...)), nil
	}, 2)

	// fs.samefile(a, b) -> bool
	set(p, "samefile", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.Bool(false), nil
		}
		a, err1 := os.Stat(args[0].String())
		b, err2 := os.Stat(args[1].String())
		if err1 != nil || err2 != nil {
			return runtime.Bool(false), nil
		}
		return runtime.Bool(os.SameFile(a, b)), nil
	}, 2)

	return p
}

func copyTree(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return copyFile(src, dst, info.Mode())
	}
	if err := os.MkdirAll(dst, info.Mode().Perm()); err != nil {
		return err
	}
	ents, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range ents {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyTree(s, d); err != nil {
				return err
			}
			continue
		}
		st, err := e.Info()
		if err != nil {
			return err
		}
		if err := copyFile(s, d, st.Mode()); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if dir := filepath.Dir(dst); dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0o755)
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode.Perm())
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	cerr := out.Close()
	if err != nil {
		return err
	}
	return cerr
}
