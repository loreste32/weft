package stdlib

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"hash"
	"io"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/loreste/weft/internal/runtime"
)

// packageCrypto — production primitives: hash, base64, uuid, random, hmac.
func packageCrypto() runtime.Value {
	p := pkg()

	hexHash := func(h hash.Hash, s string) string {
		_, _ = h.Write([]byte(s))
		return hex.EncodeToString(h.Sum(nil))
	}

	set(p, "md5", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		return runtime.Str(hexHash(md5.New(), args[0].String())), nil
	}, 1)

	set(p, "sha1", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		return runtime.Str(hexHash(sha1.New(), args[0].String())), nil
	}, 1)

	set(p, "sha256", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		sum := sha256.Sum256([]byte(args[0].String()))
		return runtime.Str(hex.EncodeToString(sum[:])), nil
	}, 1)

	set(p, "sha512", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		sum := sha512.Sum512([]byte(args[0].String()))
		return runtime.Str(hex.EncodeToString(sum[:])), nil
	}, 1)

	set(p, "b64_encode", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return runtime.Str(""), nil
		}
		return runtime.Str(base64.StdEncoding.EncodeToString([]byte(args[0].String()))), nil
	}, 1)

	set(p, "b64_decode", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("crypto.b64_decode(s)", "crypto"), nil
		}
		b, err := base64.StdEncoding.DecodeString(args[0].String())
		if err != nil {
			return errRes(err.Error(), "crypto"), nil
		}
		return runtime.Ok(runtime.Str(string(b))), nil
	}, 1)

	set(p, "uuid", func(args []runtime.Value) (runtime.Value, error) {
		return runtime.Str(uuid.NewString()), nil
	}, 0)

	set(p, "random_hex", func(args []runtime.Value) (runtime.Value, error) {
		n := 16
		if len(args) >= 1 {
			if x, err := runtime.AsInt(args[0]); err == nil && x > 0 && x < 1024 {
				n = int(x)
			}
		}
		b := make([]byte, n)
		if _, err := rand.Read(b); err != nil {
			return errRes(err.Error(), "crypto"), nil
		}
		return runtime.Str(hex.EncodeToString(b)), nil
	}, 1)

	set(p, "hmac_sha256", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("crypto.hmac_sha256(key, msg)", "crypto"), nil
		}
		mac := hmac.New(sha256.New, []byte(args[0].String()))
		_, _ = mac.Write([]byte(args[1].String()))
		return runtime.Str(hex.EncodeToString(mac.Sum(nil))), nil
	}, 2)

	set(p, "hmac_sha512", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("crypto.hmac_sha512(key, msg)", "crypto"), nil
		}
		mac := hmac.New(sha512.New, []byte(args[0].String()))
		_, _ = mac.Write([]byte(args[1].String()))
		return runtime.Str(hex.EncodeToString(mac.Sum(nil))), nil
	}, 2)

	// crypto.hash(algo, s) -> hex  algo: md5|sha1|sha256|sha512
	set(p, "hash", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("crypto.hash(algo, s)", "crypto"), nil
		}
		algo := strings.ToLower(args[0].String())
		s := args[1].String()
		switch algo {
		case "md5":
			return runtime.Str(hexHash(md5.New(), s)), nil
		case "sha1":
			return runtime.Str(hexHash(sha1.New(), s)), nil
		case "sha256":
			sum := sha256.Sum256([]byte(s))
			return runtime.Str(hex.EncodeToString(sum[:])), nil
		case "sha512":
			sum := sha512.Sum512([]byte(s))
			return runtime.Str(hex.EncodeToString(sum[:])), nil
		default:
			return errRes("crypto.hash: algo md5|sha1|sha256|sha512", "crypto"), nil
		}
	}, 2)

	// crypto.equal(a, b) -> bool  constant-time for equal-length strings (tokens)
	set(p, "equal", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return runtime.Bool(false), nil
		}
		a, b := []byte(args[0].String()), []byte(args[1].String())
		if len(a) != len(b) {
			return runtime.Bool(false), nil
		}
		return runtime.Bool(hmac.Equal(a, b)), nil
	}, 2)

	// crypto.random_bytes(n) -> Result[str]  CSPRNG
	set(p, "random_bytes", func(args []runtime.Value) (runtime.Value, error) {
		n := 32
		if len(args) >= 1 {
			if x, err := runtime.AsInt(args[0]); err == nil && x > 0 && x <= 1<<20 {
				n = int(x)
			}
		}
		b := make([]byte, n)
		if _, err := rand.Read(b); err != nil {
			return errRes(err.Error(), "crypto"), nil
		}
		return runtime.Ok(runtime.Str(string(b))), nil
	}, 1)

	// crypto.file_hash(path, algo?) -> Result[str]  algo: md5|sha1|sha256|sha512 (default sha256)
	set(p, "file_hash", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 1 {
			return errRes("crypto.file_hash(path, algo?)", "crypto"), nil
		}
		algo := "sha256"
		if len(args) >= 2 && args[1].String() != "" {
			algo = strings.ToLower(args[1].String())
		}
		var h hash.Hash
		switch algo {
		case "md5":
			h = md5.New()
		case "sha1":
			h = sha1.New()
		case "sha256":
			h = sha256.New()
		case "sha512":
			h = sha512.New()
		default:
			return errRes("crypto.file_hash: algo md5|sha1|sha256|sha512", "crypto"), nil
		}
		f, err := os.Open(args[0].String())
		if err != nil {
			return errRes(err.Error(), "crypto"), nil
		}
		defer f.Close()
		if _, err := io.Copy(h, f); err != nil {
			return errRes(err.Error(), "crypto"), nil
		}
		return runtime.Ok(runtime.Str(hex.EncodeToString(h.Sum(nil)))), nil
	}, 2)

	return p
}
