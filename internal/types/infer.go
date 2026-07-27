package types

import (
	"github.com/loreste/weft/internal/ast"
	"github.com/loreste/weft/internal/diag"
	"github.com/loreste/weft/internal/stdlib"
	"github.com/loreste/weft/internal/token"
)

// re-export token kinds used in BasicLit — need token import for e.Kind compare
var _ = token.Int

// Info holds inference results for a file.
type Info struct {
	// VarTypes: function.local or global.fn names -> type
	Globals map[string]*Type
	// FnInferred return types
	FnRet map[string]*Type
	// Binding annotations inferred for lets (debug)
	Bindings map[string]*Type
}

// Infer runs type inference + checking on a file.
func Infer(file *ast.File) (Info, diag.List) {
	inf := &inferrer{
		file:     file.Path,
		globals:  map[string]*Type{},
		fnRet:    map[string]*Type{},
		bindings: map[string]*Type{},
		userTypes: map[string]*Type{
			"str": tyStr(), "int": tyInt(), "float": tyFloat(), "bool": tyBool(),
			"unit": tyUnit(), "any": tyAny(), "Error": tyNamed("Error"),
		},
	}
	inf.installPrelude()

	// Collect decls
	for _, d := range file.Decls {
		switch d := d.(type) {
		case *ast.FnDecl:
			// provisional any; refined after body
			params := make([]*Type, len(d.Params))
			for i, p := range d.Params {
				if p.Type != nil {
					params[i] = FromAST(p.Type)
				} else {
					params[i] = tyAny()
				}
			}
			ret := tyAny()
			if d.Ret != nil {
				ret = FromAST(d.Ret)
			}
			inf.globals[d.Name] = tyFn(params, ret)
		case *ast.TypeDecl:
			inf.userTypes[d.Name] = tyNamed(d.Name)
			inf.globals[d.Name] = tyNamed(d.Name) // type as value / TypeInfo
		case *ast.EnumDecl:
			// enum Name { A, B } → global map of string tags
			inf.globals[d.Name] = tyMap(tyStr(), tyStr())
		case *ast.ConstDecl:
			// later
		case *ast.ImportDecl:
			name := importName(d)
			inf.globals[name] = tyNamed("package:" + name)
		}
	}

	// Infer each function body (may refine signatures)
	for _, d := range file.Decls {
		if fn, ok := d.(*ast.FnDecl); ok {
			inf.inferFn(fn)
		}
	}

	// Modules and *_test.weft have no main — only `weft run` requires main at compile time.

	return Info{
		Globals:  inf.globals,
		FnRet:    inf.fnRet,
		Bindings: inf.bindings,
	}, inf.errs
}

// Check runs name + type inference (public API used by weft check).
func Check(file *ast.File) diag.List {
	_, errs := Infer(file)
	return errs
}

type inferrer struct {
	file       string
	errs       diag.List
	globals    map[string]*Type
	fnRet      map[string]*Type
	bindings   map[string]*Type
	userTypes  map[string]*Type
	currentRet *Type // declared return of function being inferred (for `?` checks)
}

func (inf *inferrer) errorf(pos token.Pos, format string, args ...any) {
	inf.errs = append(inf.errs, diag.Errorf(inf.file, pos, format, args...))
}

