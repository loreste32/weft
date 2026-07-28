// Package compile lowers AST to bytecode.
package compile

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/loreste/weft/internal/ast"
	"github.com/loreste/weft/internal/diag"
	"github.com/loreste/weft/internal/parse"
	"github.com/loreste/weft/internal/pkgman"
	"github.com/loreste/weft/internal/runtime"
	"github.com/loreste/weft/internal/stdlib"
	"github.com/loreste/weft/internal/token"
)

// os used by pathWithin

// Program is a compiled Weft module.
type Program struct {
	Main    *runtime.FuncObj
	Funcs   map[string]*runtime.FuncObj
	Globals []string
	File    string
}

// Compiler compiles a file.
type Compiler struct {
	file          string
	errs          diag.List
	chunk         *Chunk
	locals        []local
	loopBreaks    [][]int
	loopContinues [][]int
	returnsResult bool
	env           *runtime.Env
	// replMode: let bindings also write globals so REPL state persists
	replMode bool
	// requireMain: CompileFile requires main; CompileFileREPL does not
	requireMain bool
	anonID      int
	// blockReturned: last expr in block already emitted RETURN
	blockReturned bool
}

type local struct {
	name string
	mut  bool
	// const binding: no reassignment and no in-place mutation through this name
	isConst bool
}

// CompileFile compiles an AST file into a Program (requires main).
func CompileFile(file *ast.File, env *runtime.Env) (*Program, diag.List) {
	return compileFile(file, env, true, false)
}

// CompileFileLib compiles a library/test module — no main required.
func CompileFileLib(file *ast.File, env *runtime.Env) (*Program, diag.List) {
	return compileFile(file, env, false, false)
}

// CompileFileREPL compiles for the REPL: no main required; __repl persists lets as globals.
func CompileFileREPL(file *ast.File, env *runtime.Env) (*Program, diag.List) {
	return compileFile(file, env, false, true)
}

func compileFile(file *ast.File, env *runtime.Env, requireMain, replMode bool) (*Program, diag.List) {
	c := &Compiler{file: file.Path, env: env, requireMain: requireMain, replMode: replMode}
	prog := &Program{Funcs: make(map[string]*runtime.FuncObj), File: file.Path}

	for _, d := range file.Decls {
		switch d := d.(type) {
		case *ast.FnDecl:
			fn, errs := c.compileFn(d)
			c.errs = append(c.errs, errs...)
			// Module home env for path-imported functions
			if env != nil {
				fn.Env = env
			}
			prog.Funcs[d.Name] = fn
			prog.Globals = append(prog.Globals, d.Name)
			if d.Name == "main" {
				prog.Main = fn
			}
			env.Set(d.Name, runtime.Func(fn))
		case *ast.ImportDecl:
			c.compileImport(d)
		case *ast.TypeDecl:
			ti := c.typeInfoFromDecl(d)
			env.Types[d.Name] = ti
			env.Set(d.Name, runtime.TypeInfoValue(ti))
		case *ast.ConstDecl:
			if lit, ok := d.Value.(*ast.BasicLit); ok {
				v, err := litValue(lit)
				if err != nil {
					c.errorf(d.Pos(), "%v", err)
				} else {
					env.Set(d.Name, v)
				}
			} else {
				c.errorf(d.Pos(), "top-level const must be a literal")
			}
		case *ast.EnumDecl:
			// enum Status { Ok, Err } → global map Status with Status.Ok == "Ok"
			// enum Shape { Circle(r), Point } → Shape.Circle(r) returns tagged struct
			m := runtime.NewMap()
			mo := m.Obj.(*runtime.MapObj)
			seen := map[string]bool{}
			for _, pv := range d.Payloads {
				v := pv.Name
				if v == "" || seen[v] {
					if seen[v] {
						c.errorf(d.Pos(), "duplicate enum variant %s.%s", d.Name, v)
					}
					continue
				}
				seen[v] = true
				mo.Keys = append(mo.Keys, v)
				if len(pv.Fields) == 0 && !enumHasPayloads(d) {
					// Pure unit enum: Status.Ok == "Ok" (backward compat)
					mo.Vals[v] = runtime.Str(v)
				} else if len(pv.Fields) == 0 {
					// Unit variant in a mixed enum: wrap as tagged struct for consistent matching
					enumName := d.Name
					variantName := v
					mo.Vals[v] = runtime.Struct(enumName, map[string]runtime.Value{
						"_tag": runtime.Str(enumName + "." + variantName),
					}, []string{"_tag"})
				} else {
					// Payload variant: Shape.Circle(5) → struct
					enumName := d.Name
					fields := pv.Fields
					variantName := v
					mo.Vals[v] = runtime.MakeBuiltin(d.Name+"."+v, len(fields), func(args []runtime.Value) (runtime.Value, error) {
						fmap := map[string]runtime.Value{
							"_tag": runtime.Str(enumName + "." + variantName),
						}
						order := []string{"_tag"}
						for i, fname := range fields {
							if i < len(args) {
								fmap[fname] = args[i]
							} else {
								fmap[fname] = runtime.Null()
							}
							order = append(order, fname)
						}
						return runtime.Struct(enumName, fmap, order), nil
					})
				}
			}
			env.Set(d.Name, m)
			prog.Globals = append(prog.Globals, d.Name)
		}
	}

	if requireMain && prog.Main == nil {
		c.errorf(token.Pos{Line: 1, Column: 1}, "no main function")
	}
	return prog, c.errs
}

func (c *Compiler) compileImport(d *ast.ImportDecl) {
	if d.IsPath {
		// string path: "./x.weft" or "pkgname" (package import as string)
		if !strings.HasPrefix(d.Path, ".") && !strings.HasPrefix(d.Path, "/") && !strings.Contains(d.Path, string(filepath.Separator)) && !strings.HasSuffix(d.Path, ".weft") && !strings.HasSuffix(d.Path, ".loom") {
			// import "hello" — package manager package
			c.loadNamedPackage(d, d.Path)
			return
		}
		c.loadPathModule(d)
		return
	}
	// bare ident: stdlib (registry) or installed package
	if stdlib.IsPackage(d.Path) {
		if _, ok := c.env.Get(d.Path); !ok {
			c.errorf(d.Pos(), "capability denied: package %q not available in this module (declare capabilities in weft.json)", d.Path)
			return
		}
		name := d.Path
		if d.Alias != "" {
			name = d.Alias
			if pkg, ok := c.env.Get(d.Path); ok {
				c.env.Set(name, pkg)
			}
		}
		return
	}
	c.loadNamedPackage(d, d.Path)
}

