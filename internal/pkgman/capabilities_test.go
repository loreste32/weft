package pkgman

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAllows(t *testing.T) {
	// nil caps → default-deny restricted
	if Allows(nil, "sh") {
		t.Fatal("sh should be denied by default")
	}
	if Allows(nil, "http") || Allows(nil, "fs") || Allows(nil, "llm") || Allows(nil, "env") {
		t.Fatal("io/llm/env should be denied by default")
	}
	if !Allows(nil, "math") {
		t.Fatal("math should be allowed by default")
	}
	// explicit grant
	if !Allows([]string{"sh"}, "sh") {
		t.Fatal("explicit grant")
	}
	// wildcard
	if !Allows([]string{"*"}, "sh") {
		t.Fatal("wildcard should allow all")
	}
}

func TestIsKnownCapability(t *testing.T) {
	if !IsKnownCapability("*") {
		t.Fatal("wildcard")
	}
	if !IsKnownCapability("sh") {
		t.Fatal("sh")
	}
	if IsKnownCapability("math") {
		t.Fatal("math is not restricted")
	}
}

func TestValidateCapabilities(t *testing.T) {
	// empty entry
	errs, _ := ValidateCapabilities([]string{""})
	if len(errs) == 0 {
		t.Fatal("empty should error")
	}
	// unknown capability
	_, warns := ValidateCapabilities([]string{"unknown_thing"})
	if len(warns) == 0 {
		t.Fatal("unknown should warn")
	}
	// valid
	errs, warns = ValidateCapabilities([]string{"sh", "db"})
	if len(errs) > 0 || len(warns) > 0 {
		t.Fatal("valid caps should pass")
	}
	// profile without @
	_, warns = ValidateCapabilities([]string{"data"})
	if len(warns) == 0 {
		t.Fatal("bare profile name should warn")
	}
	// @profile
	errs, warns = ValidateCapabilities([]string{"@data"})
	if len(errs) > 0 {
		t.Fatal("@data should be valid")
	}
	// unknown @profile
	_, warns = ValidateCapabilities([]string{"@nonexistent"})
	if len(warns) == 0 {
		t.Fatal("unknown @profile should warn")
	}
}

func TestValidateCapabilityProfile(t *testing.T) {
	errs, warns := ValidateCapabilityProfile("")
	if len(errs) > 0 || len(warns) > 0 {
		t.Fatal("empty should pass")
	}
	errs, warns = ValidateCapabilityProfile("full")
	if len(errs) > 0 || len(warns) > 0 {
		t.Fatal("full should pass")
	}
	_, warns = ValidateCapabilityProfile("nonexistent")
	if len(warns) == 0 {
		t.Fatal("unknown profile should warn")
	}
}

func TestLoadCapabilities(t *testing.T) {
	dir := t.TempDir()
	SaveManifest(dir, &Manifest{
		Name:         "test",
		Capabilities: []string{"sh", "db"},
	})
	caps := LoadCapabilities(dir)
	if len(caps) < 2 {
		t.Fatalf("caps: %v", caps)
	}
}

func TestLoadCapabilitiesMissing(t *testing.T) {
	caps := LoadCapabilities("/nonexistent")
	if caps != nil {
		t.Fatal("missing dir should return nil")
	}
}

func TestLoadCapabilitiesWithProfileCaps(t *testing.T) {
	dir := t.TempDir()
	SaveManifest(dir, &Manifest{
		Name:              "test",
		CapabilityProfile: "data",
		Capabilities:      []string{"sh"},
	})
	caps := LoadCapabilities(dir)
	if len(caps) == 0 {
		t.Fatal("should expand profile")
	}
	// should contain sh from explicit + data profile packages
	found := false
	for _, c := range caps {
		if c == "sh" {
			found = true
		}
	}
	if !found {
		t.Fatal("sh should be in caps")
	}
}

func TestRestrictedByDefault(t *testing.T) {
	if len(RestrictedByDefault) == 0 {
		t.Fatal("should have restricted packages")
	}
}

func TestAllowsNonRestricted(t *testing.T) {
	// A package not in restricted list should be allowed even with empty caps
	if !Allows(nil, "json") {
		t.Fatal("json should be allowed")
	}
}

func TestFindPackageEntryInSrc(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "src"), 0755)
	os.WriteFile(filepath.Join(dir, "src", "lib.weft"), []byte("fn f { 1 }"), 0644)

	entry, err := FindPackageEntry(dir)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(entry) != "lib.weft" {
		t.Fatalf("src entry = %s", entry)
	}
}
