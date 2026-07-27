package stdlib

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/loreste/weft/internal/runtime"
)

// packageSH runs external processes for devops CLIs.
func packageSH(env *runtime.Env) runtime.Value {
	p := pkg()

	// sh.run(cmd, args?, opts?) -> Result[{code, stdout, stderr, ok}]
	// sh.run("git", ["status"])
	// sh.run(["git", "status"])
	set(p, "run", func(args []runtime.Value) (runtime.Value, error) {
		cmd, argv, opts, err := parseShArgs(args)
		if err != nil {
			return errRes(err.Error(), "sh"), nil
		}
		return runCmd(env, cmd, argv, opts, false)
	}, 3)

	// sh.capture(cmd, args?) -> Result[str]  (stdout, fails if non-zero)
	set(p, "capture", func(args []runtime.Value) (runtime.Value, error) {
		cmd, argv, opts, err := parseShArgs(args)
		if err != nil {
			return errRes(err.Error(), "sh"), nil
		}
		opts.check = true
		res, err := runCmd(env, cmd, argv, opts, true)
		if err != nil {
			return res, err
		}
		if res.Kind == runtime.KindResult {
			ro := res.Obj.(*runtime.ResultObj)
			if !ro.Ok {
				return res, nil
			}
			if out, ok := mapGet(ro.Val, "stdout"); ok {
				return runtime.Ok(out), nil
			}
		}
		return res, nil
	}, 2)

	// sh.ok(cmd, args?) -> Result[bool]  true if exit 0
	set(p, "ok", func(args []runtime.Value) (runtime.Value, error) {
		cmd, argv, opts, err := parseShArgs(args)
		if err != nil {
			return errRes(err.Error(), "sh"), nil
		}
		res, err := runCmd(env, cmd, argv, opts, false)
		if err != nil {
			return res, err
		}
		if res.Kind == runtime.KindResult {
			ro := res.Obj.(*runtime.ResultObj)
			if !ro.Ok {
				return runtime.Ok(runtime.Bool(false)), nil
			}
			if c, ok := mapGet(ro.Val, "ok"); ok {
				return runtime.Ok(c), nil
			}
		}
		return runtime.Ok(runtime.Bool(false)), nil
	}, 2)

	// sh.shell(line) -> Result[...]  runs via sh -c (devops glue)
	set(p, "shell", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("sh.shell(line)", "sh"), nil
		}
		line := args[0].String()
		opts := shOpts{}
		if len(args) >= 2 {
			opts = parseShOpts(args[1])
		}
		return runCmd(env, "sh", []string{"-c", line}, opts, false)
	}, 2)

	// sh.which(bin) -> str|null
	set(p, "which", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Null(), nil
		}
		p, err := exec.LookPath(args[0].String())
		if err != nil {
			return runtime.Null(), nil
		}
		return runtime.Str(p), nil
	}, 1)

	// sh.lines(cmd, args?, opts?) -> Result[[str]]  stdout lines (fails if non-zero)
	set(p, "lines", func(args []runtime.Value) (runtime.Value, error) {
		cmd, argv, opts, err := parseShArgs(args)
		if err != nil {
			return errRes(err.Error(), "sh"), nil
		}
		opts.check = true
		res, err := runCmd(env, cmd, argv, opts, true)
		if err != nil {
			return res, err
		}
		if res.Kind != runtime.KindResult {
			return res, nil
		}
		ro := res.Obj.(*runtime.ResultObj)
		if !ro.Ok {
			return res, nil
		}
		out, _ := mapGet(ro.Val, "stdout")
		text := strings.TrimSuffix(out.String(), "\n")
		if text == "" {
			return runtime.Ok(runtime.List()), nil
		}
		parts := strings.Split(text, "\n")
		items := make([]runtime.Value, len(parts))
		for i, s := range parts {
			items[i] = runtime.Str(s)
		}
		return runtime.Ok(runtime.List(items...)), nil
	}, 3)

	// sh.code(cmd, args?, opts?) -> Result[int]  exit code only
	set(p, "code", func(args []runtime.Value) (runtime.Value, error) {
		cmd, argv, opts, err := parseShArgs(args)
		if err != nil {
			return errRes(err.Error(), "sh"), nil
		}
		res, err := runCmd(env, cmd, argv, opts, false)
		if err != nil {
			return res, err
		}
		if res.Kind != runtime.KindResult {
			return res, nil
		}
		ro := res.Obj.(*runtime.ResultObj)
		if !ro.Ok {
			// timeout / start failure
			return res, nil
		}
		c, _ := mapGet(ro.Val, "code")
		return runtime.Ok(c), nil
	}, 3)

	return p
}

type shOpts struct {
	dir      string
	env      []string // KEY=VAL extra
	timeout  time.Duration
	check    bool // fail Result on non-zero
	stdin    string
	mergeOut bool // combined stdout+stderr
}

