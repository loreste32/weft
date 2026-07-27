package stdlib

import (
	"encoding/base64"
	"encoding/hex"

	"github.com/loreste/weft/internal/runtime"
)

// packageBase64 — base64 + hex codecs (Python base64 / binascii lite).
func packageBase64() runtime.Value {
	p := pkg()

	set(p, "encode", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		return runtime.Str(base64.StdEncoding.EncodeToString([]byte(args[0].String()))), nil
	}, 1)

	set(p, "decode", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("base64.decode(s)", "base64"), nil
		}
		b, err := base64.StdEncoding.DecodeString(args[0].String())
		if err != nil {
			// try raw std without padding issues
			b, err = base64.RawStdEncoding.DecodeString(args[0].String())
			if err != nil {
				return errRes(err.Error(), "base64"), nil
			}
		}
		return runtime.Ok(runtime.Str(string(b))), nil
	}, 1)

	set(p, "url_encode", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		return runtime.Str(base64.URLEncoding.EncodeToString([]byte(args[0].String()))), nil
	}, 1)

	set(p, "url_decode", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("base64.url_decode(s)", "base64"), nil
		}
		b, err := base64.URLEncoding.DecodeString(args[0].String())
		if err != nil {
			b, err = base64.RawURLEncoding.DecodeString(args[0].String())
			if err != nil {
				return errRes(err.Error(), "base64"), nil
			}
		}
		return runtime.Ok(runtime.Str(string(b))), nil
	}, 1)

	// base64.hex_encode / hex_decode
	set(p, "hex_encode", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		return runtime.Str(hex.EncodeToString([]byte(args[0].String()))), nil
	}, 1)

	set(p, "hex_decode", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("base64.hex_decode(s)", "base64"), nil
		}
		b, err := hex.DecodeString(args[0].String())
		if err != nil {
			return errRes(err.Error(), "base64"), nil
		}
		return runtime.Ok(runtime.Str(string(b))), nil
	}, 1)

	// binascii-style aliases
	set(p, "b2a_hex", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		return runtime.Str(hex.EncodeToString([]byte(args[0].String()))), nil
	}, 1)
	set(p, "a2b_hex", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("base64.a2b_hex(s)", "base64"), nil
		}
		b, err := hex.DecodeString(args[0].String())
		if err != nil {
			return errRes(err.Error(), "base64"), nil
		}
		return runtime.Ok(runtime.Str(string(b))), nil
	}, 1)
	set(p, "b2a_base64", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		return runtime.Str(base64.StdEncoding.EncodeToString([]byte(args[0].String()))), nil
	}, 1)
	set(p, "a2b_base64", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("base64.a2b_base64(s)", "base64"), nil
		}
		b, err := base64.StdEncoding.DecodeString(args[0].String())
		if err != nil {
			return errRes(err.Error(), "base64"), nil
		}
		return runtime.Ok(runtime.Str(string(b))), nil
	}, 1)

	return p
}