func (c *Compiler) loadNamedPackage(d *ast.ImportDecl, name string) {
	project := c.env.ProjectDir
	if project == "" {
		project = c.projectRoot()
		c.env.ProjectDir = project
	}
	pkgDir, entry, err := pkgman.FindInstalledPackage(project, name)
	if err != nil {
		c.errorf(d.Pos(), "%v", err)
		return
	}
	// Reuse path loader against the package entry file; confine path imports to pkgDir.
	fake := &ast.ImportDecl{
		Pos_:   d.Pos_,
		Path:   entry,
		IsPath: true,
		Alias:  d.Alias,
	}
	if fake.Alias == "" {
		fake.Alias = name
	}
	prevRoot := c.env.PackageRoot
	c.env.PackageRoot = pkgDir
	c.loadPathModule(fake)
	c.env.PackageRoot = prevRoot
}

func (c *Compiler) projectRoot() string {
	start := filepath.Dir(c.file)
	if c.file == "" || c.file == "<repl>" {
		if wd, err := os.Getwd(); err == nil {
			start = wd
		}
	}
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, "weft.json")); err == nil {
			return dir
		}
		if _, err := os.Stat(filepath.Join(dir, "vendor")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return start
}

func (c *Compiler) loadPathModule(d *ast.ImportDecl) {
	base := filepath.Dir(c.file)
	if c.file == "" || c.file == "<repl>" || !filepath.IsAbs(c.file) && !fileExists(c.file) {
		if wd, err := os.Getwd(); err == nil {
			if base == "." || base == "" {
				base = wd
			} else if !filepath.IsAbs(base) {
				base = filepath.Join(wd, base)
			}
		}
	}
	path := d.Path
	if !filepath.IsAbs(path) {
		path = filepath.Join(base, path)
	}
	path = filepath.Clean(path)

	// Installed packages may not path-import outside their root (supply-chain / escape).
	if root := c.env.PackageRoot; root != "" {
		if !pathWithin(root, path) {
			c.errorf(d.Pos(), "path import %q escapes package root %s", d.Path, root)
			return
		}
	}

	if c.env.ModuleCache == nil {
		c.env.ModuleCache = make(map[string]runtime.Value)
	}
	bindName := d.Alias
	if bindName == "" {
		bindName = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		// sanitize: foo-bar -> need valid ident; keep as-is for map key / global name
		bindName = strings.ReplaceAll(bindName, "-", "_")
	}

	if mod, ok := c.env.ModuleCache[path]; ok {
		c.env.Set(bindName, mod)
		return
	}
	for _, p := range c.env.ImportStack {
		if p == path {
			c.errorf(d.Pos(), "import cycle involving %q", path)
			return
		}
	}
	src, err := os.ReadFile(path)
	if err != nil {
		c.errorf(d.Pos(), "cannot read module %q: %v", path, err)
		return
	}
	file, perrs := parse.ParseFile(path, string(src))
	if perrs.HasErrors() {
		c.errs = append(c.errs, perrs...)
		return
	}

	// Fresh env for module: share prelude/stdlib by copying package refs + Call hooks
	modEnv := cloneEnvForModule(c.env)
	// Third-party packages (PackageRoot set) get capability filtering.
	if c.env.PackageRoot != "" {
		applyPackageCapabilities(modEnv, c.env.PackageRoot)
	}
	c.env.ImportStack = append(c.env.ImportStack, path)
	prog, cerrs := compileFile(file, modEnv, false, false)
	c.env.ImportStack = c.env.ImportStack[:len(c.env.ImportStack)-1]
	if cerrs.HasErrors() {
		c.errs = append(c.errs, cerrs...)
		return
	}

	// Re-home every function on modEnv so internal calls resolve sibling globals.
	for name, fn := range prog.Funcs {
		fn.Env = modEnv
		modEnv.Set(name, runtime.Func(fn))
	}
	modEnv.Call = c.env.Call

	// Exports: if any `pub`, only those; else all fns except main (non-verbose default)
	hasPub := false
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FnDecl); ok && fn.Pub {
			hasPub = true
			break
		}
		if td, ok := decl.(*ast.TypeDecl); ok && td.Pub {
			hasPub = true
			break
		}
		if ed, ok := decl.(*ast.EnumDecl); ok && ed.Pub {
			hasPub = true
			break
		}
	}
	mod := runtime.NewMap()
	mo := mod.Obj.(*runtime.MapObj)
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FnDecl:
			if d.Name == "main" {
				continue
			}
			if hasPub && !d.Pub {
				continue
			}
			if fn, ok := prog.Funcs[d.Name]; ok {
				mo.Keys = append(mo.Keys, d.Name)
				mo.Vals[d.Name] = runtime.Func(fn)
			}
		case *ast.TypeDecl:
			if hasPub && !d.Pub {
				continue
			}
			if ti, ok := modEnv.Types[d.Name]; ok {
				mo.Keys = append(mo.Keys, d.Name)
				mo.Vals[d.Name] = runtime.TypeInfoValue(ti)
			}
		case *ast.EnumDecl:
			if hasPub && !d.Pub {
				continue
			}
			if v, ok := modEnv.Get(d.Name); ok {
				mo.Keys = append(mo.Keys, d.Name)
				mo.Vals[d.Name] = v
			}
		}
	}
	c.env.ModuleCache[path] = mod
	c.env.Set(bindName, mod)
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func cloneEnvForModule(parent *runtime.Env) *runtime.Env {
	// Fresh prelude, then overlay parent's host-shared symbols (stdlib + os/args…).
	// No hardcoded name list — Register/SetShared mark what modules inherit.
	mod := runtime.NewEnv()
	mod.Stdout = parent.Stdout
	mod.Stderr = parent.Stderr
	mod.HTTPClient = parent.HTTPClient
	mod.Environ = parent.Environ
	mod.LLMBaseURL = parent.LLMBaseURL
	mod.LLMDo = parent.LLMDo
	mod.Call = parent.Call
	mod.ModuleCache = parent.ModuleCache
	mod.ImportStack = parent.ImportStack
	mod.ProjectDir = parent.ProjectDir
	mod.PackageRoot = parent.PackageRoot
	mod.Ctx = parent.Ctx
	parent.CopyHostSharedInto(mod)
	return mod
}

