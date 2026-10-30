package scriptmanager

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/cyber-godzilla/praetor/internal/config"
)

type fakeGit struct {
	info        RemoteInfo
	inspectErr  error
	checkoutErr error
	commit      string
	contents    string
	checkouts   []CheckoutRequest
}

func (f *fakeGit) Inspect(context.Context, string) (RemoteInfo, error) {
	return f.info, f.inspectErr
}

func (f *fakeGit) Checkout(_ context.Context, req CheckoutRequest) (string, error) {
	f.checkouts = append(f.checkouts, req)
	if f.checkoutErr != nil {
		return "", f.checkoutErr
	}
	contents := f.contents
	if contents == "" {
		contents = "return {}\n"
	}
	if err := os.WriteFile(filepath.Join(req.Destination, "mode.lua"), []byte(contents), 0o644); err != nil {
		return "", err
	}
	if f.commit != "" {
		return f.commit, nil
	}
	return "0123456789abcdef", nil
}

func TestValidateRepositoryURL(t *testing.T) {
	for _, raw := range []string{
		"http://example.test/scripts.git",
		"https://user:secret@example.test/scripts.git",
		"ssh://git@example.test/scripts.git",
		"file:///tmp/scripts",
		"https://example.test",
	} {
		if _, err := ValidateRepositoryURL(raw); err == nil {
			t.Errorf("ValidateRepositoryURL(%q) succeeded", raw)
		}
	}
	got, err := ValidateRepositoryURL(" HTTPS://Example.TEST/team/scripts.git ")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://example.test/team/scripts.git" {
		t.Fatalf("canonical URL = %q", got)
	}
}

func TestSelectRevisionPrefersHighestStableSemverTag(t *testing.T) {
	info := RemoteInfo{
		Tags: map[string]string{
			"v0.2.9":      "old",
			"v0.10.0":     "new",
			"v1.0.0-beta": "prerelease",
			"nightly":     "nightly",
		},
		DefaultBranch: "main",
		Branches:      map[string]string{"main": "head"},
	}
	got, err := SelectRevision(info)
	if err != nil {
		t.Fatal(err)
	}
	if got.Tracking != config.ScriptTrackingTag || got.Ref != "v0.10.0" || got.Commit != "new" {
		t.Fatalf("revision = %#v", got)
	}
}

