package stdlib

import (
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"net/url"

	"github.com/loreste/weft/internal/runtime"
)

// packageEncoding — hex, base32, URL encoding beyond base64.
func packageEncoding() runtime.Value {
	p := pkg()

	// hex
	set(p, "hex_encode", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("encoding.hex_encode(data)", "encoding"), nil
		}
		return runtime.Str(hex.EncodeToString([]byte(args[0].String()))), nil
	}, 1)

	set(p, "hex_decode", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("encoding.hex_decode(s)", "encoding"), nil
		}
		b, err := hex.DecodeString(args[0].String())
		if err != nil {
			return errRes(err.Error(), "encoding"), nil
		}
		return runtime.Ok(runtime.Str(string(b))), nil
	}, 1)

	// base32
	set(p, "base32_encode", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("encoding.base32_encode(data)", "encoding"), nil
		}
		return runtime.Str(base32.StdEncoding.EncodeToString([]byte(args[0].String()))), nil
	}, 1)

	set(p, "base32_decode", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("encoding.base32_decode(s)", "encoding"), nil
		}
		b, err := base32.StdEncoding.DecodeString(args[0].String())
		if err != nil {
			return errRes(err.Error(), "encoding"), nil
		}
		return runtime.Ok(runtime.Str(string(b))), nil
	}, 1)

	// URL encoding
	set(p, "url_encode", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("encoding.url_encode(s)", "encoding"), nil
		}
		return runtime.Str(url.QueryEscape(args[0].String())), nil
	}, 1)

	set(p, "url_decode", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("encoding.url_decode(s)", "encoding"), nil
		}
		s, err := url.QueryUnescape(args[0].String())
		if err != nil {
			return errRes(err.Error(), "encoding"), nil
		}
		return runtime.Ok(runtime.Str(s)), nil
	}, 1)

	// path encoding
	set(p, "path_encode", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("encoding.path_encode(s)", "encoding"), nil
		}
		return runtime.Str(url.PathEscape(args[0].String())), nil
	}, 1)

	set(p, "path_decode", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("encoding.path_decode(s)", "encoding"), nil
		}
		s, err := url.PathUnescape(args[0].String())
		if err != nil {
			return errRes(err.Error(), "encoding"), nil
		}
		return runtime.Ok(runtime.Str(s)), nil
	}, 1)

	// hex helpers
	set(p, "to_hex", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("encoding.to_hex(n)", "encoding"), nil
		}
		if n, err := runtime.AsInt(args[0]); err == nil {
			return runtime.Str(fmt.Sprintf("%X", n)), nil
		}
		return runtime.Str(hex.EncodeToString([]byte(args[0].String()))), nil
	}, 1)

	return p
}