// pathWithin reports whether target is root or a file/dir under root.
func pathWithin(root, target string) bool {
	root = filepath.Clean(root)
	target = filepath.Clean(target)
	if root == target {
		return true
	}
	sep := string(os.PathSeparator)
	return strings.HasPrefix(target, root+sep)
}

// applyPackageCapabilities revokes restricted stdlib packages unless granted
// in the package's weft.json capabilities field.
func applyPackageCapabilities(env *runtime.Env, pkgDir string) {
	caps := pkgman.LoadCapabilities(pkgDir)
	for _, r := range pkgman.RestrictedByDefault {
		if !pkgman.Allows(caps, r) {
			env.RevokeHost(r)
		}
	}
}

func (c *Compiler) errorf(pos token.Pos, format string, args ...any) {
	c.errs = append(c.errs, diag.Errorf(c.file, pos, format, args...))
}

func (c *Compiler) compileFn(d *ast.FnDecl) (*runtime.FuncObj, diag.List) {
	return c.compileFnWithFree(d, nil)
}

// compileFnWithFree compiles a function; free vars become locals after params (filled from Upvalues on call).
func (c *Compiler) compileFnWithFree(d *ast.FnDecl, free []string) (*runtime.FuncObj, diag.List) {
	// REPL frame functions persist bindings as globals
	repl := c.replMode && (d.Name == "__repl" || strings.HasPrefix(d.Name, "__repl"))
	sub := &Compiler{
		file:          c.file,
		env:           c.env,
		chunk:         &Chunk{Name: d.Name, Arity: len(d.Params), File: c.file},
		returnsResult: isResultType(d.Ret),
		replMode:      repl,
		anonID:        c.anonID,
	}
	for _, p := range d.Params {
		sub.locals = append(sub.locals, local{name: p.Name, mut: false})
	}
	// Captured names: immutable bindings of values frozen at closure creation.
	for _, name := range free {
		sub.locals = append(sub.locals, local{name: name, mut: false, isConst: true})
	}
	sub.compileBlock(d.Body, true) // function body: last expr may return
	// If block didn't end with expression-return, return unit
	if !sub.blockReturned {
		sub.emit(OpUnit, d.Body.Pos())
		if sub.returnsResult {
			sub.emit(OpWrapResult, d.Body.Pos())
		}
		sub.emit(OpReturn, d.Body.Pos())
	}
	sub.chunk.NumLocs = len(sub.locals)
	// Debug: record local variable names for debugger inspection
	sub.chunk.LocalNames = make([]string, len(sub.locals))
	for i, l := range sub.locals {
		sub.chunk.LocalNames[i] = l.name
	}

	fn := &runtime.FuncObj{
		Name:  d.Name,
		Arity: len(d.Params),
		Chunk: sub.chunk,
		TypeInfo: &runtime.TypeInfo{
			Name: d.Name,
			Kind: "fn",
		},
	}
	for _, p := range d.Params {
		fi := runtime.FieldInfo{Name: p.Name}
		if p.Type != nil {
			fi.Type = typeExprInfo(p.Type)
		}
		fn.TypeInfo.Fields = append(fn.TypeInfo.Fields, fi)
	}
	c.anonID = sub.anonID
	return fn, sub.errs
}

func isResultType(t ast.TypeExpr) bool {
	_, ok := t.(*ast.ResultType)
	return ok
}

func typeExprInfo(t ast.TypeExpr) *runtime.TypeInfo {
	if t == nil {
		return runtime.TypeAny
	}
	switch t := t.(type) {
	case *ast.NamedType:
		switch t.Name {
		case "str":
			return runtime.TypeStr
		case "int":
			return runtime.TypeInt
		case "float":
			return runtime.TypeFloat
		case "bool":
			return runtime.TypeBool
		case "unit":
			return runtime.TypeUnit
		case "any":
			return runtime.TypeAny
		default:
			return &runtime.TypeInfo{Name: t.Name, Kind: "named"}
		}
	case *ast.ListType:
		return &runtime.TypeInfo{Name: "list", Kind: "list", Elem: typeExprInfo(t.Element)}
	case *ast.ResultType:
		return &runtime.TypeInfo{Name: "Result", Kind: "result", Elem: typeExprInfo(t.Ok)}
	case *ast.OptionalType:
		return &runtime.TypeInfo{Name: "optional", Kind: "optional", Elem: typeExprInfo(t.Element)}
	case *ast.MapType:
		return &runtime.TypeInfo{Name: "Map", Kind: "map", Key: typeExprInfo(t.Key), Elem: typeExprInfo(t.Value)}
	default:
		return runtime.TypeAny
	}
}

func (c *Compiler) typeInfoFromDecl(d *ast.TypeDecl) *runtime.TypeInfo {
	if d.Alias != nil {
		return &runtime.TypeInfo{Name: d.Name, Kind: "alias", Alias: typeExprInfo(d.Alias)}
	}
	ti := &runtime.TypeInfo{Name: d.Name, Kind: "struct"}
	for _, f := range d.Fields {
		fi := runtime.FieldInfo{Name: f.Name, Type: typeExprInfo(f.Type)}
		if f.Default != nil {
			if lit, ok := f.Default.(*ast.BasicLit); ok {
				v, err := litValue(lit)
				if err == nil {
					fi.Default = &v
				}
			}
		}
		ti.Fields = append(ti.Fields, fi)
	}
	return ti
}

