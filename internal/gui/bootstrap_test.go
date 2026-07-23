package gui

import (
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cyber-godzilla/praetor/internal/config"
	"github.com/cyber-godzilla/praetor/internal/session"
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

func TestBootstrapUsesConfiguredEncryptedCredentialStore(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	stateDir := filepath.Join(root, "state")
	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := config.Defaults()
	cfg.Credentials.Backend = session.CredentialBackendEncryptedFile
	cfg.Credentials.EncryptedFile.KeyEnv = "PRAETOR_TEST_CREDENTIALS_KEY"
	if err := config.Save(cfg, filepath.Join(configDir, "config.yaml")); err != nil {
		t.Fatal(err)
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PRAETOR_TEST_CREDENTIALS_KEY", base64.StdEncoding.EncodeToString(key))
	t.Setenv("PRAETOR_CONFIG_DIR", configDir)
	t.Setenv("PRAETOR_STATE_DIR", stateDir)
	t.Setenv("PRAETOR_DATA_DIR", dataDir)

	deps, err := Bootstrap("test", false)
	if err != nil {
		t.Fatal(err)
	}
	defer deps.Close()
	if deps.Creds.Descriptor().Backend != session.CredentialBackendEncryptedFile {
		t.Fatalf("credential backend = %+v", deps.Creds.Descriptor())
	}
	if _, exists := os.LookupEnv("PRAETOR_TEST_CREDENTIALS_KEY"); exists {
		t.Fatal("Bootstrap retained the credential encryption key in the environment")
	}
	if err := deps.Creds.SetAccount("alice", "password"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(stateDir, "credentials", "credentials.enc")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("encrypted credential file: %v", err)
	}
}

func TestBootstrapRejectsEncryptedCredentialStoreWithoutKey(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := config.Defaults()
	cfg.Credentials.Backend = session.CredentialBackendEncryptedFile
	cfg.Credentials.EncryptedFile.KeyEnv = "PRAETOR_MISSING_CREDENTIALS_KEY"
	if err := config.Save(cfg, filepath.Join(configDir, "config.yaml")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PRAETOR_CONFIG_DIR", configDir)
	t.Setenv("PRAETOR_STATE_DIR", filepath.Join(root, "state"))
	t.Setenv("PRAETOR_DATA_DIR", filepath.Join(root, "data"))
	if deps, err := Bootstrap("test", false); err == nil || !strings.Contains(err.Error(), "PRAETOR_MISSING_CREDENTIALS_KEY") {
		if deps != nil {
			deps.Close()
		}
		t.Fatalf("Bootstrap error = %v", err)
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
