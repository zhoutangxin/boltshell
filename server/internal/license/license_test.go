package license_test

import (
	"os"
	"testing"

	"boltshell/internal/license"
)

func TestIsProRequiresDevForConfigFlag(t *testing.T) {
	os.Unsetenv("BOLTSHELL_DEV")
	os.Unsetenv("BOLTSHELL_PRO")
	if license.IsPro(true) {
		t.Fatal("proLicensed config must be ignored without BOLTSHELL_DEV")
	}
}

func TestIsProDevMode(t *testing.T) {
	os.Setenv("BOLTSHELL_DEV", "1")
	defer os.Unsetenv("BOLTSHELL_DEV")
	if !license.IsPro(true) {
		t.Fatal("expected pro in dev mode with config flag")
	}
}