func (c *Compiler) emit(op Op, pos token.Pos) {
	c.chunk.Code = append(c.chunk.Code, byte(op))
	c.chunk.Lines = append(c.chunk.Lines, pos.Line)
}

func (c *Compiler) emitArg(op Op, arg uint16, pos token.Pos) {
	c.emit(op, pos)
	c.chunk.Code = append(c.chunk.Code, byte(arg>>8), byte(arg))
	c.chunk.Lines = append(c.chunk.Lines, pos.Line, pos.Line)
}

func (c *Compiler) emitJump(op Op, pos token.Pos) int {
	c.emitArg(op, 0, pos)
	return len(c.chunk.Code) - 2
}

func (c *Compiler) patchJump(at int) {
	jump := len(c.chunk.Code) - (at + 2)
	if jump > 65535 {
		c.errorf(token.Pos{}, "jump too large")
	}
	c.chunk.Code[at] = byte(jump >> 8)
	c.chunk.Code[at+1] = byte(jump)
}

func (c *Compiler) emitLoop(loopStart int, pos token.Pos) {
	c.emit(OpJump, pos)
	rel := loopStart - (len(c.chunk.Code) + 2)
	u := uint16(int16(rel))
	c.chunk.Code = append(c.chunk.Code, byte(u>>8), byte(u))
	c.chunk.Lines = append(c.chunk.Lines, pos.Line, pos.Line)
}

func (c *Compiler) addConst(v runtime.Value) uint16 {
	idx := len(c.chunk.Consts)
	if idx >= 1<<16 {
		c.errorf(token.Pos{}, "too many constants (max 65535)")
		return 0
	}
	c.chunk.Consts = append(c.chunk.Consts, v)
	return uint16(idx)
}

func (c *Compiler) localSlot(name string) (int, local, bool) {
	for i := len(c.locals) - 1; i >= 0; i-- {
		if c.locals[i].name == name {
			return i, c.locals[i], true
		}
	}
	return -1, local{}, false
}

func (c *Compiler) declareLocal(name string, mut, isConst bool) int {
	c.locals = append(c.locals, local{name: name, mut: mut, isConst: isConst})
	return len(c.locals) - 1
}

func (c *Compiler) compileBlock(b *ast.Block, fnBody bool) {
	if b == nil {
		return
	}
	for i, s := range b.Stmts {
		// Non-verbose: last expression of a *function body* is the return value
		// (fn(x) { x + 1 }). Also: last `if` is an expression (branches yield values).
		if fnBody && i == len(b.Stmts)-1 {
			if es, ok := s.(*ast.ExprStmt); ok {
				c.compileExpr(es.X)
				if c.returnsResult {
					c.emit(OpWrapResult, es.Pos())
				}
				c.emit(OpReturn, es.Pos())
				c.blockReturned = true
				return
			}
			if is, ok := s.(*ast.IfStmt); ok {
				c.compileIfExpr(is)
				if c.returnsResult {
					c.emit(OpWrapResult, is.Pos())
				}
				c.emit(OpReturn, is.Pos())
				c.blockReturned = true
				return
			}
		}
		c.compileStmt(s)
	}
}

// compileBlockValue evaluates a block for its value: last expr/if leaves a value;
// empty or statement-only ends with unit.
func (c *Compiler) compileBlockValue(b *ast.Block) {
	if b == nil || len(b.Stmts) == 0 {
		c.emit(OpUnit, token.Pos{})
		return
	}
	for i, s := range b.Stmts {
		last := i == len(b.Stmts)-1
		if last {
			switch s := s.(type) {
			case *ast.ExprStmt:
				c.compileExpr(s.X)
				return
			case *ast.IfStmt:
				c.compileIfExpr(s)
				return
			case *ast.ReturnStmt:
				// return inside branch: still compile as return (exits fn)
				c.compileStmt(s)
				return
			}
		}
		c.compileStmt(s)
	}
	// last was a non-value statement
	c.emit(OpUnit, b.Pos())
}

// compileIfExpr leaves a value from then/else branches (else defaults to unit).
func (c *Compiler) compileIfExpr(s *ast.IfStmt) {
	c.compileIfBranches(s.Cond, s.Then, s.Else, s.Pos())
}

// compileIfExprNode compiles if-as-value expressions (RHS of :=, call args, …).
func (c *Compiler) compileIfExprNode(e *ast.IfExpr) {
	c.compileIfBranches(e.Cond, e.Then, e.Else, e.Pos())
}

// compileMatchExpr: first matching arm's block value wins; no match → unit.
func (c *Compiler) compileMatchExpr(e *ast.MatchExpr) {
	pos := e.Pos()
	if len(e.Arms) == 0 {
		c.errorf(pos, "match has no arms")
		c.emit(OpUnit, pos)
		return
	}
	c.compileExpr(e.Scrutinee)
	tmp := fmt.Sprintf("__m%d", c.anonID)
	c.anonID++
	slot := c.declareLocal(tmp, false, false)
	c.emitArg(OpStoreLocal, uint16(slot), pos)

	var endJumps []int
	for i, arm := range e.Arms {
		isLast := i == len(e.Arms)-1
		if arm.IsWildcard {
			c.compileBlockValue(arm.Body)
			if !isLast {
				endJumps = append(endJumps, c.emitJump(OpJump, arm.Pos_))
			}
			break
		}
		if arm.Pattern == nil {
			c.errorf(arm.Pos_, "invalid match pattern")
			continue
		}
		if len(arm.Bindings) > 0 {
			// Payload destructuring: compare _tag, then extract fields
			// Load scrutinee._tag
			c.emitArg(OpLoadLocal, uint16(slot), arm.Pos_)
			idx := c.addConst(runtime.Str("_tag"))
			c.emitArg(OpLoadConst, idx, arm.Pos_)
			c.emit(OpGetField, arm.Pos_)
			// Compare with expected tag string (e.g. "Shape.Circle")
			tagStr := patternToTagString(arm.Pattern)
			tagIdx := c.addConst(runtime.Str(tagStr))
			c.emitArg(OpLoadConst, tagIdx, arm.Pos_)
			c.emit(OpEq, arm.Pos_)
			failJ := c.emitJump(OpJumpIfFalsePop, arm.Pos_)
			// Bind each payload field to a local
			for _, bname := range arm.Bindings {
				c.emitArg(OpLoadLocal, uint16(slot), arm.Pos_)
				bidx := c.addConst(runtime.Str(bname))
				c.emitArg(OpLoadConst, bidx, arm.Pos_)
				c.emit(OpGetField, arm.Pos_)
				bslot := c.declareLocal(bname, false, false)
				c.emitArg(OpStoreLocal, uint16(bslot), arm.Pos_)
			}
			c.compileBlockValue(arm.Body)
			endJumps = append(endJumps, c.emitJump(OpJump, arm.Pos_))
			c.patchJump(failJ)
		} else {
			// Simple equality match (existing behavior)
			c.emitArg(OpLoadLocal, uint16(slot), arm.Pos_)
			c.compileExpr(arm.Pattern)
			c.emit(OpEq, arm.Pos_)
			failJ := c.emitJump(OpJumpIfFalsePop, arm.Pos_)
			c.compileBlockValue(arm.Body)
			endJumps = append(endJumps, c.emitJump(OpJump, arm.Pos_))
			c.patchJump(failJ)
		}
	}
	// Fall-through when no pattern matched and no trailing wildcard.
	if !e.Arms[len(e.Arms)-1].IsWildcard {
		c.emit(OpUnit, pos)
	}
	for _, j := range endJumps {
		c.patchJump(j)
	}
}

