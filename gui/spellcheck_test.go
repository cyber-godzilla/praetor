package main

import "testing"

func TestSpellcheckLanguageFromEnv(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{name: "language preference", env: map[string]string{"LANGUAGE": "fr_CA:en_US"}, want: "fr_CA"},
		{name: "skip C locale", env: map[string]string{"LC_ALL": "C.UTF-8", "LANG": "en_US.UTF-8"}, want: "en_US"},
		{name: "hyphenated locale", env: map[string]string{"LC_MESSAGES": "de-DE.UTF-8"}, want: "de_DE"},
		{name: "script locale", env: map[string]string{"LANG": "zh-hant-tw"}, want: "zh_Hant_TW"},
		{name: "invalid locale", env: map[string]string{"LANG": "not a locale"}, want: defaultSpellcheckLanguage},
		{name: "empty environment", env: map[string]string{}, want: defaultSpellcheckLanguage},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			getenv := func(name string) string { return test.env[name] }
			if got := spellcheckLanguageFromEnv(getenv); got != test.want {
				t.Fatalf("spellcheckLanguageFromEnv() = %q, want %q", got, test.want)
			}
		})
	}
}
