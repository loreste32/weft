//go:build js

package stdlib

import (
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/loreste/weft/internal/runtime"
)

// Browser builds expose a process-local virtual filesystem. It gives scripts
// deterministic fs semantics without pretending that a browser can access the
// user's disk. The store lasts for the lifetime of the Wasm module.
const (
	wasmMaxFileBytes  = 16 << 20
	wasmMaxStoreBytes = 64 << 20
	wasmMaxFileCount  = 10000
)

type wasmFile struct {
	data     []byte
	modified time.Time
}

type wasmFilesystem struct {
	mu    sync.RWMutex
	files map[string]wasmFile
	dirs  map[string]struct{}
	bytes int64
}

var (
	wasmFS     = &wasmFilesystem{files: map[string]wasmFile{}, dirs: map[string]struct{}{".": {}}}
	wasmTempID uint64
)

func wasmPath(raw string) string {
	raw = strings.ReplaceAll(raw, "\\", "/")
	clean := path.Clean(raw)
	if clean == "." || clean == "" || clean == "/" {
		return "."
	}
	clean = strings.TrimPrefix(clean, "./")
	// Block path traversal — reject any path that escapes the virtual root
	if clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
		return "."
	}
	// Strip leading slash (everything is relative to virtual root)
	clean = strings.TrimPrefix(clean, "/")
	if clean == "" {
		return "."
	}
	return clean
}

func wasmParent(name string) string {
	parent := path.Dir(name)
	if parent == "/" {
		return "."
	}
	return wasmPath(parent)
}

func (fs *wasmFilesystem) ensureParents(name string) {
	parents := []string{}
	for parent := wasmParent(name); parent != "."; parent = wasmParent(parent) {
		parents = append(parents, parent)
	}
	for i := len(parents) - 1; i >= 0; i-- {
		fs.dirs[parents[i]] = struct{}{}
	}
}

func wasmError(op, name, detail string) runtime.Value {
	return errRes(fmt.Sprintf("fs.%s(%q): %s", op, name, detail), "fs")
}

func wasmStoreBytes(fs *wasmFilesystem, name string, data []byte) error {
	if len(data) > wasmMaxFileBytes {
		return fmt.Errorf("file exceeds %d MiB limit", wasmMaxFileBytes>>20)
	}
	old := int64(len(fs.files[name].data))
	if fs.bytes-old+int64(len(data)) > wasmMaxStoreBytes {
		return fmt.Errorf("virtual filesystem exceeds %d MiB limit", wasmMaxStoreBytes>>20)
	}
	if _, exists := fs.files[name]; !exists && len(fs.files) >= wasmMaxFileCount {
		return fmt.Errorf("virtual filesystem exceeds %d file limit", wasmMaxFileCount)
	}
	fs.ensureParents(name)
	fs.files[name] = wasmFile{data: append([]byte(nil), data...), modified: time.Now()}
	fs.bytes += int64(len(data)) - old
	return nil
}

func wasmInfoMap(name string, file *wasmFile, isDir bool) runtime.Value {
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	put := func(key string, value runtime.Value) {
		mo.Keys = append(mo.Keys, key)
		mo.Vals[key] = value
	}
	put("name", runtime.Str(path.Base(name)))
	if isDir {
		put("size", runtime.Int(0))
		put("mtime", runtime.Int(time.Now().Unix()))
		put("mode", runtime.Int(0o755))
	} else {
		put("size", runtime.Int(int64(len(file.data))))
		put("mtime", runtime.Int(file.modified.Unix()))
		put("mode", runtime.Int(0o644))
	}
	put("is_dir", runtime.Bool(isDir))
	put("is_file", runtime.Bool(!isDir))
	return m
}