// enumHasPayloads reports whether any variant in the enum carries fields.
func enumHasPayloads(d *ast.EnumDecl) bool {
	for _, pv := range d.Payloads {
		if len(pv.Fields) > 0 {
			return true
		}
	}
	return false
}

// patternToTagString converts a match pattern AST (Ident or FieldExpr chain) to a dot-joined string.
// Shape.Circle → "Shape.Circle"
func patternToTagString(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.FieldExpr:
		return patternToTagString(e.X) + "." + e.Name
	default:
		return ""
	}
}

func (c *Compiler) compileIfBranches(cond ast.Expr, then *ast.Block, els any, pos token.Pos) {
	c.compileExpr(cond)
	elseJ := c.emitJump(OpJumpIfFalsePop, pos)
	c.compileBlockValue(then)
	endJ := c.emitJump(OpJump, pos)
	c.patchJump(elseJ)
	if els != nil {
		switch e := els.(type) {
		case *ast.Block:
			c.compileBlockValue(e)
		case *ast.IfStmt:
			c.compileIfExpr(e)
		case *ast.IfExpr:
			c.compileIfExprNode(e)
		default:
			c.emit(OpUnit, pos)
		}
	} else {
		c.emit(OpUnit, pos)
	}
	c.patchJump(endJ)
}

func (c *Compiler) compileStmt(s ast.Stmt) {
	switch s := s.(type) {
	case *ast.LetStmt:
		c.compileExpr(s.Init)
		slot := c.declareLocal(s.Name, s.Mut, false)
		c.emitArg(OpStoreLocal, uint16(slot), s.Pos())
		if c.replMode {
			// persist: reload local and store global
			c.emitArg(OpLoadLocal, uint16(slot), s.Pos())
			idx := c.addConst(runtime.Str(s.Name))
			c.emitArg(OpLoadConst, idx, s.Pos())
			c.emit(OpStoreGlobal, s.Pos())
		}
	case *ast.ConstDecl:
		c.compileExpr(s.Value)
		slot := c.declareLocal(s.Name, false, true)
		c.emitArg(OpStoreLocal, uint16(slot), s.Pos())
	case *ast.ReturnStmt:
		if s.Value != nil {
			c.compileExpr(s.Value)
		} else {
			c.emit(OpUnit, s.Pos())
		}
		if c.returnsResult {
			c.emit(OpWrapResult, s.Pos())
		}
		c.emit(OpReturn, s.Pos())
	case *ast.ExprStmt:
		c.compileExpr(s.X)
		c.emit(OpPop, s.Pos())
	case *ast.AssignStmt:
		c.compileAssign(s)
	case *ast.IfStmt:
		c.compileIf(s)
	case *ast.WhileStmt:
		c.compileWhile(s)
	case *ast.ForStmt:
		c.compileFor(s)
	case *ast.BreakStmt:
		if len(c.loopBreaks) == 0 {
			c.errorf(s.Pos(), "break outside loop")
			return
		}
		j := c.emitJump(OpJump, s.Pos())
		last := len(c.loopBreaks) - 1
		c.loopBreaks[last] = append(c.loopBreaks[last], j)
	case *ast.ContinueStmt:
		if len(c.loopContinues) == 0 {
			c.errorf(s.Pos(), "continue outside loop")
			return
		}
		j := c.emitJump(OpJump, s.Pos())
		last := len(c.loopContinues) - 1
		c.loopContinues[last] = append(c.loopContinues[last], j)
	case *ast.Block:
		c.compileBlock(s, false)
	case *ast.DeferStmt:
		// defer f(args) — args evaluated now; call runs LIFO at function exit (return / ? / fallthrough).
		call, ok := s.Call.(*ast.CallExpr)
		if !ok {
			c.errorf(s.Pos(), "defer requires a call expression (e.g. defer close(ch))")
			return
		}
		c.compileExpr(call.Fun)
		for _, a := range call.Args {
			c.compileExpr(a)
		}
		c.emitArg(OpDefer, uint16(len(call.Args)), s.Pos())
	default:
		c.errorf(s.Pos(), "unsupported statement %T", s)
	}
}

