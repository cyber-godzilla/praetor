package config

import "testing"

func TestValidateNormalizesManagedScriptSourcesWithoutChangingLocalDirectories(t *testing.T) {
	cfg := Defaults()
	cfg.Scripts = []string{"~/scripts", "/srv/shared-scripts"}
	cfg.ManagedScripts = []ManagedScriptSourceConfig{
		{ID: "not-a-uuid", Name: " Community ", URL: " https://example.test/community.git ", Enabled: true, Trust: "", Tracking: "tag"},
		{ID: "not-a-uuid", Name: "", URL: "https://example.test/other.git", Trust: "unexpected", Tracking: "branch"},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(cfg.ManagedScripts) != 2 {
		t.Fatalf("managed scripts = %#v", cfg.ManagedScripts)
	}
	first, second := cfg.ManagedScripts[0], cfg.ManagedScripts[1]
	if !ValidUUID(first.ID) || !ValidUUID(second.ID) || first.ID == second.ID {
		t.Fatalf("managed script IDs were not repaired uniquely: %q %q", first.ID, second.ID)
	}
	if first.Name != "Community" || first.URL != "https://example.test/community.git" {
		t.Fatalf("first source not trimmed: %#v", first)
	}
	if second.Name != "other" {
		t.Fatalf("derived source name = %q, want other", second.Name)
	}
	if first.Trust != ScriptTrustRestricted || second.Trust != ScriptTrustRestricted {
		t.Fatalf("new managed sources must be restricted: %#v", cfg.ManagedScripts)
	}
	if !first.Enabled || second.Enabled {
		t.Fatalf("managed source enabled state changed: %#v", cfg.ManagedScripts)
	}
	if got := cfg.Scripts; len(got) != 2 || got[0] != "~/scripts" || got[1] != "/srv/shared-scripts" {
		t.Fatalf("local directories changed: %#v", got)
	}
}

func TestDefaultsHaveNoManagedScripts(t *testing.T) {
	cfg := Defaults()
	if cfg.ManagedScripts == nil || len(cfg.ManagedScripts) != 0 {
		t.Fatalf("managed scripts default = %#v, want empty non-nil slice", cfg.ManagedScripts)
	}
	if cfg.DisabledScripts == nil || len(cfg.DisabledScripts) != 0 {
		t.Fatalf("disabled local scripts default = %#v, want empty non-nil slice", cfg.DisabledScripts)
	}
}
