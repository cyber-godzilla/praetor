package gui

import (
	"path/filepath"
	"testing"
)

func TestAppDirUsesExactOverride(t *testing.T) {
	direct := filepath.Join(t.TempDir(), "config")
	t.Setenv("PRAETOR_TEST_CONFIG_DIR", direct)
	t.Setenv("PRAETOR_TEST_XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "ignored"))

	got := appDir(
		"PRAETOR_TEST_CONFIG_DIR",
		"PRAETOR_TEST_XDG_CONFIG_HOME",
		".config",
		"praetor",
	)
	if got != direct {
		t.Fatalf("appDir() = %q, want exact override %q", got, direct)
	}
}

func TestAppDirRetainsXDGParentBehaviorWithoutOverride(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "xdg-config")
	t.Setenv("PRAETOR_TEST_CONFIG_DIR", "")
	t.Setenv("PRAETOR_TEST_XDG_CONFIG_HOME", parent)

	got := appDir(
		"PRAETOR_TEST_CONFIG_DIR",
		"PRAETOR_TEST_XDG_CONFIG_HOME",
		".config",
		"praetor",
	)
	want := filepath.Join(parent, "praetor")
	if got != want {
		t.Fatalf("appDir() = %q, want XDG-derived path %q", got, want)
	}
}
