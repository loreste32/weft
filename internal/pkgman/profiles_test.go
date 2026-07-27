package pkgman

import (
	"reflect"
	"testing"
)

func TestExpandCapabilitiesProfile(t *testing.T) {
	got := ExpandCapabilities("data", nil)
	want := []string{"db", "redis", "mongo", "nats", "amqp"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestExpandCapabilitiesAtToken(t *testing.T) {
	got := ExpandCapabilities("", []string{"@net", "sh"})
	// net expands socket,email,http then sh
	if len(got) != 4 {
		t.Fatalf("got %v", got)
	}
	if !Allows(got, "socket") || !Allows(got, "email") || !Allows(got, "http") || !Allows(got, "sh") {
		t.Fatalf("expanded %v", got)
	}
	if Allows(got, "db") {
		t.Fatal("db should not be granted by @net+sh")
	}
}

func TestExpandFullProfile(t *testing.T) {
	got := ExpandCapabilities("full", nil)
	if !Allows(got, "secrets") || !Allows(got, "sh") {
		t.Fatalf("%v", got)
	}
}

func TestExpandNone(t *testing.T) {
	got := ExpandCapabilities("none", nil)
	if Allows(got, "sh") {
		t.Fatal("none must not grant sh")
	}
	if !Allows(got, "json") {
		t.Fatal("json always allowed")
	}
}

func TestValidateCapabilitiesProfiles(t *testing.T) {
	errs, warns := ValidateCapabilities([]string{"@data", "sh"})
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	_ = warns
	_, warns = ValidateCapabilities([]string{"@nope"})
	if len(warns) == 0 {
		t.Fatal("want warning for unknown profile")
	}
}

func TestLoadCapabilitiesWithProfile(t *testing.T) {
	dir := t.TempDir()
	if err := SaveManifest(dir, &Manifest{
		Name:              "x",
		CapabilityProfile: "data",
		Capabilities:      []string{"sh"},
	}); err != nil {
		t.Fatal(err)
	}
	caps := LoadCapabilities(dir)
	if !Allows(caps, "db") || !Allows(caps, "sh") {
		t.Fatalf("%v", caps)
	}
	if Allows(caps, "secrets") {
		t.Fatal("secrets not in data+sh")
	}
}