func parseShArgs(args []runtime.Value) (cmd string, argv []string, opts shOpts, err error) {
	if len(args) < 1 {
		return "", nil, opts, fmt.Errorf("sh.run needs command")
	}
	if args[0].Kind == runtime.KindList {
		lo := args[0].Obj.(*runtime.ListObj)
		if len(lo.Items) == 0 {
			return "", nil, opts, fmt.Errorf("empty command list")
		}
		cmd = lo.Items[0].String()
		for _, it := range lo.Items[1:] {
			argv = append(argv, it.String())
		}
		if len(args) >= 2 {
			opts = parseShOpts(args[1])
		}
		return cmd, argv, opts, nil
	}
	cmd = args[0].String()
	if len(args) >= 2 && args[1].Kind == runtime.KindList {
		for _, it := range args[1].Obj.(*runtime.ListObj).Items {
			argv = append(argv, it.String())
		}
		if len(args) >= 3 {
			opts = parseShOpts(args[2])
		}
	} else if len(args) >= 2 && (args[1].Kind == runtime.KindMap || args[1].Kind == runtime.KindStruct) {
		opts = parseShOpts(args[1])
	}
	return cmd, argv, opts, nil
}

func parseShOpts(v runtime.Value) shOpts {
	o := shOpts{}
	if v.Kind != runtime.KindMap && v.Kind != runtime.KindStruct {
		return o
	}
	o.dir = mapGetStr(v, "dir", "")
	if o.dir == "" {
		o.dir = mapGetStr(v, "cwd", "")
	}
	o.stdin = mapGetStr(v, "stdin", "")
	if n := mapGetInt(v, "timeout", 0); n > 0 {
		o.timeout = time.Duration(n) * time.Second
	} else if s := mapGetStr(v, "timeout", ""); s != "" {
		if d, err := time.ParseDuration(s); err == nil {
			o.timeout = d
		}
	}
	if b, ok := mapGet(v, "check"); ok && b.Kind == runtime.KindBool {
		o.check = b.B
	}
	if b, ok := mapGet(v, "merge"); ok && b.Kind == runtime.KindBool {
		o.mergeOut = b.B
	}
	if e, ok := mapGet(v, "env"); ok && e.Kind == runtime.KindMap {
		mo := e.Obj.(*runtime.MapObj)
		for _, k := range mo.Keys {
			o.env = append(o.env, k+"="+mo.Vals[k].String())
		}
	} else if e, ok := mapGet(v, "env"); ok && e.Kind == runtime.KindList {
		for _, it := range e.Obj.(*runtime.ListObj).Items {
			o.env = append(o.env, it.String())
		}
	}
	return o
}

func runCmd(env *runtime.Env, name string, argv []string, opts shOpts, captureOnly bool) (runtime.Value, error) {
	c := exec.Command(name, argv...)
	if opts.dir != "" {
		c.Dir = opts.dir
	}
	if len(opts.env) > 0 {
		c.Env = append(c.Environ(), opts.env...)
	}
	var stdout, stderr bytes.Buffer
	if opts.mergeOut {
		c.Stdout = &stdout
		c.Stderr = &stdout
	} else {
		c.Stdout = &stdout
		c.Stderr = &stderr
	}
	if opts.stdin != "" {
		c.Stdin = strings.NewReader(opts.stdin)
	}

	var err error
	if opts.timeout > 0 {
		err = c.Start()
		if err == nil {
			done := make(chan error, 1)
			go func() { done <- c.Wait() }()
			select {
			case err = <-done:
			case <-time.After(opts.timeout):
				if c.Process != nil {
					_ = c.Process.Kill()
				}
				err = fmt.Errorf("timeout after %s", opts.timeout)
			}
		}
	} else {
		err = c.Run()
	}

	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else if opts.timeout > 0 && err != nil {
			return errRes(err.Error(), "sh"), nil
		} else if _, ok := err.(*exec.Error); ok {
			return errRes(err.Error(), "sh"), nil
		} else {
			if code == 0 {
				code = 1
			}
		}
	}
	ok := code == 0
	m := runtime.NewMap()
	mo := m.Obj.(*runtime.MapObj)
	put := func(k string, v runtime.Value) {
		mo.Keys = append(mo.Keys, k)
		mo.Vals[k] = v
	}
	put("code", runtime.Int(int64(code)))
	put("ok", runtime.Bool(ok))
	put("stdout", runtime.Str(stdout.String()))
	put("stderr", runtime.Str(stderr.String()))
	if opts.mergeOut {
		put("output", runtime.Str(stdout.String()))
	}
	put("cmd", runtime.Str(name+" "+strings.Join(argv, " ")))

	if (opts.check || captureOnly) && !ok {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg == "" {
			msg = fmt.Sprintf("command exited %d", code)
		}
		return errRes(msg, "sh"), nil
	}
	return runtime.Ok(m), nil
}
