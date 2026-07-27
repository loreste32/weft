package stdlib

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/loreste/weft/internal/runtime"
)

// packageArchive — zip / gzip / tar for scripts (safe path rules on extract).
func packageArchive() runtime.Value {
	p := pkg()

	// archive.zip(out_path, [files|dir], opts?) -> Result[str]
	// files: list of paths or {path, name?} maps
	set(p, "zip", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("archive.zip(out, files|dir)", "archive"), nil
		}
		outPath := args[0].String()
		if dir := filepath.Dir(outPath); dir != "" && dir != "." {
			_ = os.MkdirAll(dir, 0o755)
		}
		f, err := os.Create(outPath)
		if err != nil {
			return errRes(err.Error(), "archive"), nil
		}
		zw := zip.NewWriter(f)
		addFile := func(src, name string) error {
			st, err := os.Stat(src)
			if err != nil {
				return err
			}
			if st.IsDir() {
				return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}
					if info.IsDir() {
						return nil
					}
					rel, err := filepath.Rel(src, path)
					if err != nil {
						return err
					}
					return writeZipFile(zw, path, filepath.ToSlash(rel))
				})
			}
			if name == "" {
				name = filepath.Base(src)
			}
			return writeZipFile(zw, src, filepath.ToSlash(name))
		}

		switch args[1].Kind {
		case runtime.KindStr:
			if err := addFile(args[1].String(), ""); err != nil {
				_ = zw.Close()
				_ = f.Close()
				return errRes(err.Error(), "archive"), nil
			}
		case runtime.KindList:
			for _, it := range args[1].Obj.(*runtime.ListObj).Items {
				switch it.Kind {
				case runtime.KindStr:
					if err := addFile(it.String(), ""); err != nil {
						_ = zw.Close()
						_ = f.Close()
						return errRes(err.Error(), "archive"), nil
					}
				case runtime.KindMap:
					src := mapGetStr(it, "path", mapGetStr(it, "src", ""))
					name := mapGetStr(it, "name", "")
					if src == "" {
						continue
					}
					if err := addFile(src, name); err != nil {
						_ = zw.Close()
						_ = f.Close()
						return errRes(err.Error(), "archive"), nil
					}
				}
			}
		default:
			_ = zw.Close()
			_ = f.Close()
			return errRes("archive.zip: files must be path or list", "archive"), nil
		}
		if err := zw.Close(); err != nil {
			_ = f.Close()
			return errRes(err.Error(), "archive"), nil
		}
		if err := f.Close(); err != nil {
			return errRes(err.Error(), "archive"), nil
		}
		return runtime.Ok(runtime.Str(outPath)), nil
	}, 2)

	// archive.unzip(zip_path, dest_dir) -> Result[[str]] extracted names
	set(p, "unzip", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("archive.unzip(zip, dest)", "archive"), nil
		}
		src, dest := args[0].String(), args[1].String()
		if err := os.MkdirAll(dest, 0o755); err != nil {
			return errRes(err.Error(), "archive"), nil
		}
		r, err := zip.OpenReader(src)
		if err != nil {
			return errRes(err.Error(), "archive"), nil
		}
		defer r.Close()
		var names []runtime.Value
		for _, zf := range r.File {
			if zf.Mode()&os.ModeSymlink != 0 {
				return errRes("illegal symlink in zip: "+zf.Name, "archive"), nil
			}
			target, err := archiveSafeJoin(dest, zf.Name)
			if err != nil {
				return errRes(err.Error(), "archive"), nil
			}
			if zf.FileInfo().IsDir() {
				_ = os.MkdirAll(target, 0o755)
				continue
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return errRes(err.Error(), "archive"), nil
			}
			rc, err := zf.Open()
			if err != nil {
				return errRes(err.Error(), "archive"), nil
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
			if err != nil {
				rc.Close()
				return errRes(err.Error(), "archive"), nil
			}
			_, err = io.Copy(out, io.LimitReader(rc, 200<<20))
			out.Close()
			rc.Close()
			if err != nil {
				return errRes(err.Error(), "archive"), nil
			}
			rel, _ := filepath.Rel(dest, target)
			names = append(names, runtime.Str(filepath.ToSlash(rel)))
		}
		return runtime.Ok(runtime.List(names...)), nil
	}, 2)

	// archive.list(zip_path) -> Result[[str]]
	set(p, "list", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("archive.list(zip)", "archive"), nil
		}
		r, err := zip.OpenReader(args[0].String())
		if err != nil {
			return errRes(err.Error(), "archive"), nil
		}
		defer r.Close()
		var names []runtime.Value
		for _, zf := range r.File {
			names = append(names, runtime.Str(zf.Name))
		}
		return runtime.Ok(runtime.List(names...)), nil
	}, 1)

	// archive.gzip(src_path, dest_path?) -> Result[str]  dest defaults to src.gz
	set(p, "gzip", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("archive.gzip(src, dest?)", "archive"), nil
		}
		src := args[0].String()
		dest := src + ".gz"
		if len(args) >= 2 && args[1].String() != "" {
			dest = args[1].String()
		}
		in, err := os.Open(src)
		if err != nil {
			return errRes(err.Error(), "archive"), nil
		}
		defer in.Close()
		if dir := filepath.Dir(dest); dir != "" && dir != "." {
			_ = os.MkdirAll(dir, 0o755)
		}
		out, err := os.Create(dest)
		if err != nil {
			return errRes(err.Error(), "archive"), nil
		}
		gw := gzip.NewWriter(out)
		if _, err := io.Copy(gw, in); err != nil {
			_ = gw.Close()
			_ = out.Close()
			return errRes(err.Error(), "archive"), nil
		}
		if err := gw.Close(); err != nil {
			_ = out.Close()
			return errRes(err.Error(), "archive"), nil
		}
		if err := out.Close(); err != nil {
			return errRes(err.Error(), "archive"), nil
		}
		return runtime.Ok(runtime.Str(dest)), nil
	}, 2)

	// archive.gunzip(src.gz, dest?) -> Result[str]
	set(p, "gunzip", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("archive.gunzip(src, dest?)", "archive"), nil
		}
		src := args[0].String()
		dest := strings.TrimSuffix(src, ".gz")
		if dest == src {
			dest = src + ".out"
		}
		if len(args) >= 2 && args[1].String() != "" {
			dest = args[1].String()
		}
		in, err := os.Open(src)
		if err != nil {
			return errRes(err.Error(), "archive"), nil
		}
		defer in.Close()
		gr, err := gzip.NewReader(in)
		if err != nil {
			return errRes(err.Error(), "archive"), nil
		}
		defer gr.Close()
		if dir := filepath.Dir(dest); dir != "" && dir != "." {
			_ = os.MkdirAll(dir, 0o755)
		}
		out, err := os.Create(dest)
		if err != nil {
			return errRes(err.Error(), "archive"), nil
		}
		if _, err := io.Copy(out, gr); err != nil {
			_ = out.Close()
			return errRes(err.Error(), "archive"), nil
		}
		if err := out.Close(); err != nil {
			return errRes(err.Error(), "archive"), nil
		}
		return runtime.Ok(runtime.Str(dest)), nil
	}, 2)

	// archive.tar(out_path, [files|dir]) -> Result[str]  uncompressed tar
	set(p, "tar", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("archive.tar(out, files|dir)", "archive"), nil
		}
		outPath := args[0].String()
		if dir := filepath.Dir(outPath); dir != "" && dir != "." {
			_ = os.MkdirAll(dir, 0o755)
		}
		f, err := os.Create(outPath)
		if err != nil {
			return errRes(err.Error(), "archive"), nil
		}
		tw := tar.NewWriter(f)
		add := func(src, name string) error {
			st, err := os.Stat(src)
			if err != nil {
				return err
			}
			if st.IsDir() {
				return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
					if err != nil || info.IsDir() {
						return err
					}
					rel, err := filepath.Rel(src, path)
					if err != nil {
						return err
					}
					return writeTarFile(tw, path, filepath.ToSlash(rel))
				})
			}
			return writeTarFile(tw, src, name)
		}
		if err := archiveAddArgs(args[1], add); err != nil {
			_ = tw.Close()
			_ = f.Close()
			return errRes(err.Error(), "archive"), nil
		}
		if err := tw.Close(); err != nil {
			_ = f.Close()
			return errRes(err.Error(), "archive"), nil
		}
		if err := f.Close(); err != nil {
			return errRes(err.Error(), "archive"), nil
		}
		return runtime.Ok(runtime.Str(outPath)), nil
	}, 2)

	// archive.untar(tar_path, dest) -> Result[[str]]
	set(p, "untar", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("archive.untar(tar, dest)", "archive"), nil
		}
		src, dest := args[0].String(), args[1].String()
		if err := os.MkdirAll(dest, 0o755); err != nil {
			return errRes(err.Error(), "archive"), nil
		}
		f, err := os.Open(src)
		if err != nil {
			return errRes(err.Error(), "archive"), nil
		}
		defer f.Close()
		var rdr io.Reader = f
		// auto gunzip if .gz
		if strings.HasSuffix(strings.ToLower(src), ".gz") || strings.HasSuffix(strings.ToLower(src), ".tgz") {
			gr, err := gzip.NewReader(f)
			if err != nil {
				return errRes(err.Error(), "archive"), nil
			}
			defer gr.Close()
			rdr = gr
		}
		tr := tar.NewReader(rdr)
		var names []runtime.Value
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return errRes(err.Error(), "archive"), nil
			}
			target, err := archiveSafeJoin(dest, hdr.Name)
			if err != nil {
				return errRes(err.Error(), "archive"), nil
			}
			switch hdr.Typeflag {
			case tar.TypeDir:
				if err := os.MkdirAll(target, 0o755); err != nil {
					return errRes(err.Error(), "archive"), nil
				}
			case tar.TypeReg:
				if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
					return errRes(err.Error(), "archive"), nil
				}
				out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)&0o777)
				if err != nil {
					return errRes(err.Error(), "archive"), nil
				}
				if _, err := io.Copy(out, tr); err != nil {
					_ = out.Close()
					return errRes(err.Error(), "archive"), nil
				}
				_ = out.Close()
				rel, _ := filepath.Rel(dest, target)
				names = append(names, runtime.Str(filepath.ToSlash(rel)))
			}
		}
		return runtime.Ok(runtime.List(names...)), nil
	}, 2)

	return p
}

