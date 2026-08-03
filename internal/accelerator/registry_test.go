package accelerator

import (
	"strings"
	"testing"
)

type fakeNative struct {
	manifestJSON []byte
	output       []byte
	closed       bool
}

func (f *fakeNative) manifest() ([]byte, error)                   { return f.manifestJSON, nil }
func (f *fakeNative) run(op string, input []byte) ([]byte, error) { return f.output, nil }
func (f *fakeNative) close() error {
	f.closed = true
	return nil
}

func TestValidateManifest(t *testing.T) {
	tests := []struct {
		name string
		m    Manifest
		want string
	}{
		{"missing name", Manifest{ABI: ABI, Version: "1", Vendors: []string{"cpu"}}, "name"},
		{"wrong abi", Manifest{Name: "x", ABI: ABI + 1, Version: "1", Vendors: []string{"cpu"}}, "ABI"},
		{"missing version", Manifest{Name: "x", ABI: ABI, Vendors: []string{"cpu"}}, "version"},
		{"missing vendor", Manifest{Name: "x", ABI: ABI, Version: "1"}, "vendor"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateManifest(tt.m)
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.want)) {
				t.Fatalf("validateManifest(%+v) = %v, want error containing %q", tt.m, err, tt.want)
			}
		})
	}
}

func TestRunJSONValidatesOutputAndCloses(t *testing.T) {
	fake := &fakeNative{
		output: []byte(`{"ok":true}`),
	}
	p := &Plugin{path: "/tmp/test-accelerator", manifest: Manifest{Name: "test", Version: "1", ABI: ABI, Vendors: []string{"cpu"}}, native: fake}
	out, err := p.RunJSON("identity", []byte(`{"x":1}`))
	if err != nil || string(out) != `{"ok":true}` {
		t.Fatalf("RunJSON() = %s, %v", out, err)
	}
	if err := p.Close(); err != nil || !fake.closed {
		t.Fatalf("Close() = %v, closed=%v", err, fake.closed)
	}
	if _, err := p.RunJSON("identity", []byte(`{}`)); err == nil {
		t.Fatal("RunJSON after Close unexpectedly succeeded")
	}
}

func TestRunJSONRejectsInvalidOutput(t *testing.T) {
	fake := &fakeNative{output: []byte("not-json")}
	p := &Plugin{manifest: Manifest{Name: "test", Version: "1", ABI: ABI, Vendors: []string{"cpu"}}, native: fake}
	if _, err := p.RunJSON("identity", []byte(`{}`)); err == nil || !strings.Contains(err.Error(), "invalid JSON") {
		t.Fatalf("RunJSON invalid output error = %v", err)
	}
}