func (c *Compiler) compileAssign(s *ast.AssignStmt) {
	switch t := s.Target.(type) {
	case *ast.Ident:
		if slot, loc, ok := c.localSlot(t.Name); ok {
			if loc.isConst {
				c.errorf(s.Pos(), "cannot reassign const %q", t.Name)
				return
			}
			if !loc.mut {
				c.errorf(s.Pos(), "cannot reassign immutable binding %q (use `let mut`)", t.Name)
				return
			}
			c.compileExpr(s.Value)
			c.emitArg(OpStoreLocal, uint16(slot), s.Pos())
			return
		}
		// globals: functions etc. — reassignment of globals allowed for now
		c.compileExpr(s.Value)
		idx := c.addConst(runtime.Str(t.Name))
		c.emitArg(OpLoadConst, idx, s.Pos())
		c.emit(OpStoreGlobal, s.Pos())
	case *ast.FieldExpr:
		// in-place field mutation allowed through immutable binding
		if id, ok := t.X.(*ast.Ident); ok {
			if _, loc, ok := c.localSlot(id.Name); ok && loc.isConst {
				c.errorf(s.Pos(), "cannot mutate const %q", id.Name)
				return
			}
		}
		c.compileExpr(t.X)
		c.compileExpr(s.Value)
		idx := c.addConst(runtime.Str(t.Name))
		c.emitArg(OpLoadConst, idx, s.Pos())
		c.emit(OpSetField, s.Pos())
	case *ast.IndexExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			if _, loc, ok := c.localSlot(id.Name); ok && loc.isConst {
				c.errorf(s.Pos(), "cannot mutate const %q", id.Name)
				return
			}
		}
		c.compileExpr(t.X)
		c.compileExpr(t.Index)
		c.compileExpr(s.Value)
		c.emit(OpSetIndex, s.Pos())
	default:
		c.errorf(s.Pos(), "invalid assignment target")
	}
}

func (c *Compiler) compileIf(s *ast.IfStmt) {
	c.compileExpr(s.Cond)
	// Pop condition; jump if falsey
	elseJ := c.emitJump(OpJumpIfFalsePop, s.Pos())
	c.compileBlock(s.Then, false)
	if s.Else != nil {
		endJ := c.emitJump(OpJump, s.Pos())
		c.patchJump(elseJ)
		switch e := s.Else.(type) {
		case *ast.Block:
			c.compileBlock(e, false)
		case *ast.IfStmt:
			c.compileIf(e)
		}
		c.patchJump(endJ)
	} else {
		c.patchJump(elseJ)
	}
}

func (c *Compiler) compileWhile(s *ast.WhileStmt) {
	c.loopBreaks = append(c.loopBreaks, nil)
	c.loopContinues = append(c.loopContinues, nil)
	start := len(c.chunk.Code)
	c.compileExpr(s.Cond)
	exitJ := c.emitJump(OpJumpIfFalsePop, s.Pos())
	c.compileBlock(s.Body, false)
	contTarget := len(c.chunk.Code)
	for _, j := range c.loopContinues[len(c.loopContinues)-1] {
		rel := contTarget - (j + 2)
		c.chunk.Code[j] = byte(rel >> 8)
		c.chunk.Code[j+1] = byte(rel)
	}
	c.emitLoop(start, s.Pos())
	c.patchJump(exitJ)
	end := len(c.chunk.Code)
	for _, j := range c.loopBreaks[len(c.loopBreaks)-1] {
		rel := end - (j + 2)
		c.chunk.Code[j] = byte(rel >> 8)
		c.chunk.Code[j+1] = byte(rel)
	}
	c.loopBreaks = c.loopBreaks[:len(c.loopBreaks)-1]
	c.loopContinues = c.loopContinues[:len(c.loopContinues)-1]
}

func (c *Compiler) compileFor(s *ast.ForStmt) {
	c.loopBreaks = append(c.loopBreaks, nil)
	c.loopContinues = append(c.loopContinues, nil)

	c.compileExpr(s.Iter)
	c.emit(OpIterNew, s.Pos())
	itSlot := c.declareLocal("__it", false, false)
	c.emitArg(OpStoreLocal, uint16(itSlot), s.Pos())

	start := len(c.chunk.Code)
	c.emitArg(OpLoadLocal, uint16(itSlot), s.Pos())
	c.emit(OpIterNext, s.Pos())
	exitJ := c.emitJump(OpJumpIfFalsePop, s.Pos()) // pops false or true
	// if true was popped, value remains for store; if false, we jumped with nothing left
	// Wait: IterNext pushes [val, true] or [false].
	// JumpIfFalsePop: pops top. If true was top, pop true, leave val, don't jump. Good.
	// If false was top, pop false, jump. Good — no val.
	nameSlot := c.declareLocal(s.Name, false, false)
	c.emitArg(OpStoreLocal, uint16(nameSlot), s.Pos())
	c.compileBlock(s.Body, false)
	contTarget := len(c.chunk.Code)
	for _, j := range c.loopContinues[len(c.loopContinues)-1] {
		rel := contTarget - (j + 2)
		c.chunk.Code[j] = byte(rel >> 8)
		c.chunk.Code[j+1] = byte(rel)
	}
	c.emitLoop(start, s.Pos())
	c.patchJump(exitJ)
	end := len(c.chunk.Code)
	for _, j := range c.loopBreaks[len(c.loopBreaks)-1] {
		rel := end - (j + 2)
		c.chunk.Code[j] = byte(rel >> 8)
		c.chunk.Code[j+1] = byte(rel)
	}
	c.loopBreaks = c.loopBreaks[:len(c.loopBreaks)-1]
	c.loopContinues = c.loopContinues[:len(c.loopContinues)-1]
}