func (inf *inferrer) installPrelude() {
	// builtins
	inf.globals["println"] = tyFn([]*Type{tyAny()}, tyUnit())
	inf.globals["print"] = tyFn([]*Type{tyAny()}, tyUnit())
	inf.globals["say"] = tyFn([]*Type{tyAny()}, tyUnit())
	inf.globals["len"] = tyFn([]*Type{tyAny()}, tyInt())
	inf.globals["push"] = tyFn([]*Type{tyList(tyAny()), tyAny()}, tyList(tyAny()))
	inf.globals["pop"] = tyFn([]*Type{tyList(tyAny())}, tyAny())
	inf.globals["concat"] = tyFn([]*Type{tyList(tyAny()), tyList(tyAny())}, tyList(tyAny()))
	inf.globals["slice"] = tyFn([]*Type{tyList(tyAny()), tyInt()}, tyList(tyAny()))
	inf.globals["range"] = tyFn([]*Type{tyInt()}, tyList(tyInt()))
	inf.globals["contains"] = tyFn([]*Type{tyAny(), tyAny()}, tyBool())
	inf.globals["keys"] = tyFn([]*Type{tyAny()}, tyList(tyStr()))
	inf.globals["values"] = tyFn([]*Type{tyAny()}, tyList(tyAny()))
	inf.globals["delete"] = tyFn([]*Type{tyAny(), tyAny()}, tyAny())
	inf.globals["map"] = tyFn([]*Type{tyList(tyAny()), tyAny()}, tyList(tyAny()))
	inf.globals["seq_map"] = tyFn([]*Type{tyList(tyAny()), tyAny()}, tyList(tyAny()))
	inf.globals["filter"] = tyFn([]*Type{tyList(tyAny()), tyAny()}, tyList(tyAny()))
	inf.globals["seq_filter"] = tyFn([]*Type{tyList(tyAny()), tyAny()}, tyList(tyAny()))
	inf.globals["reduce"] = tyFn([]*Type{tyList(tyAny()), tyAny(), tyAny()}, tyAny())
	inf.globals["each"] = tyFn([]*Type{tyList(tyAny()), tyAny()}, tyUnit())
	inf.globals["flat_map"] = tyFn([]*Type{tyList(tyAny()), tyAny()}, tyList(tyAny()))
	inf.globals["par_map"] = tyFn([]*Type{tyList(tyAny()), tyAny()}, tyList(tyAny()))
	inf.globals["find"] = tyFn([]*Type{tyList(tyAny()), tyAny()}, tyAny())
	inf.globals["any"] = tyFn([]*Type{tyList(tyAny()), tyAny()}, tyBool())
	inf.globals["all"] = tyFn([]*Type{tyList(tyAny()), tyAny()}, tyBool())
	inf.globals["Ok"] = tyFn([]*Type{tyAny()}, tyResult(tyAny()))
	inf.globals["Err"] = tyFn([]*Type{tyAny()}, tyResult(tyAny()))
	// error helpers (Result + ?)
	inf.globals["ensure"] = tyFn([]*Type{tyAny()}, tyResult(tyUnit()))
	inf.globals["bail"] = tyFn([]*Type{tyAny()}, tyResult(tyAny()))
	inf.globals["is_ok"] = tyFn([]*Type{tyAny()}, tyBool())
	inf.globals["is_err"] = tyFn([]*Type{tyAny()}, tyBool())
	inf.globals["unit"] = tyUnit()
	inf.globals["int"] = tyNamed("package:int")
	inf.globals["Error"] = tyNamed("package:Error")
	inf.globals["args"] = tyList(tyStr())
	inf.globals["os"] = tyNamed("package:os")

	// stdlib packages as opaque named packages (from registry — no hardcoded list)
	for _, p := range stdlib.Names() {
		inf.globals[p] = tyNamed("package:" + p)
	}
	// concurrency (default; not asyncio)
	inf.globals["spawn"] = tyFn([]*Type{tyAny()}, tyNamed("task"))
	inf.globals["parallel"] = tyFn([]*Type{tyList(tyAny())}, tyResult(tyList(tyAny())))
	inf.globals["gather"] = tyFn([]*Type{tyList(tyAny())}, tyResult(tyList(tyAny())))
	inf.globals["race"] = tyFn([]*Type{tyList(tyAny())}, tyResult(tyAny()))
	inf.globals["timeout"] = tyFn([]*Type{tyAny(), tyAny()}, tyResult(tyAny()))
	inf.globals["group"] = tyFn(nil, tyNamed("group"))
	inf.globals["channel"] = tyFn([]*Type{tyInt()}, tyChannel())
	inf.globals["send"] = tyFn([]*Type{tyChannel(), tyAny()}, tyResult(tyUnit()))
	inf.globals["recv"] = tyFn([]*Type{tyChannel()}, tyResult(tyAny()))
	inf.globals["close"] = tyFn([]*Type{tyChannel()}, tyResult(tyUnit()))
	inf.globals["select_recv"] = tyFn([]*Type{tyList(tyChannel())}, tyResult(tyNamed("select")))
}

func importName(d *ast.ImportDecl) string {
	if d.Alias != "" {
		return d.Alias
	}
	if !d.IsPath {
		return d.Path
	}
	name := d.Path
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '/' {
			name = name[i+1:]
			break
		}
	}
	if len(name) > 5 && name[len(name)-5:] == ".weft" {
		name = name[:len(name)-5]
	}
	return name
}

