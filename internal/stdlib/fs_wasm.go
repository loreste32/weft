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
	wasmMaxDirCount   = 5000
	wasmMaxEntries    = 15000 // files + dirs combined
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

var errWasmTraversal = fmt.Errorf("path escapes virtual filesystem root")

func wasmPath(raw string) (string, error) {
	raw = strings.ReplaceAll(raw, "\\", "/")
	if len(raw) > 4096 {
		return "", fmt.Errorf("path exceeds 4096 character limit")
	}
	clean := path.Clean(raw)
	if clean == "." || clean == "" || clean == "/" {
		return ".", nil
	}
	clean = strings.TrimPrefix(clean, "./")
	// Reject path traversal — any path escaping the virtual root
	if clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
		return "", errWasmTraversal
	}
	// Strip leading slash (everything is relative to virtual root)
	clean = strings.TrimPrefix(clean, "/")
	if clean == "" {
		return ".", nil
	}
	return clean, nil
}

// wasmMustPath calls wasmPath and returns an error Result on traversal.
func wasmMustPath(raw, op string) (string, runtime.Value) {
	name, err := wasmPath(raw)
	if err != nil {
		return "", wasmError(op, raw, err.Error())
	}
	return name, runtime.Value{}
}

func wasmParent(name string) string {
	parent := path.Dir(name)
	if parent == "/" {
		return "."
	}
	p, err := wasmPath(parent)
	if err != nil {
		return "."
	}
	return p
}

func (fs *wasmFilesystem) missingParents(name string) ([]string, error) {
	parents := []string{}
	for parent := wasmParent(name); parent != "."; parent = wasmParent(parent) {
		if _, isFile := fs.files[parent]; isFile {
			return nil, fmt.Errorf("parent path is a file")
		}
		if _, exists := fs.dirs[parent]; !exists {
			parents = append(parents, parent)
		}
	}
	return parents, nil
}

func (fs *wasmFilesystem) checkCapacity(addFiles, addDirs int) error {
	if len(fs.files)+addFiles > wasmMaxFileCount {
		return fmt.Errorf("virtual filesystem exceeds %d file limit", wasmMaxFileCount)
	}
	if len(fs.dirs)+addDirs > wasmMaxDirCount {
		return fmt.Errorf("virtual filesystem exceeds %d directory limit", wasmMaxDirCount)
	}
	if len(fs.files)+len(fs.dirs)+addFiles+addDirs > wasmMaxEntries {
		return fmt.Errorf("virtual filesystem exceeds %d total entry limit", wasmMaxEntries)
	}
	return nil
}

func (fs *wasmFilesystem) applyParents(parents []string) {
	for i := len(parents) - 1; i >= 0; i-- {
		fs.dirs[parents[i]] = struct{}{}
	}
}

