package stdlib

import (
	"net/url"

	"github.com/loreste/weft/internal/runtime"
)

// packageURL — URL parse/build/query (Python urllib.parse lite).
func packageURL() runtime.Value {
	p := pkg()

	// url.parse(s) -> Result[map] {scheme,host,path,query,fragment,raw,user,port}
	set(p, "parse", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("url.parse(s)", "url"), nil
		}
		u, err := url.Parse(args[0].String())
		if err != nil {
			return errRes(err.Error(), "url"), nil
		}
		m := runtime.NewMap()
		mo := m.Obj.(*runtime.MapObj)
		put := func(k, v string) {
			mo.Keys = append(mo.Keys, k)
			mo.Vals[k] = runtime.Str(v)
		}
		put("scheme", u.Scheme)
		put("host", u.Hostname())
		put("port", u.Port())
		put("path", u.Path)
		put("query", u.RawQuery)
		put("fragment", u.Fragment)
		put("raw", u.String())
		if u.User != nil {
			put("user", u.User.Username())
			if pw, ok := u.User.Password(); ok {
				put("password", pw)
			}
		} else {
			put("user", "")
		}
		// query as map of first values
		qmap := runtime.NewMap()
		qmo := qmap.Obj.(*runtime.MapObj)
		for k, vs := range u.Query() {
			if len(vs) > 0 {
				qmo.Keys = append(qmo.Keys, k)
				qmo.Vals[k] = runtime.Str(vs[0])
			}
		}
		mo.Keys = append(mo.Keys, "params")
		mo.Vals["params"] = qmap
		return runtime.Ok(m), nil
	}, 1)

	// url.build({scheme,host,path,query|params,fragment,port,user}) -> str
	set(p, "build", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 || args[0].Kind != runtime.KindMap {
			return errRes("url.build({scheme,host,path,...})", "url"), nil
		}
		scheme := mapGetStr(args[0], "scheme", "https")
		host := mapGetStr(args[0], "host", "")
		path := mapGetStr(args[0], "path", "")
		fragment := mapGetStr(args[0], "fragment", "")
		port := mapGetStr(args[0], "port", "")
		user := mapGetStr(args[0], "user", "")
		pass := mapGetStr(args[0], "password", "")
		u := &url.URL{Scheme: scheme, Path: path, Fragment: fragment}
		if host != "" {
			if port != "" {
				u.Host = host + ":" + port
			} else {
				u.Host = host
			}
		}
		if user != "" {
			if pass != "" {
				u.User = url.UserPassword(user, pass)
			} else {
				u.User = url.User(user)
			}
		}
		// query string or params map
		if q := mapGetStr(args[0], "query", ""); q != "" {
			u.RawQuery = q
		} else if pm, ok := mapGet(args[0], "params"); ok && pm.Kind == runtime.KindMap {
			vals := url.Values{}
			mo := pm.Obj.(*runtime.MapObj)
			for _, k := range mo.Keys {
				vals.Set(k, mo.Vals[k].String())
			}
			// also range Vals in case keys incomplete
			for k, v := range mo.Vals {
				if vals.Get(k) == "" {
					vals.Set(k, v.String())
				}
			}
			u.RawQuery = vals.Encode()
		}
		return runtime.Str(u.String()), nil
	}, 1)

	// url.encode_query({k:v}) or url.encode_query("k", "v", ...)
	set(p, "encode_query", func(args []runtime.Value) (runtime.Value, error) {
		vals := url.Values{}
		if len(args) == 1 && args[0].Kind == runtime.KindMap {
			mo := args[0].Obj.(*runtime.MapObj)
			for _, k := range mo.Keys {
				vals.Set(k, mo.Vals[k].String())
			}
			for k, v := range mo.Vals {
				if vals.Get(k) == "" {
					vals.Set(k, v.String())
				}
			}
		} else {
			for i := 0; i+1 < len(args); i += 2 {
				vals.Set(args[i].String(), args[i+1].String())
			}
		}
		return runtime.Str(vals.Encode()), nil
	}, -1)

	// url.escape(s) / unescape
	set(p, "escape", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		return runtime.Str(url.QueryEscape(args[0].String())), nil
	}, 1)

	set(p, "unescape", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("url.unescape(s)", "url"), nil
		}
		s, err := url.QueryUnescape(args[0].String())
		if err != nil {
			return errRes(err.Error(), "url"), nil
		}
		return runtime.Ok(runtime.Str(s)), nil
	}, 1)

	// url.join(base, ref) -> str
	set(p, "join", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("url.join(base, ref)", "url"), nil
		}
		base, err := url.Parse(args[0].String())
		if err != nil {
			return errRes(err.Error(), "url"), nil
		}
		ref, err := url.Parse(args[1].String())
		if err != nil {
			return errRes(err.Error(), "url"), nil
		}
		return runtime.Str(base.ResolveReference(ref).String()), nil
	}, 2)

	// url.path_escape
	set(p, "path_escape", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		return runtime.Str(url.PathEscape(args[0].String())), nil
	}, 1)

	// url.merge_query(base_url, params_map) -> str  merge/override query keys
	set(p, "merge_query", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("url.merge_query(base, params)", "url"), nil
		}
		u, err := url.Parse(args[0].String())
		if err != nil {
			return errRes(err.Error(), "url"), nil
		}
		q := u.Query()
		if args[1].Kind == runtime.KindMap {
			mo := args[1].Obj.(*runtime.MapObj)
			for _, k := range mo.Keys {
				q.Set(k, mo.Vals[k].String())
			}
			for k, v := range mo.Vals {
				if q.Get(k) == "" {
					q.Set(k, v.String())
				}
			}
		}
		u.RawQuery = q.Encode()
		return runtime.Str(u.String()), nil
	}, 2)

	// url.path_unescape(s) -> Result[str]
	set(p, "path_unescape", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("url.path_unescape(s)", "url"), nil
		}
		s, err := url.PathUnescape(args[0].String())
		if err != nil {
			return errRes(err.Error(), "url"), nil
		}
		return runtime.Ok(runtime.Str(s)), nil
	}, 1)

	return p
}