func TestSelectRevisionFallsBackToDefaultBranch(t *testing.T) {
	got, err := SelectRevision(RemoteInfo{
		DefaultBranch: "trunk",
		Branches:      map[string]string{"trunk": "head"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Tracking != config.ScriptTrackingBranch || got.Ref != "trunk" || got.Commit != "head" {
		t.Fatalf("revision = %#v", got)
	}
}

func TestInstallStagesValidatesAndCommitsDisabledRestrictedSource(t *testing.T) {
	backend := &fakeGit{info: RemoteInfo{
		Tags: map[string]string{"v0.2.4": "tag-commit"},
	}}
	root := t.TempDir()
	mgr := NewWithBackend(root, backend)

	source, err := mgr.Install(context.Background(), InstallRequest{
		URL:            SuggestedRepositoryURL,
		Suggested:      true,
		CheckOnStartup: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if source.Enabled || source.Trust != config.ScriptTrustRestricted {
		t.Fatalf("unsafe installed source = %#v", source)
	}
	if source.Name != "praetor-scripts" || source.Tracking != config.ScriptTrackingTag || source.Ref != "v0.2.4" {
		t.Fatalf("installed source = %#v", source)
	}
	if source.InstalledCommit != "0123456789abcdef" || source.InstalledVersion != "v0.2.4" {
		t.Fatalf("installed identity = %#v", source)
	}
	if _, err := os.Stat(filepath.Join(mgr.Path(source.ID), "mode.lua")); err != nil {
		t.Fatalf("installed mode: %v", err)
	}
}

func TestInstallFailureLeavesNoManagedCheckout(t *testing.T) {
	backend := &fakeGit{
		info:        RemoteInfo{DefaultBranch: "main", Branches: map[string]string{"main": "head"}},
		checkoutErr: errors.New("network down"),
	}
	root := t.TempDir()
	mgr := NewWithBackend(root, backend)
	if _, err := mgr.Install(context.Background(), InstallRequest{URL: "https://example.test/scripts.git"}); err == nil {
		t.Fatal("Install succeeded")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("failed install left files: %#v", entries)
	}
}

func TestCheckReportsNewStableTagWithoutInstalling(t *testing.T) {
	backend := &fakeGit{info: RemoteInfo{Tags: map[string]string{"v0.2.4": "old", "v0.3.0": "new"}}}
	mgr := NewWithBackend(t.TempDir(), backend)
	status, err := mgr.Check(context.Background(), config.ManagedScriptSourceConfig{
		URL:              "https://example.test/scripts.git",
		Tracking:         config.ScriptTrackingTag,
		InstalledVersion: "v0.2.4",
		InstalledCommit:  "old",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !status.Available || status.Version != "v0.3.0" || status.Commit != "new" {
		t.Fatalf("status = %#v", status)
	}
	if len(backend.checkouts) != 0 {
		t.Fatal("an update check downloaded code")
	}
}

func TestUpdateSwapsCheckoutAndCommitsIdentity(t *testing.T) {
	backend := &fakeGit{
		info:     RemoteInfo{Tags: map[string]string{"v0.3.0": "new-commit"}},
		commit:   "new-commit",
		contents: "return { version = 'new' }\n",
	}
	mgr := NewWithBackend(t.TempDir(), backend)
	source := config.ManagedScriptSourceConfig{
		ID: config.NewUUID(), Name: "Scripts", URL: "https://example.test/scripts.git",
		Trust: config.ScriptTrustRestricted, Tracking: config.ScriptTrackingTag,
		Ref: "v0.2.4", InstalledVersion: "v0.2.4", InstalledCommit: "old-commit",
	}
	if err := os.MkdirAll(mgr.Path(source.ID), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mgr.Path(source.ID), "mode.lua"), []byte("return { version = 'old' }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var committed config.ManagedScriptSourceConfig
	updated, status, err := mgr.Update(context.Background(), source, func(next config.ManagedScriptSourceConfig) error {
		committed = next
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !status.Available || updated.InstalledVersion != "v0.3.0" || updated.InstalledCommit != "new-commit" {
		t.Fatalf("updated = %#v, status = %#v", updated, status)
	}
	if committed != updated {
		t.Fatalf("committed = %#v, updated = %#v", committed, updated)
	}
	contents, err := os.ReadFile(filepath.Join(mgr.Path(source.ID), "mode.lua"))
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != backend.contents {
		t.Fatalf("checkout contents = %q", contents)
	}
}

func TestUpdateRollsBackCheckoutWhenConfigCommitFails(t *testing.T) {
	backend := &fakeGit{
		info:     RemoteInfo{Tags: map[string]string{"v0.3.0": "new-commit"}},
		commit:   "new-commit",
		contents: "return { version = 'new' }\n",
	}
	mgr := NewWithBackend(t.TempDir(), backend)
	source := config.ManagedScriptSourceConfig{
		ID: config.NewUUID(), URL: "https://example.test/scripts.git",
		Trust: config.ScriptTrustRestricted, Tracking: config.ScriptTrackingTag,
		InstalledVersion: "v0.2.4", InstalledCommit: "old-commit",
	}
	if err := os.MkdirAll(mgr.Path(source.ID), 0o755); err != nil {
		t.Fatal(err)
	}
	const old = "return { version = 'old' }\n"
	if err := os.WriteFile(filepath.Join(mgr.Path(source.ID), "mode.lua"), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("save failed")
	if _, _, err := mgr.Update(context.Background(), source, func(config.ManagedScriptSourceConfig) error { return wantErr }); !errors.Is(err, wantErr) {
		t.Fatalf("Update error = %v", err)
	}
	contents, err := os.ReadFile(filepath.Join(mgr.Path(source.ID), "mode.lua"))
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != old {
		t.Fatalf("rollback contents = %q", contents)
	}
}

func TestRemoveRequiresDisabledValidSource(t *testing.T) {
	mgr := NewWithBackend(t.TempDir(), &fakeGit{})
	source := config.ManagedScriptSourceConfig{ID: config.NewUUID()}
	if err := os.MkdirAll(mgr.Path(source.ID), 0o755); err != nil {
		t.Fatal(err)
	}
	source.Enabled = true
	if err := mgr.Remove(source); err == nil {
		t.Fatal("Remove enabled source succeeded")
	}
	source.Enabled = false
	if err := mgr.Remove(source); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(mgr.Path(source.ID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("removed path stat error = %v", err)
	}
}