func (c *Compiler) compileExpr(e ast.Expr) {
	if e == nil {
		c.emit(OpNull, token.Pos{})
		return
	}
	switch e := e.(type) {
	case *ast.BasicLit:
		v, err := litValue(e)
		if err != nil {
			c.errorf(e.Pos(), "%v", err)
			c.emit(OpNull, e.Pos())
			return
		}
		switch e.Kind {
		case token.True:
			c.emit(OpTrue, e.Pos())
		case token.False:
			c.emit(OpFalse, e.Pos())
		case token.Null:
			c.emit(OpNull, e.Pos())
		default:
			c.emitArg(OpLoadConst, c.addConst(v), e.Pos())
		}
	case *ast.Ident:
		if e.Name == "unit" {
			c.emit(OpUnit, e.Pos())
			return
		}
		if slot, _, ok := c.localSlot(e.Name); ok {
			c.emitArg(OpLoadLocal, uint16(slot), e.Pos())
			return
		}
		// REPL / globals
		idx := c.addConst(runtime.Str(e.Name))
		c.emitArg(OpLoadConst, idx, e.Pos())
		c.emit(OpLoadGlobal, e.Pos())
	case *ast.BinaryExpr:
		c.compileBinary(e)
	case *ast.UnaryExpr:
		c.compileExpr(e.X)
		switch e.Op {
		case token.Minus:
			c.emit(OpNeg, e.Pos())
		case token.Bang:
			c.emit(OpNot, e.Pos())
		}
	case *ast.CallExpr:
		c.compileExpr(e.Fun)
		for _, a := range e.Args {
			c.compileExpr(a)
		}
		c.emitArg(OpCall, uint16(len(e.Args)), e.Pos())
	case *ast.IndexExpr:
		c.compileExpr(e.X)
		c.compileExpr(e.Index)
		c.emit(OpGetIndex, e.Pos())
	case *ast.FieldExpr:
		c.compileExpr(e.X)
		idx := c.addConst(runtime.Str(e.Name))
		c.emitArg(OpLoadConst, idx, e.Pos())
		c.emit(OpGetField, e.Pos())
	case *ast.QuestionExpr:
		c.compileExpr(e.X)
		c.emit(OpTryQ, e.Pos())
	case *ast.ListLit:
		for _, el := range e.Elts {
			c.compileExpr(el)
		}
		c.emitArg(OpMakeList, uint16(len(e.Elts)), e.Pos())
	case *ast.MapLit:
		for i := range e.Keys {
			c.compileExpr(e.Keys[i])
			c.compileExpr(e.Vals[i])
		}
		c.emitArg(OpMakeMap, uint16(len(e.Keys)), e.Pos())
	case *ast.StructLit:
		for _, f := range e.Fields {
			idx := c.addConst(runtime.Str(f.Name))
			c.emitArg(OpLoadConst, idx, e.Pos())
			c.compileExpr(f.Value)
		}
		nameIdx := c.addConst(runtime.Str(e.Name))
		c.emitArg(OpLoadConst, nameIdx, e.Pos())
		c.emitArg(OpMakeStruct, uint16(len(e.Fields)), e.Pos())
	case *ast.FStringExpr:
		if len(e.Parts) == 0 {
			c.emitArg(OpLoadConst, c.addConst(runtime.Str("")), e.Pos())
			return
		}
		first := true
		for _, part := range e.Parts {
			if part.Expr != nil {
				c.compileExpr(part.Expr)
			} else {
				c.emitArg(OpLoadConst, c.addConst(runtime.Str(part.Text)), e.Pos())
			}
			if !first {
				c.emit(OpConcat, e.Pos())
			}
			first = false
		}
	case *ast.IfExpr:
		c.compileIfExprNode(e)
	case *ast.MatchExpr:
		c.compileMatchExpr(e)
	case *ast.FuncLit:
		// Anonymous fn with by-value capture of outer locals (deep-copied at creation).
		c.anonID++
		name := fmt.Sprintf("__anon_%d", c.anonID)
		free := findFreeVars(e.Body, e.Params, c)
		fn, errs := c.compileFnWithFree(&ast.FnDecl{
			Pos_:   e.Pos(),
			Name:   name,
			Params: e.Params,
			Ret:    e.Ret,
			Body:   e.Body,
		}, free)
		c.errs = append(c.errs, errs...)
		// Prototype in const; OpClose attaches upvalues (do not share one FuncObj across closures).
		idx := c.addConst(runtime.Func(fn))
		c.emitArg(OpLoadConst, idx, e.Pos())
		for _, name := range free {
			if slot, _, ok := c.localSlot(name); ok {
				c.emitArg(OpLoadLocal, uint16(slot), e.Pos())
			} else {
				// free analysis missed; load global as fallback
				c.emitArg(OpLoadConst, c.addConst(runtime.Str(name)), e.Pos())
				c.emit(OpLoadGlobal, e.Pos())
			}
		}
		c.emitArg(OpClose, uint16(len(free)), e.Pos())
	default:
		c.errorf(e.Pos(), "unsupported expression %T", e)
		c.emit(OpNull, e.Pos())
	}
}

func (c *Compiler) compileBinary(e *ast.BinaryExpr) {
	switch e.Op {
	case token.And:
		c.compileAnd(e)
		return
	case token.Or:
		c.compileOr(e)
		return
	case token.NullCoalesce:
		c.compileExpr(e.X)
		c.compileExpr(e.Y)
		c.emit(OpNullCoalesce, e.Pos())
		return
	}

	c.compileExpr(e.X)
	c.compileExpr(e.Y)
	switch e.Op {
	case token.Plus:
		c.emit(OpAdd, e.Pos())
	case token.Minus:
		c.emit(OpSub, e.Pos())
	case token.Star:
		c.emit(OpMul, e.Pos())
	case token.Slash:
		c.emit(OpDiv, e.Pos())
	case token.Percent:
		c.emit(OpMod, e.Pos())
	case token.Eq:
		c.emit(OpEq, e.Pos())
	case token.Neq:
		c.emit(OpNeq, e.Pos())
	case token.Lt:
		c.emit(OpLt, e.Pos())
	case token.Lte:
		c.emit(OpLte, e.Pos())
	case token.Gt:
		c.emit(OpGt, e.Pos())
	case token.Gte:
		c.emit(OpGte, e.Pos())
	default:
		c.errorf(e.Pos(), "unsupported operator %s", e.Op)
	}
}

func (c *Compiler) compileAnd(e *ast.BinaryExpr) {
	// keep-value short-circuit
	c.compileExpr(e.X)
	elseJ := c.emitJump(OpJumpIfFalse, e.Pos())
	c.emit(OpPop, e.Pos())
	c.compileExpr(e.Y)
	c.patchJump(elseJ)
}

func (c *Compiler) compileOr(e *ast.BinaryExpr) {
	c.compileExpr(e.X)
	elseJ := c.emitJump(OpJumpIfTrue, e.Pos())
	c.emit(OpPop, e.Pos())
	c.compileExpr(e.Y)
	c.patchJump(elseJ)
}