func packageFS() runtime.Value {
	p := pkg()

	set(p, "read", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return wasmError("read", "", "path required"), nil
		}
		name := wasmPath(args[0].String())
		wasmFS.mu.RLock()
		file, ok := wasmFS.files[name]
		_, dir := wasmFS.dirs[name]
		wasmFS.mu.RUnlock()
		if dir {
			return wasmError("read", name, "is a directory"), nil
		}
		if !ok {
			return wasmError("read", name, "file not found"), nil
		}
		return runtime.Ok(runtime.Str(string(file.data))), nil
	}, 1)
	set(p, "write", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return wasmError("write", "", "path and text required"), nil
		}
		return wasmWrite(args[0].String(), []byte(args[1].String()), "write"), nil
	}, 2)
	set(p, "read_bytes", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return wasmError("read_bytes", "", "path required"), nil
		}
		return packageFSReadBytes(args[0].String()), nil
	}, 1)
	set(p, "write_bytes", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return wasmError("write_bytes", "", "path and data required"), nil
		}
		return wasmWrite(args[0].String(), []byte(args[1].String()), "write_bytes"), nil
	}, 2)
	set(p, "append", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return wasmError("append", "", "path and text required"), nil
		}
		name := wasmPath(args[0].String())
		wasmFS.mu.Lock()
		old := append([]byte(nil), wasmFS.files[name].data...)
		err := wasmStoreBytes(wasmFS, name, append(old, []byte(args[1].String())...))
		wasmFS.mu.Unlock()
		if err != nil {
			return wasmError("append", name, err.Error()), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	}, 2)

	set(p, "stem", func(args []runtime.Value) (runtime.Value, error) { return runtime.Str(wasmStem(args)), nil }, 1)
	set(p, "base", func(args []runtime.Value) (runtime.Value, error) { return runtime.Str(wasmBase(args)), nil }, 1)
	set(p, "dir", func(args []runtime.Value) (runtime.Value, error) { return runtime.Str(wasmDir(args)), nil }, 1)
	set(p, "ext", func(args []runtime.Value) (runtime.Value, error) { return runtime.Str(path.Ext(wasmArg(args))), nil }, 1)
	set(p, "join", func(args []runtime.Value) (runtime.Value, error) {
		parts := make([]string, len(args))
		for i, arg := range args {
			parts[i] = arg.String()
		}
		return runtime.Str(wasmPath(path.Join(parts...))), nil
	}, -1)
	set(p, "norm", func(args []runtime.Value) (runtime.Value, error) { return runtime.Str(wasmPath(wasmArg(args))), nil }, 1)
	set(p, "cwd", func(args []runtime.Value) (runtime.Value, error) { return runtime.Str("."), nil }, 0)
	set(p, "abs", func(args []runtime.Value) (runtime.Value, error) {
		name := wasmPath(wasmArg(args))
		if name == "." {
			return runtime.Ok(runtime.Str("/")), nil
		}
		if strings.HasPrefix(name, "/") {
			return runtime.Ok(runtime.Str(name)), nil
		}
		return runtime.Ok(runtime.Str("/" + name)), nil
	}, 1)
	set(p, "with_suffix", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.Str(""), nil
		}
		name, suffix := args[0].String(), args[1].String()
		name = strings.TrimSuffix(name, path.Ext(name))
		if suffix != "" && !strings.HasPrefix(suffix, ".") {
			suffix = "." + suffix
		}
		return runtime.Str(name + suffix), nil
	}, 2)
	set(p, "parents", func(args []runtime.Value) (runtime.Value, error) {
		name := wasmPath(wasmArg(args))
		var parents []runtime.Value
		for parent := wasmParent(name); parent != "."; parent = wasmParent(parent) {
			parents = append(parents, runtime.Str(parent))
		}
		return runtime.List(parents...), nil
	}, 1)

	set(p, "exists", func(args []runtime.Value) (runtime.Value, error) {
		name := wasmPath(wasmArg(args))
		wasmFS.mu.RLock()
		_, f := wasmFS.files[name]
		_, d := wasmFS.dirs[name]
		wasmFS.mu.RUnlock()
		return runtime.Bool(f || d), nil
	}, 1)
	set(p, "is_file", func(args []runtime.Value) (runtime.Value, error) {
		name := wasmPath(wasmArg(args))
		wasmFS.mu.RLock()
		_, ok := wasmFS.files[name]
		wasmFS.mu.RUnlock()
		return runtime.Bool(ok), nil
	}, 1)
	set(p, "is_dir", func(args []runtime.Value) (runtime.Value, error) {
		name := wasmPath(wasmArg(args))
		wasmFS.mu.RLock()
		_, ok := wasmFS.dirs[name]
		wasmFS.mu.RUnlock()
		return runtime.Bool(ok), nil
	}, 1)
	set(p, "list", func(args []runtime.Value) (runtime.Value, error) { return wasmList(wasmArgDefault(args, ".")), nil }, 1)
	set(p, "mkdir", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return wasmError("mkdir", "", "path required"), nil
		}
		name := wasmPath(args[0].String())
		wasmFS.mu.Lock()
		wasmFS.ensureParents(name)
		wasmFS.dirs[name] = struct{}{}
		wasmFS.mu.Unlock()
		return runtime.Ok(runtime.Unit()), nil
	}, 1)
	set(p, "remove", func(args []runtime.Value) (runtime.Value, error) { return wasmRemove(wasmArg(args), false), nil }, 1)
	set(p, "remove_all", func(args []runtime.Value) (runtime.Value, error) { return wasmRemove(wasmArg(args), true), nil }, 1)
	set(p, "copy", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return wasmError("copy", "", "source and destination required"), nil
		}
		wasmFS.mu.RLock()
		file, ok := wasmFS.files[wasmPath(args[0].String())]
		wasmFS.mu.RUnlock()
		if !ok {
			return wasmError("copy", args[0].String(), "file not found"), nil
		}
		return wasmWrite(args[1].String(), file.data, "copy"), nil
	}, 2)
	set(p, "replace", func(args []runtime.Value) (runtime.Value, error) { return wasmRename(args, "replace"), nil }, 2)
	set(p, "rename", func(args []runtime.Value) (runtime.Value, error) { return wasmRename(args, "rename"), nil }, 2)
	set(p, "write_atomic", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return wasmError("write_atomic", "", "path and text required"), nil
		}
		return wasmWrite(args[0].String(), []byte(args[1].String()), "write_atomic"), nil
	}, 2)
	set(p, "lines", func(args []runtime.Value) (runtime.Value, error) { return packageFSReadLines(wasmArg(args)), nil }, 1)
	set(p, "glob", func(args []runtime.Value) (runtime.Value, error) { return wasmGlob(wasmArg(args)), nil }, 1)
	set(p, "stat", func(args []runtime.Value) (runtime.Value, error) { return wasmStat(wasmArg(args)), nil }, 1)
	set(p, "size", func(args []runtime.Value) (runtime.Value, error) { return wasmSize(wasmArg(args)), nil }, 1)
	set(p, "chmod", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return wasmError("chmod", "", "path and mode required"), nil
		}
		name := wasmPath(args[0].String())
		wasmFS.mu.RLock()
		_, ok := wasmFS.files[name]
		_, dir := wasmFS.dirs[name]
		wasmFS.mu.RUnlock()
		if !ok && !dir {
			return wasmError("chmod", name, "path not found"), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	}, 2)
	set(p, "walk", func(args []runtime.Value) (runtime.Value, error) { return wasmWalk(wasmArg(args)), nil }, 1)
	set(p, "rel", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return wasmError("rel", "", "path required"), nil
		}
		base := "."
		if len(args) > 1 {
			base = args[1].String()
		}
		value, err := filepath.Rel(wasmPath(base), wasmPath(args[0].String()))
		if err != nil {
			return wasmError("rel", args[0].String(), err.Error()), nil
		}
		return runtime.Ok(runtime.Str(value)), nil
	}, 2)
	set(p, "splitext", func(args []runtime.Value) (runtime.Value, error) {
		name := wasmArg(args)
		ext := path.Ext(name)
		return runtime.List(runtime.Str(strings.TrimSuffix(name, ext)), runtime.Str(ext)), nil
	}, 1)
	set(p, "expanduser", func(args []runtime.Value) (runtime.Value, error) { return runtime.Str(wasmArg(args)), nil }, 1)
	set(p, "temp_dir", func(args []runtime.Value) (runtime.Value, error) {
		return wasmTemp(true, wasmArgDefault(args, "weft-")), nil
	}, 1)
	set(p, "temp_file", func(args []runtime.Value) (runtime.Value, error) {
		return wasmTemp(false, wasmArgDefault(args, "weft-")), nil
	}, 1)

	return p
}

