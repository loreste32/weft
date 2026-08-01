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
	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/pbkdf2"
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

	// crypto.hash(algo, s) -> hex
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

	// argon2id(password, salt, time?, memory?, threads?, keyLen?) → hex
	set(p, "argon2id", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("crypto.argon2id(password, salt, time?, memory?, threads?, keyLen?)", "crypto"), nil
		}
		password := []byte(args[0].String())
		salt := []byte(args[1].String())
		t := uint32(1)
		mem := uint32(64 * 1024)
		threads := uint8(4)
		keyLen := uint32(32)
		if len(args) > 2 {
			if v, err := runtime.AsInt(args[2]); err == nil && v > 0 {
				t = uint32(v)
			}
		}
		if len(args) > 3 {
			if v, err := runtime.AsInt(args[3]); err == nil && v > 0 {
				mem = uint32(v)
			}
		}
		if len(args) > 4 {
			if v, err := runtime.AsInt(args[4]); err == nil && v > 0 {
				threads = uint8(v)
			}
		}
		if len(args) > 5 {
			if v, err := runtime.AsInt(args[5]); err == nil && v > 0 {
				keyLen = uint32(v)
			}
		}
		// Enforce safety bounds at the Go level — cannot be bypassed from Weft
		if t < 1 || t > 10 {
			return errRes("crypto.argon2id: time must be 1-10", "crypto"), nil
		}
		if mem < 8*1024 || mem > 256*1024 {
			return errRes("crypto.argon2id: memory must be 8192-262144 KiB (8-256 MiB)", "crypto"), nil
		}
		if threads < 1 || threads > 16 {
			return errRes("crypto.argon2id: threads must be 1-16", "crypto"), nil
		}
		if keyLen < 16 || keyLen > 64 {
			return errRes("crypto.argon2id: keyLen must be 16-64", "crypto"), nil
		}
		key := argon2.IDKey(password, salt, t, mem, threads, keyLen)
		return runtime.Str(hex.EncodeToString(key)), nil
	}, 2)

	// pbkdf2(password, salt, iterations?, keyLen?, algo?) → hex
	set(p, "pbkdf2", func(args []runtime.Value) (runtime.Value, error) {
		if len(args) < 2 {
			return errRes("crypto.pbkdf2(password, salt, iterations?, keyLen?, algo?)", "crypto"), nil
		}
		password := []byte(args[0].String())
		salt := []byte(args[1].String())
		iter := 100000
		keyLen := 32
		var h func() hash.Hash = sha256.New
		if len(args) > 2 {
			if v, err := runtime.AsInt(args[2]); err == nil && v > 0 {
				iter = int(v)
			}
		}
		if len(args) > 3 {
			if v, err := runtime.AsInt(args[3]); err == nil && v > 0 {
				keyLen = int(v)
			}
		}
		if len(args) > 4 {
			switch strings.ToLower(args[4].String()) {
			case "sha512":
				h = sha512.New
			case "sha1":
				h = sha1.New
			}
		}
		// Enforce safety bounds at the Go level
		if iter < 10000 || iter > 10_000_000 {
			return errRes("crypto.pbkdf2: iterations must be 10000-10000000", "crypto"), nil
		}
		if keyLen < 16 || keyLen > 64 {
			return errRes("crypto.pbkdf2: keyLen must be 16-64", "crypto"), nil
		}
		key := pbkdf2.Key(password, salt, iter, keyLen, h)
		return runtime.Str(hex.EncodeToString(key)), nil
	}, 2)

	return p
}