func writeTarFile(tw *tar.Writer, src, name string) error {
	st, err := os.Stat(src)
	if err != nil {
		return err
	}
	hdr, err := tar.FileInfoHeader(st, "")
	if err != nil {
		return err
	}
	hdr.Name = name
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	_, err = io.Copy(tw, in)
	return err
}

// archiveAddArgs handles list of paths or single dir for zip/tar writers.
func archiveAddArgs(arg runtime.Value, add func(src, name string) error) error {
	if arg.Kind == runtime.KindStr {
		src := arg.String()
		return add(src, filepath.Base(src))
	}
	if arg.Kind != runtime.KindList {
		return fmt.Errorf("files must be list or path")
	}
	for _, it := range arg.Obj.(*runtime.ListObj).Items {
		if it.Kind == runtime.KindMap {
			src := mapGetStr(it, "path", mapGetStr(it, "src", ""))
			name := mapGetStr(it, "name", filepath.Base(src))
			if src == "" {
				continue
			}
			if err := add(src, name); err != nil {
				return err
			}
			continue
		}
		src := it.String()
		if err := add(src, filepath.Base(src)); err != nil {
			return err
		}
	}
	return nil
}

func writeZipFile(zw *zip.Writer, src, name string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, in)
	return err
}

func archiveSafeJoin(dest, name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("empty archive path")
	}
	n := strings.ReplaceAll(name, "\\", "/")
	if strings.Contains(n, "\x00") {
		return "", fmt.Errorf("illegal path (null byte)")
	}
	if strings.HasPrefix(n, "/") || strings.HasPrefix(n, "//") {
		return "", fmt.Errorf("illegal absolute path: %q", name)
	}
	if len(n) >= 2 && n[1] == ':' {
		return "", fmt.Errorf("illegal drive path: %q", name)
	}
	parts := strings.Split(n, "/")
	var clean []string
	for _, p := range parts {
		if p == "" || p == "." {
			continue
		}
		if p == ".." {
			return "", fmt.Errorf("illegal path traversal: %q", name)
		}
		clean = append(clean, p)
	}
	if len(clean) == 0 {
		return "", fmt.Errorf("empty path")
	}
	rel := filepath.Join(clean...)
	target := filepath.Join(dest, rel)
	destClean := filepath.Clean(dest)
	targetClean := filepath.Clean(target)
	sep := string(filepath.Separator)
	if targetClean != destClean && !strings.HasPrefix(targetClean, destClean+sep) {
		return "", fmt.Errorf("path escapes dest: %q", name)
	}
	return targetClean, nil
}
