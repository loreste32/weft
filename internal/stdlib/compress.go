package stdlib

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"io"

	"github.com/loreste/weft/internal/runtime"
)

// packageCompress — gzip/deflate/zlib compression.
func packageCompress() runtime.Value {
	p := pkg()

	set(p, "gzip", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("compress.gzip(data)", "compress"), nil
		}
		var buf bytes.Buffer
		w := gzip.NewWriter(&buf)
		w.Write([]byte(args[0].String()))
		w.Close()
		return runtime.Ok(runtime.Str(buf.String())), nil
	}, 1)

	set(p, "gunzip", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("compress.gunzip(data)", "compress"), nil
		}
		r, err := gzip.NewReader(bytes.NewReader([]byte(args[0].String())))
		if err != nil {
			return errRes(err.Error(), "compress"), nil
		}
		defer r.Close()
		b, err := io.ReadAll(io.LimitReader(r, 100<<20))
		if err != nil {
			return errRes(err.Error(), "compress"), nil
		}
		return runtime.Ok(runtime.Str(string(b))), nil
	}, 1)

	set(p, "deflate", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("compress.deflate(data)", "compress"), nil
		}
		var buf bytes.Buffer
		w, _ := zlib.NewWriterLevel(&buf, zlib.DefaultCompression)
		w.Write([]byte(args[0].String()))
		w.Close()
		return runtime.Ok(runtime.Str(buf.String())), nil
	}, 1)

	set(p, "inflate", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("compress.inflate(data)", "compress"), nil
		}
		r, err := zlib.NewReader(bytes.NewReader([]byte(args[0].String())))
		if err != nil {
			return errRes(err.Error(), "compress"), nil
		}
		defer r.Close()
		b, err := io.ReadAll(io.LimitReader(r, 100<<20))
		if err != nil {
			return errRes(err.Error(), "compress"), nil
		}
		return runtime.Ok(runtime.Str(string(b))), nil
	}, 1)

	return p
}