// ensureParents validates the complete parent plan before mutating the map.
// Callers must hold fs.mu for writing.
func (fs *wasmFilesystem) ensureParents(name string) error {
	parents, err := fs.missingParents(name)
	if err != nil {
		return err
	}
	if err := fs.checkCapacity(0, len(parents)); err != nil {
		return err
	}
	fs.applyParents(parents)
	return nil
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
	if _, isDir := fs.dirs[name]; isDir {
		return fmt.Errorf("path is a directory")
	}
	parents, err := fs.missingParents(name)
	if err != nil {
		return err
	}
	addFile := 0
	if _, exists := fs.files[name]; !exists {
		addFile = 1
	}
	if err := fs.checkCapacity(addFile, len(parents)); err != nil {
		return err
	}
	fs.applyParents(parents)
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
		name, errVal := wasmMustPath(args[0].String(), "read")
		if errVal.Kind != 0 {
			return errVal, nil
		}
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
		name, pErr := wasmPath(args[0].String())
		if pErr != nil {
			return wasmError("append", args[0].String(), pErr.Error()), nil
		}
		wasmFS.mu.Lock()
		old := append([]byte(nil), wasmFS.files[name].data...)
		err := wasmStoreBytes(wasmFS, name, append(old, []byte(args[1].String())...))
		wasmFS.mu.Unlock()
		if err != nil {
			return wasmError("append", name, err.Error()), nil
		}
		return runtime.Ok(runtime.Unit()), nil
	}, 2)

	set(p, "stem", func(args []runtime.Value) (runtime.Value, error) { return wasmStem(args), nil }, 1)
	set(p, "base", func(args []runtime.Value) (runtime.Value, error) { return wasmBase(args), nil }, 1)
	set(p, "dir", func(args []runtime.Value) (runtime.Value, error) { return wasmDir(args), nil }, 1)
	set(p, "ext", func(args []runtime.Value) (runtime.Value, error) {
		name, pErr := wasmPath(wasmArg(args))
		if pErr != nil {
			return wasmError("ext", wasmArg(args), pErr.Error()), nil
		}
		return runtime.Str(path.Ext(name)), nil
	}, 1)
	set(p, "join", func(args []runtime.Value) (runtime.Value, error) {
		parts := make([]string, len(args))
		for i, arg := range args {
			parts[i] = arg.String()
		}
		p, pv := wasmMustPath(path.Join(parts...), "join")
		if pv.Kind != 0 {
			return pv, nil
		}
		return runtime.Str(p), nil
	}, -1)
	set(p, "norm", func(args []runtime.Value) (runtime.Value, error) {
		p, pv := wasmMustPath(wasmArg(args), "norm")
		if pv.Kind != 0 {
			return pv, nil
		}
		return runtime.Str(p), nil
	}, 1)
	set(p, "cwd", func(args []runtime.Value) (runtime.Value, error) { return runtime.Str("."), nil }, 0)
	set(p, "abs", func(args []runtime.Value) (runtime.Value, error) {
		name, pv := wasmMustPath(wasmArg(args), "fs")
		if pv.Kind != 0 {
			return pv, nil
		}
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
		name, pErr := wasmPath(args[0].String())
		if pErr != nil {
			return wasmError("with_suffix", args[0].String(), pErr.Error()), nil
		}
		suffix := args[1].String()
		name = strings.TrimSuffix(name, path.Ext(name))
		if suffix != "" && !strings.HasPrefix(suffix, ".") {
			suffix = "." + suffix
		}
		return runtime.Str(name + suffix), nil
	}, 2)
	set(p, "parents", func(args []runtime.Value) (runtime.Value, error) {
		name, pv := wasmMustPath(wasmArg(args), "fs")
		if pv.Kind != 0 {
			return pv, nil
		}
		var parents []runtime.Value
		for parent := wasmParent(name); parent != "."; parent = wasmParent(parent) {
			parents = append(parents, runtime.Str(parent))
		}
		return runtime.List(parents...), nil
	}, 1)

	set(p, "exists", func(args []runtime.Value) (runtime.Value, error) {
		name, pv := wasmMustPath(wasmArg(args), "fs")
		if pv.Kind != 0 {
			return pv, nil
		}
		wasmFS.mu.RLock()
		_, f := wasmFS.files[name]
		_, d := wasmFS.dirs[name]
		wasmFS.mu.RUnlock()
		return runtime.Bool(f || d), nil
	}, 1)
	set(p, "is_file", func(args []runtime.Value) (runtime.Value, error) {
		name, pv := wasmMustPath(wasmArg(args), "fs")
		if pv.Kind != 0 {
			return pv, nil
		}
		wasmFS.mu.RLock()
		_, ok := wasmFS.files[name]
		wasmFS.mu.RUnlock()
		return runtime.Bool(ok), nil
	}, 1)
	set(p, "is_dir", func(args []runtime.Value) (runtime.Value, error) {
		name, pv := wasmMustPath(wasmArg(args), "fs")
		if pv.Kind != 0 {
			return pv, nil
		}
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
		name, pErr := wasmPath(args[0].String())
		if pErr != nil {
			return wasmError("mkdir", args[0].String(), pErr.Error()), nil
		}
		wasmFS.mu.Lock()
		if _, isFile := wasmFS.files[name]; isFile {
			wasmFS.mu.Unlock()
			return wasmError("mkdir", name, "path is a file"), nil
		}
		if _, exists := wasmFS.dirs[name]; exists {
			wasmFS.mu.Unlock()
			return runtime.Ok(runtime.Unit()), nil
		}
		parents, pErr := wasmFS.missingParents(name)
		if pErr != nil {
			wasmFS.mu.Unlock()
			return wasmError("mkdir", name, pErr.Error()), nil
		}
		if epErr := wasmFS.checkCapacity(0, len(parents)+1); epErr != nil {
			wasmFS.mu.Unlock()
			return wasmError("mkdir", args[0].String(), epErr.Error()), nil
		}
		wasmFS.applyParents(parents)
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
		p, pErr := wasmPath(args[0].String())
		if pErr != nil {
			return wasmError("copy", args[0].String(), pErr.Error()), nil
		}
		wasmFS.mu.RLock()
		file, ok := wasmFS.files[p]
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
		name, pErr := wasmPath(args[0].String())
		if pErr != nil {
			return wasmError("chmod", args[0].String(), pErr.Error()), nil
		}
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
		value, err := func() (string, error) {
			b, bv := wasmMustPath(base, "rel")
			if bv.Kind != 0 {
				return "", fmt.Errorf("rel: invalid base")
			}
			a, av := wasmMustPath(args[0].String(), "rel")
			if av.Kind != 0 {
				return "", fmt.Errorf("rel: invalid path")
			}
			return filepath.Rel(b, a)
		}()
		if err != nil {
			return wasmError("rel", args[0].String(), err.Error()), nil
		}
		return runtime.Ok(runtime.Str(value)), nil
	}, 2)
	set(p, "splitext", func(args []runtime.Value) (runtime.Value, error) {
		name, pErr := wasmPath(wasmArg(args))
		if pErr != nil {
			return wasmError("splitext", wasmArg(args), pErr.Error()), nil
		}
		ext := path.Ext(name)
		return runtime.List(runtime.Str(strings.TrimSuffix(name, ext)), runtime.Str(ext)), nil
	}, 1)
	set(p, "expanduser", func(args []runtime.Value) (runtime.Value, error) {
		name, pErr := wasmPath(wasmArg(args))
		if pErr != nil {
			return wasmError("expanduser", wasmArg(args), pErr.Error()), nil
		}
		return runtime.Str(name), nil
	}, 1)
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
func wasmBase(args []runtime.Value) runtime.Value {
	p, err := wasmPath(wasmArg(args))
	if err != nil {
		return wasmError("base", wasmArg(args), err.Error())
	}
	return runtime.Str(path.Base(p))
}
func wasmDir(args []runtime.Value) runtime.Value {
	p, err := wasmPath(wasmArg(args))
	if err != nil {
		return wasmError("dir", wasmArg(args), err.Error())
	}
	return runtime.Str(path.Dir(p))
}
func wasmStem(args []runtime.Value) runtime.Value {
	name, err := wasmPath(wasmArg(args))
	if err != nil {
		return wasmError("stem", wasmArg(args), err.Error())
	}
	base := path.Base(name)
	return runtime.Str(strings.TrimSuffix(base, path.Ext(base)))
}