func wasmArg(args []runtime.Value) string {
	if len(args) == 0 {
		return ""
	}
	return args[0].String()
}
func wasmArgDefault(args []runtime.Value, fallback string) string {
	if len(args) == 0 || args[0].String() == "" {
		return fallback
	}
	return args[0].String()
}
func wasmBase(args []runtime.Value) string { return path.Base(wasmArg(args)) }
func wasmDir(args []runtime.Value) string  { return path.Dir(wasmPath(wasmArg(args))) }
func wasmStem(args []runtime.Value) string {
	name := wasmBase(args)
	return strings.TrimSuffix(name, path.Ext(name))
}

func wasmWrite(raw string, data []byte, op string) runtime.Value {
	name := wasmPath(raw)
	wasmFS.mu.Lock()
	defer wasmFS.mu.Unlock()
	if _, isDir := wasmFS.dirs[name]; isDir {
		return wasmError(op, name, "is a directory")
	}
	if err := wasmStoreBytes(wasmFS, name, data); err != nil {
		return wasmError(op, name, err.Error())
	}
	return runtime.Ok(runtime.Unit())
}

func packageFSReadBytes(raw string) runtime.Value { return packageFSRead(raw, "read_bytes") }
func packageFSRead(raw, op string) runtime.Value {
	name := wasmPath(raw)
	wasmFS.mu.RLock()
	file, ok := wasmFS.files[name]
	wasmFS.mu.RUnlock()
	if !ok {
		return wasmError(op, name, "file not found")
	}
	return runtime.Ok(runtime.Str(string(file.data)))
}
func packageFSReadLines(raw string) runtime.Value {
	value := packageFSRead(raw, "lines")
	if value.Kind == runtime.KindResult && !value.Obj.(*runtime.ResultObj).Ok {
		return value
	}
	result := value.Obj.(*runtime.ResultObj)
	text := strings.TrimSuffix(strings.ReplaceAll(result.Val.S, "\r\n", "\n"), "\n")
	if text == "" {
		return runtime.Ok(runtime.List())
	}
	parts := strings.Split(text, "\n")
	items := make([]runtime.Value, len(parts))
	for i, part := range parts {
		items[i] = runtime.Str(part)
	}
	return runtime.Ok(runtime.List(items...))
}