func (inf *inferrer) inferFn(d *ast.FnDecl) {
	locals := map[string]*Type{}
	for _, p := range d.Params {
		if p.Type != nil {
			locals[p.Name] = FromAST(p.Type)
		} else {
			locals[p.Name] = tyAny() // will refine from uses if we add constraint pass later
		}
	}
	declaredRet := (*Type)(nil)
	if d.Ret != nil {
		declaredRet = FromAST(d.Ret)
	}
	prevRet := inf.currentRet
	inf.currentRet = declaredRet
	defer func() { inf.currentRet = prevRet }()
	ret := inf.inferBlock(d.Body, locals, declaredRet)

	// parameter refinement left as any when unannotated (local inference is the win)

	// store refined function type
	params := make([]*Type, len(d.Params))
	for i, p := range d.Params {
		if t, ok := locals[p.Name]; ok {
			params[i] = t
		} else {
			params[i] = tyAny()
		}
		if p.Type != nil {
			params[i] = FromAST(p.Type)
		}
	}
	finalRet := ret
	if declaredRet != nil {
		finalRet = declaredRet
		if ret != nil && !Assignable(declaredRet, ret) {
			// allow Result wrap
			if !(declaredRet.Kind == TyResult && Assignable(declaredRet.Elem, ret)) {
				inf.errorf(d.Pos(), "function %q returns %s, declared %s", d.Name, fmtType(ret), fmtType(declaredRet))
			}
		}
	}
	inf.fnRet[d.Name] = finalRet
	inf.globals[d.Name] = tyFn(params, finalRet)
}

func (inf *inferrer) inferBlock(b *ast.Block, locals map[string]*Type, expectedRet *Type) *Type {
	if b == nil {
		return tyUnit()
	}
	var last *Type = tyUnit()
	for i, s := range b.Stmts {
		isLast := i == len(b.Stmts)-1
		last = inf.inferStmt(s, locals, expectedRet, isLast)
	}
	return last
}

func (inf *inferrer) inferStmt(s ast.Stmt, locals map[string]*Type, expectedRet *Type, isLast bool) *Type {
	switch s := s.(type) {
	case *ast.LetStmt:
		initT := inf.inferExpr(s.Init, locals)
		if s.Type != nil {
			ann := FromAST(s.Type)
			if !Assignable(ann, initT) {
				inf.errorf(s.Pos(), "cannot assign %s to %s (variable %q)", fmtType(initT), fmtType(ann), s.Name)
			}
			locals[s.Name] = ann
			inf.bindings[s.Name] = ann
		} else {
			// INFERENCE: bind from RHS
			if initT == nil {
				initT = tyAny()
			}
			locals[s.Name] = initT
			inf.bindings[s.Name] = initT
		}
		return tyUnit()
	case *ast.ConstDecl:
		t := inf.inferExpr(s.Value, locals)
		if s.Type != nil {
			ann := FromAST(s.Type)
			if !Assignable(ann, t) {
				inf.errorf(s.Pos(), "const %q: %s is not %s", s.Name, fmtType(t), fmtType(ann))
			}
			t = ann
		}
		locals[s.Name] = t
		return tyUnit()
	case *ast.AssignStmt:
		valT := inf.inferExpr(s.Value, locals)
		if id, ok := s.Target.(*ast.Ident); ok {
			if lt, ok := locals[id.Name]; ok {
				if !Assignable(lt, valT) {
					inf.errorf(s.Pos(), "cannot assign %s to %s (%q)", fmtType(valT), fmtType(lt), id.Name)
				}
			} else if _, ok := inf.globals[id.Name]; !ok {
				inf.errorf(id.Pos(), "undefined name %q", id.Name)
			}
		} else {
			inf.inferExpr(s.Target, locals)
		}
		return tyUnit()
	case *ast.ReturnStmt:
		var t *Type = tyUnit()
		if s.Value != nil {
			t = inf.inferExpr(s.Value, locals)
		}
		if expectedRet != nil && !Assignable(expectedRet, t) {
			if !(expectedRet.Kind == TyResult && Assignable(expectedRet.Elem, t)) {
				inf.errorf(s.Pos(), "return has type %s, expected %s", fmtType(t), fmtType(expectedRet))
			}
		}
		return t
	case *ast.ExprStmt:
		t := inf.inferExpr(s.X, locals)
		if isLast {
			return t // last expr is return value
		}
		return tyUnit()
	case *ast.IfStmt:
		ct := inf.inferExpr(s.Cond, locals)
		if ct != nil && ct.Kind != TyBool && ct.Kind != TyAny {
			// non-bool conditions are allowed (truthiness) — no error
		}
		thenT := inf.inferBlock(s.Then, copyTypes(locals), expectedRet)
		elseT := tyUnit()
		if s.Else != nil {
			switch e := s.Else.(type) {
			case *ast.Block:
				elseT = inf.inferBlock(e, copyTypes(locals), expectedRet)
			case *ast.IfStmt:
				elseT = inf.inferStmt(e, locals, expectedRet, true)
			}
		}
		if isLast {
			return Unify(thenT, elseT)
		}
		return tyUnit()
	case *ast.WhileStmt:
		inf.inferExpr(s.Cond, locals)
		inf.inferBlock(s.Body, copyTypes(locals), expectedRet)
		return tyUnit()
	case *ast.ForStmt:
		it := inf.inferExpr(s.Iter, locals)
		elem := tyAny()
		if it != nil && it.Kind == TyList && it.Elem != nil {
			elem = it.Elem
		}
		bodyLocals := copyTypes(locals)
		bodyLocals[s.Name] = elem
		inf.bindings[s.Name] = elem
		inf.inferBlock(s.Body, bodyLocals, expectedRet)
		return tyUnit()
	case *ast.Block:
		return inf.inferBlock(s, copyTypes(locals), expectedRet)
	case *ast.BreakStmt, *ast.ContinueStmt:
		return tyUnit()
	case *ast.DeferStmt:
		inf.inferExpr(s.Call, locals)
		return tyUnit()
	default:
		return tyUnit()
	}
}