func wasmWrite(raw string, data []byte, op string) runtime.Value {
	name, errVal := wasmMustPath(raw, op)
	if errVal.Kind != 0 {
		return errVal
	}
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
	name, pErr := wasmPath(raw)
	if pErr != nil {
		return wasmError("fs", raw, pErr.Error())
	}
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
	dir, pErr := wasmPath(raw)
	if pErr != nil {
		return wasmError("list", raw, pErr.Error())
	}
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
	name, pErr := wasmPath(raw)
	if pErr != nil {
		return wasmError("fs", raw, pErr.Error())
	}
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
	src, sErr := wasmPath(args[0].String())
	if sErr != nil {
		return wasmError(op, args[0].String(), sErr.Error())
	}
	dst, dErr := wasmPath(args[1].String())
	if dErr != nil {
		return wasmError(op, args[1].String(), dErr.Error())
	}
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
	parents, pErr := wasmFS.missingParents(dst)
	if pErr != nil {
		return wasmError(op, args[1].String(), pErr.Error())
	}
	if epErr := wasmFS.checkCapacity(0, len(parents)); epErr != nil {
		return wasmError(op, args[1].String(), epErr.Error())
	}
	wasmFS.applyParents(parents)
	if old, exists := wasmFS.files[dst]; exists {
		wasmFS.bytes -= int64(len(old.data))
	}
	delete(wasmFS.files, src)
	wasmFS.files[dst] = file
	return runtime.Ok(runtime.Unit())
}

func wasmStat(raw string) runtime.Value {
	name, pErr := wasmPath(raw)
	if pErr != nil {
		return wasmError("fs", raw, pErr.Error())
	}
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
	root, pErr := wasmPath(raw)
	if pErr != nil {
		return wasmError("walk", raw, pErr.Error())
	}
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
	p2, pErr := wasmPath(pattern)
	if pErr != nil {
		return wasmError("glob", pattern, pErr.Error())
	}
	pattern = p2
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
	name, pErr := wasmPath("/tmp/" + prefix + fmt.Sprint(id))
	if pErr != nil {
		return wasmError("temp", prefix, pErr.Error())
	}
	wasmFS.mu.Lock()
	defer wasmFS.mu.Unlock()
	parents, pErr := wasmFS.missingParents(name)
	if pErr != nil {
		return wasmError("temp", prefix, pErr.Error())
	}
	addDirs := len(parents)
	if directory {
		if _, exists := wasmFS.dirs[name]; !exists {
			addDirs++
		}
	}
	addFiles := 0
	if !directory {
		if _, exists := wasmFS.files[name]; !exists {
			addFiles = 1
		}
	}
	if err := wasmFS.checkCapacity(addFiles, addDirs); err != nil {
		return wasmError("temp", prefix, err.Error())
	}
	wasmFS.applyParents(parents)
	if directory {
		wasmFS.dirs[name] = struct{}{}
	} else if addFiles == 1 {
		wasmFS.files[name] = wasmFile{modified: time.Now()}
	}
	return runtime.Ok(runtime.Str(name))
}