func wasmList(raw string) runtime.Value {
	dir := wasmPath(raw)
	wasmFS.mu.RLock()
	defer wasmFS.mu.RUnlock()
	if _, ok := wasmFS.dirs[dir]; !ok {
		return wasmError("list", dir, "directory not found")
	}
	seen := map[string]bool{}
	prefix := dir
	if prefix == "." {
		prefix = ""
	}
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	for name := range wasmFS.files {
		if strings.HasPrefix(name, prefix) {
			rest := strings.TrimPrefix(name, prefix)
			if !strings.Contains(rest, "/") {
				seen[rest] = true
			}
		}
	}
	for name := range wasmFS.dirs {
		if name != dir && strings.HasPrefix(name, prefix) {
			rest := strings.TrimPrefix(name, prefix)
			if !strings.Contains(rest, "/") {
				seen[rest] = true
			}
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	items := make([]runtime.Value, len(names))
	for i, name := range names {
		items[i] = runtime.Str(name)
	}
	return runtime.Ok(runtime.List(items...))
}

func wasmRemove(raw string, all bool) runtime.Value {
	name := wasmPath(raw)
	if name == "." {
		return wasmError("remove", name, "cannot remove virtual root")
	}
	wasmFS.mu.Lock()
	defer wasmFS.mu.Unlock()
	if file, ok := wasmFS.files[name]; ok {
		delete(wasmFS.files, name)
		wasmFS.bytes -= int64(len(file.data))
		return runtime.Ok(runtime.Unit())
	}
	if _, ok := wasmFS.dirs[name]; !ok {
		return wasmError("remove", name, "path not found")
	}
	if !all {
		for child := range wasmFS.files {
			if wasmParent(child) == name {
				return wasmError("remove", name, "directory not empty")
			}
		}
		for child := range wasmFS.dirs {
			if child != name && wasmParent(child) == name {
				return wasmError("remove", name, "directory not empty")
			}
		}
		delete(wasmFS.dirs, name)
		return runtime.Ok(runtime.Unit())
	}
	for file := range wasmFS.files {
		if file == name || strings.HasPrefix(file, name+"/") {
			wasmFS.bytes -= int64(len(wasmFS.files[file].data))
			delete(wasmFS.files, file)
		}
	}
	for dir := range wasmFS.dirs {
		if dir == name || strings.HasPrefix(dir, name+"/") {
			delete(wasmFS.dirs, dir)
		}
	}
	return runtime.Ok(runtime.Unit())
}

func wasmRename(args []runtime.Value, op string) runtime.Value {
	if len(args) < 2 {
		return wasmError(op, "", "source and destination required")
	}
	src, dst := wasmPath(args[0].String()), wasmPath(args[1].String())
	if src == dst {
		return runtime.Ok(runtime.Unit())
	}
	wasmFS.mu.Lock()
	defer wasmFS.mu.Unlock()
	file, ok := wasmFS.files[src]
	if !ok {
		return wasmError(op, src, "file not found")
	}
	if _, isDir := wasmFS.dirs[dst]; isDir {
		return wasmError(op, dst, "is a directory")
	}
	if old, exists := wasmFS.files[dst]; exists {
		wasmFS.bytes -= int64(len(old.data))
	}
	delete(wasmFS.files, src)
	wasmFS.files[dst] = file
	wasmFS.ensureParents(dst)
	return runtime.Ok(runtime.Unit())
}

func wasmStat(raw string) runtime.Value {
	name := wasmPath(raw)
	wasmFS.mu.RLock()
	defer wasmFS.mu.RUnlock()
	if file, ok := wasmFS.files[name]; ok {
		return runtime.Ok(wasmInfoMap(name, &file, false))
	}
	if _, ok := wasmFS.dirs[name]; ok {
		return runtime.Ok(wasmInfoMap(name, nil, true))
	}
	return wasmError("stat", name, "path not found")
}
func wasmSize(raw string) runtime.Value {
	value := wasmStat(raw)
	if value.Kind == runtime.KindResult && !value.Obj.(*runtime.ResultObj).Ok {
		return value
	}
	result := value.Obj.(*runtime.ResultObj)
	return runtime.Ok(runtime.Int(result.Val.Obj.(*runtime.MapObj).Vals["size"].I))
}

func wasmWalk(raw string) runtime.Value {
	root := wasmPath(raw)
	wasmFS.mu.RLock()
	defer wasmFS.mu.RUnlock()
	if _, d := wasmFS.dirs[root]; !d {
		if _, f := wasmFS.files[root]; !f {
			return wasmError("walk", root, "path not found")
		}
	}
	paths := []string{}
	if _, ok := wasmFS.dirs[root]; ok {
		paths = append(paths, root)
	}
	prefix := root + "/"
	if root == "." {
		prefix = ""
	}
	for name := range wasmFS.files {
		if name == root || strings.HasPrefix(name, prefix) {
			paths = append(paths, name)
		}
	}
	for name := range wasmFS.dirs {
		if name != root && strings.HasPrefix(name, prefix) {
			paths = append(paths, name)
		}
	}
	sort.Strings(paths)
	items := make([]runtime.Value, 0, len(paths))
	for _, name := range paths {
		if file, ok := wasmFS.files[name]; ok {
			items = append(items, wasmInfoMap(name, &file, false))
		} else {
			items = append(items, wasmInfoMap(name, nil, true))
		}
	}
	return runtime.Ok(runtime.List(items...))
}

func wasmGlob(pattern string) runtime.Value {
	pattern = wasmPath(pattern)
	wasmFS.mu.RLock()
	defer wasmFS.mu.RUnlock()
	matches := []string{}
	for name := range wasmFS.files {
		if ok, _ := path.Match(pattern, name); ok {
			matches = append(matches, name)
		}
	}
	for name := range wasmFS.dirs {
		if name != "." {
			if ok, _ := path.Match(pattern, name); ok {
				matches = append(matches, name)
			}
		}
	}
	sort.Strings(matches)
	items := make([]runtime.Value, len(matches))
	for i, name := range matches {
		items[i] = runtime.Str(name)
	}
	return runtime.Ok(runtime.List(items...))
}

func wasmTemp(directory bool, prefix string) runtime.Value {
	id := atomic.AddUint64(&wasmTempID, 1)
	name := wasmPath("/tmp/" + prefix + fmt.Sprint(id))
	wasmFS.mu.Lock()
	defer wasmFS.mu.Unlock()
	wasmFS.ensureParents(name)
	if directory {
		wasmFS.dirs[name] = struct{}{}
	} else if err := wasmStoreBytes(wasmFS, name, nil); err != nil {
		return wasmError("temp_file", name, err.Error())
	}
	return runtime.Ok(runtime.Str(name))
}
