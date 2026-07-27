package stdlib

import (
	"testing"

	"github.com/loreste/weft/internal/runtime"
)

func FuzzXMLParse(f *testing.F) {
	f.Add(`<root><a>1</a></root>`)
	f.Add(`<?xml version="1.0"?><x/>`)
	f.Add(`<<<`)
	f.Add(``)
	f.Add(string([]byte{0xff, 0xfe, '<'}))
	f.Fuzz(func(t *testing.T, s string) {
		_, _ = parseXMLRoot(s)
	})
}

func FuzzArchiveSafeJoin(f *testing.F) {
	f.Add("../etc/passwd")
	f.Add("ok/file.txt")
	f.Add("/abs")
	f.Add("a/../../b")
	f.Add("C:\\windows")
	f.Fuzz(func(t *testing.T, name string) {
		_, _ = archiveSafeJoin("/tmp/dest", name)
	})
}

func FuzzPickleGob(f *testing.F) {
	f.Add("hello")
	f.Add("")
	f.Add(string(make([]byte, 256)))
	f.Fuzz(func(t *testing.T, s string) {
		// only decode path is adversarial for panic; loads base64 of random
		_, _ = gobDecodeValue([]byte(s))
	})
}

func FuzzDecimalParse(f *testing.F) {
	f.Add("1.5")
	f.Add("1/3")
	f.Add("notanumber")
	f.Add("")
	f.Fuzz(func(t *testing.T, s string) {
		_, _ = parseRat(runtime.Str(s))
	})
}