func copyTypes(m map[string]*Type) map[string]*Type {
	out := make(map[string]*Type, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func (inf *inferrer) lookup(name string, locals map[string]*Type) (*Type, bool) {
	if t, ok := locals[name]; ok {
		return t, true
	}
	if t, ok := inf.globals[name]; ok {
		return t, true
	}
	if t, ok := inf.userTypes[name]; ok {
		return t, true
	}
	return nil, false
}

func (inf *inferrer) inferExpr(e ast.Expr, locals map[string]*Type) *Type {
	if e == nil {
		return tyAny()
	}
	switch e := e.(type) {
	case *ast.Ident:
		switch e.Name {
		case "true", "false":
			return tyBool()
		case "null":
			return tyNull()
		case "unit":
			return tyUnit()
		}
		if t, ok := inf.lookup(e.Name, locals); ok {
			return t
		}
		inf.errorf(e.Pos(), "undefined name %q", e.Name)
		return tyAny()
	case *ast.BasicLit:
		switch e.Kind {
		case token.Int:
			return tyInt()
		case token.Float:
			return tyFloat()
		case token.String, token.RawString, token.FString:
			return tyStr()
		case token.True, token.False:
			return tyBool()
		case token.Null:
			return tyNull()
		default:
			return tyAny()
		}
	case *ast.BinaryExpr:
		l := inf.inferExpr(e.X, locals)
		r := inf.inferExpr(e.Y, locals)
		return inf.inferBinary(e, l, r)
	case *ast.UnaryExpr:
		t := inf.inferExpr(e.X, locals)
		if e.Op == token.Bang {
			return tyBool()
		}
		return t
	case *ast.CallExpr:
		ft := inf.inferExpr(e.Fun, locals)
		argTs := make([]*Type, len(e.Args))
		for i, a := range e.Args {
			argTs[i] = inf.inferExpr(a, locals)
		}
		// special: Ok/Err
		if id, ok := e.Fun.(*ast.Ident); ok {
			switch id.Name {
			case "Ok":
				if len(argTs) > 0 {
					return tyResult(argTs[0])
				}
				return tyResult(tyAny())
			case "Err":
				return tyResult(tyAny())
			}
		}
		// package.method(...) — opaque any or Result[any] for known fallible
		if fe, ok := e.Fun.(*ast.FieldExpr); ok {
			return inf.inferMethod(fe, argTs)
		}
		if ft != nil && ft.Kind == TyFn {
			// arity soft-check
			if len(ft.Params) > 0 && len(argTs) != len(ft.Params) && ft.Params[0].Kind != TyAny {
				// only warn-ish when params were annotated; skip for any
			}
			for i := 0; i < len(argTs) && i < len(ft.Params); i++ {
				if !Assignable(ft.Params[i], argTs[i]) && ft.Params[i].Kind != TyAny {
					inf.errorf(e.Pos(), "argument %d: have %s, want %s", i+1, fmtType(argTs[i]), fmtType(ft.Params[i]))
				}
			}
			if ft.Ret != nil {
				return ft.Ret
			}
		}
		return tyAny()
	case *ast.IndexExpr:
		base := inf.inferExpr(e.X, locals)
		inf.inferExpr(e.Index, locals)
		if base != nil && base.Kind == TyList {
			return base.Elem
		}
		if base != nil && base.Kind == TyMap {
			return base.Elem
		}
		if base != nil && base.Kind == TyStr {
			return tyStr()
		}
		return tyAny()
	case *ast.FieldExpr:
		inf.inferExpr(e.X, locals)
		// package or map field → any
		return tyAny()
	case *ast.QuestionExpr:
		// `?` returns Err from the current function — only valid when return is Result
		// (or unannotated). Explicit non-Result returns are an error.
		if inf.currentRet != nil && inf.currentRet.Kind != TyResult {
			inf.errorf(e.Pos(), "`?` only allowed in functions returning Result (got %s)", fmtType(inf.currentRet))
		}
		t := inf.inferExpr(e.X, locals)
		if t != nil && t.Kind == TyResult {
			return t.Elem
		}
		// if not Result, still any (runtime checks)
		return tyAny()
	case *ast.ListLit:
		var elem *Type
		for _, el := range e.Elts {
			et := inf.inferExpr(el, locals)
			elem = Unify(elem, et)
		}
		if elem == nil {
			elem = tyAny()
		}
		return tyList(elem)
	case *ast.MapLit:
		var vt *Type
		for i := range e.Keys {
			inf.inferExpr(e.Keys[i], locals)
			vt = Unify(vt, inf.inferExpr(e.Vals[i], locals))
		}
		if vt == nil {
			vt = tyAny()
		}
		return tyMap(tyStr(), vt)
	case *ast.StructLit:
		for _, f := range e.Fields {
			inf.inferExpr(f.Value, locals)
		}
		if e.Name != "" {
			return tyNamed(e.Name)
		}
		return tyNamed("struct")
	case *ast.FStringExpr:
		for _, p := range e.Parts {
			if p.Expr != nil {
				inf.inferExpr(p.Expr, locals)
			}
		}
		return tyStr()
	case *ast.IfExpr:
		inf.inferExpr(e.Cond, locals)
		tThen := inf.inferBlock(e.Then, copyTypes(locals), nil)
		var tElse *Type = tyUnit()
		if e.Else != nil {
			switch el := e.Else.(type) {
			case *ast.Block:
				tElse = inf.inferBlock(el, copyTypes(locals), nil)
			case *ast.IfExpr:
				tElse = inf.inferExpr(el, locals)
			case *ast.IfStmt:
				tElse = inf.inferStmt(el, locals, nil, true)
			}
		}
		return Unify(tThen, tElse)
	case *ast.MatchExpr:
		inf.inferExpr(e.Scrutinee, locals)
		var t *Type = tyUnit()
		for i, arm := range e.Arms {
			if arm.Pattern != nil {
				inf.inferExpr(arm.Pattern, locals)
			}
			bt := inf.inferBlock(arm.Body, copyTypes(locals), nil)
			if i == 0 {
				t = bt
			} else {
				t = Unify(t, bt)
			}
		}
		return t
	case *ast.FuncLit:
		bodyLocals := copyTypes(locals)
		params := make([]*Type, len(e.Params))
		for i, p := range e.Params {
			var pt *Type = tyAny()
			if p.Type != nil {
				pt = FromAST(p.Type)
			}
			params[i] = pt
			bodyLocals[p.Name] = pt
		}
		var exp *Type
		if e.Ret != nil {
			exp = FromAST(e.Ret)
		}
		ret := inf.inferBlock(e.Body, bodyLocals, exp)
		if exp != nil {
			ret = exp
		}
		return tyFn(params, ret)
	default:
		return tyAny()
	}
}

func (inf *inferrer) inferBinary(e *ast.BinaryExpr, l, r *Type) *Type {
	switch e.Op {
	case token.Plus:
		if l != nil && l.Kind == TyStr || r != nil && r.Kind == TyStr {
			return tyStr()
		}
		return Unify(l, r)
	case token.Minus, token.Star, token.Slash, token.Percent:
		return Unify(l, r)
	case token.Eq, token.Neq, token.Lt, token.Lte, token.Gt, token.Gte:
		return tyBool()
	case token.And, token.Or:
		return tyBool()
	case token.NullCoalesce:
		// T? ?? T → T
		if l != nil && l.Kind == TyOptional {
			return Unify(l.Elem, r)
		}
		return Unify(l, r)
	default:
		return tyAny()
	}
}

func (inf *inferrer) inferMethod(fe *ast.FieldExpr, args []*Type) *Type {
	// Known fallible stdlib patterns return Result
	pkg, _ := fe.X.(*ast.Ident)
	if pkg == nil {
		return tyAny()
	}
	switch pkg.Name {
	case "json":
		switch fe.Name {
		case "parse":
			return tyResult(tyAny())
		case "stringify":
			return tyStr()
		}
	case "fs":
		switch fe.Name {
		case "read", "write", "append", "list", "mkdir", "remove", "remove_all",
			"abs", "glob", "lines", "copy", "cwd":
			return tyResult(tyAny())
		case "exists", "is_dir", "is_file":
			return tyBool()
		case "join", "base", "dir", "ext":
			return tyStr()
		}
	case "cli":
		switch fe.Name {
		case "parse":
			return tyResult(tyAny())
		case "args", "argv":
			return tyList(tyStr())
		case "prog", "usage", "flag":
			return tyAny()
		case "has":
			return tyBool()
		case "exit", "die", "ok":
			return tyAny()
		}
	case "sh":
		switch fe.Name {
		case "run", "capture", "ok", "shell":
			return tyResult(tyAny())
		case "which":
			return tyAny()
		}
	case "io":
		switch fe.Name {
		case "stdin", "lines":
			return tyResult(tyAny())
		case "is_tty":
			return tyBool()
		}
	case "str":
		switch fe.Name {
		case "split", "fields", "lines":
			return tyList(tyStr())
		case "join", "trim", "lower", "upper", "replace", "repeat":
			return tyStr()
		case "contains", "has_prefix", "has_suffix":
			return tyBool()
		}
	case "csv":
		switch fe.Name {
		case "parse", "read", "write":
			return tyResult(tyAny())
		case "stringify":
			return tyStr()
		}
	case "time":
		switch fe.Name {
		case "now", "now_ms", "since":
			return tyInt()
		case "iso", "format":
			return tyStr()
		case "sleep":
			return tyUnit()
		}
	case "db", "redis", "nats", "amqp", "mongo":
		switch fe.Name {
		case "open", "connect", "drivers":
			return tyResult(tyAny())
		}
	case "graphql":
		switch fe.Name {
		case "query", "mutation", "request":
			return tyResult(tyAny())
		}
	case "ollama", "vllm":
		switch fe.Name {
		case "chat", "list", "connect", "generate", "pull", "health", "ps":
			return tyResult(tyAny())
		case "host", "base", "openai_base":
			return tyStr()
		}
	case "http":
		switch fe.Name {
		case "get", "post", "fetch":
			return tyResult(tyNamed("Response"))
		case "text", "json":
			return tyAny()
		case "serve":
			return tyResult(tyUnit())
		}
	case "web":
		switch fe.Name {
		case "app":
			return tyNamed("web.App")
		case "text", "json", "html", "redirect", "status":
			return tyAny()
		}
	case "webrtc":
		switch fe.Name {
		case "hub":
			return tyNamed("webrtc.Hub")
		case "ice_servers":
			return tyList(tyAny())
		}
	case "viz":
		switch fe.Name {
		case "bar", "line", "area", "scatter", "pie", "hist", "table", "page", "html", "svg", "spark":
			return tyAny()
		case "save":
			return tyResult(tyStr())
		}
	case "env", "secrets":
		switch fe.Name {
		case "require":
			return tyResult(tyStr())
		case "get":
			return tyOpt(tyStr())
		}
	case "llm":
		switch fe.Name {
		case "chat", "ask", "extract", "stream":
			return tyResult(tyAny())
		case "tool", "agent", "client":
			return tyAny()
		}
	case "int":
		if fe.Name == "parse" {
			return tyResult(tyInt())
		}
	}
	return tyAny()
}