func litValue(e *ast.BasicLit) (runtime.Value, error) {
	switch e.Kind {
	case token.Int:
		// base 0: decimal, 0x hex, 0b binary, 0o octal; strip digit separators
		n, err := strconv.ParseInt(stripNumSep(e.Value), 0, 64)
		return runtime.Int(n), err
	case token.Float:
		f, err := strconv.ParseFloat(stripNumSep(e.Value), 64)
		return runtime.Float(f), err
	case token.String, token.RawString:
		return runtime.Str(e.Value), nil
	case token.True:
		return runtime.Bool(true), nil
	case token.False:
		return runtime.Bool(false), nil
	case token.Null:
		return runtime.Null(), nil
	default:
		return runtime.Null(), fmt.Errorf("bad literal kind")
	}
}

func stripNumSep(s string) string {
	if !strings.Contains(s, "_") {
		return s
	}
	return strings.ReplaceAll(s, "_", "")
}

// findFreeVars returns outer local names referenced by body but not defined inside the fn.
func findFreeVars(body *ast.Block, params []*ast.Param, outer *Compiler) []string {
	if body == nil || outer == nil {
		return nil
	}
	declared := map[string]bool{}
	for _, p := range params {
		declared[p.Name] = true
	}
	var walkStmt func(ast.Stmt)
	var walkExpr func(ast.Expr)
	var free []string
	seen := map[string]bool{}

	addFree := func(name string) {
		if name == "" || declared[name] || seen[name] {
			return
		}
		if _, _, ok := outer.localSlot(name); ok {
			seen[name] = true
			free = append(free, name)
		}
	}

	walkExpr = func(e ast.Expr) {
		if e == nil {
			return
		}
		switch e := e.(type) {
		case *ast.Ident:
			addFree(e.Name)
		case *ast.BinaryExpr:
			walkExpr(e.X)
			walkExpr(e.Y)
		case *ast.UnaryExpr:
			walkExpr(e.X)
		case *ast.CallExpr:
			walkExpr(e.Fun)
			for _, a := range e.Args {
				walkExpr(a)
			}
		case *ast.IndexExpr:
			walkExpr(e.X)
			walkExpr(e.Index)
		case *ast.FieldExpr:
			walkExpr(e.X)
		case *ast.QuestionExpr:
			walkExpr(e.X)
		case *ast.ListLit:
			for _, el := range e.Elts {
				walkExpr(el)
			}
		case *ast.MapLit:
			for i := range e.Keys {
				walkExpr(e.Keys[i])
				walkExpr(e.Vals[i])
			}
		case *ast.StructLit:
			for _, f := range e.Fields {
				walkExpr(f.Value)
			}
		case *ast.FStringExpr:
			for _, part := range e.Parts {
				if part.Expr != nil {
					walkExpr(part.Expr)
				}
			}
		case *ast.IfExpr:
			walkExpr(e.Cond)
			if e.Then != nil {
				for _, s := range e.Then.Stmts {
					walkStmt(s)
				}
			}
			switch el := e.Else.(type) {
			case *ast.Block:
				for _, s := range el.Stmts {
					walkStmt(s)
				}
			case *ast.IfExpr:
				walkExpr(el)
			}
		case *ast.MatchExpr:
			walkExpr(e.Scrutinee)
			for _, arm := range e.Arms {
				if arm.Pattern != nil {
					walkExpr(arm.Pattern)
				}
				if arm.Body != nil {
					for _, s := range arm.Body.Stmts {
						walkStmt(s)
					}
				}
			}
		case *ast.FuncLit:
			// Nested lit: only scan for free that resolve in *this* outer (c).
			// Nested will capture from its own outer when compiled.
			innerDecl := map[string]bool{}
			for k, v := range declared {
				innerDecl[k] = v
			}
			for _, p := range e.Params {
				innerDecl[p.Name] = true
			}
			// Walk nested body with temporary declared set — free of nested
			// that are free of outer still need outer to list them if nested
			// only references them... Actually nested compile handles its free.
			// Parent doesn't need nested's free unless parent body also uses them.
			// Just walk nested body Idents that aren't nested params/lets:
			// Use recursive findFreeVars(e.Body, e.Params, outer) and merge.
			for _, name := range findFreeVars(e.Body, e.Params, outer) {
				addFree(name)
			}
		}
	}

	walkStmt = func(s ast.Stmt) {
		if s == nil {
			return
		}
		switch s := s.(type) {
		case *ast.LetStmt:
			walkExpr(s.Init)
			declared[s.Name] = true
		case *ast.ConstDecl:
			walkExpr(s.Value)
			declared[s.Name] = true
		case *ast.AssignStmt:
			walkExpr(s.Value)
			// target may be ident — assignment doesn't declare
			walkExpr(s.Target)
		case *ast.ReturnStmt:
			walkExpr(s.Value)
		case *ast.ExprStmt:
			walkExpr(s.X)
		case *ast.IfStmt:
			walkExpr(s.Cond)
			if s.Then != nil {
				for _, st := range s.Then.Stmts {
					walkStmt(st)
				}
			}
			switch e := s.Else.(type) {
			case *ast.Block:
				for _, st := range e.Stmts {
					walkStmt(st)
				}
			case *ast.IfStmt:
				walkStmt(e)
			}
		case *ast.WhileStmt:
			walkExpr(s.Cond)
			if s.Body != nil {
				for _, st := range s.Body.Stmts {
					walkStmt(st)
				}
			}
		case *ast.ForStmt:
			walkExpr(s.Iter)
			// for-var is local to loop body
			prev := declared[s.Name]
			declared[s.Name] = true
			if s.Body != nil {
				for _, st := range s.Body.Stmts {
					walkStmt(st)
				}
			}
			if !prev {
				delete(declared, s.Name)
			}
		case *ast.DeferStmt:
			walkExpr(s.Call)
		case *ast.Block:
			for _, st := range s.Stmts {
				walkStmt(st)
			}
		}
	}

	for _, s := range body.Stmts {
		walkStmt(s)
	}
	return free
}
