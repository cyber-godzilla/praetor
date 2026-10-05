package main

import (
	"os"
	"strings"
)

const defaultSpellcheckLanguage = "en_US"

// enableSpellcheck turns on the native webview spellchecker where the platform
// requires an explicit opt-in. The textarea's spellcheck attribute remains the
// per-user on/off switch.
func enableSpellcheck() {
	enablePlatformSpellcheck(spellcheckLanguageFromEnv(os.Getenv))
}

func spellcheckLanguageFromEnv(getenv func(string) string) string {
	for _, name := range []string{"LANGUAGE", "LC_ALL", "LC_MESSAGES", "LANG"} {
		for _, candidate := range strings.Split(getenv(name), ":") {
			if language, ok := normalizeSpellcheckLanguage(candidate); ok {
				return language
			}
		}
	}
	return defaultSpellcheckLanguage
}

func normalizeSpellcheckLanguage(locale string) (string, bool) {
	locale = strings.TrimSpace(locale)
	if i := strings.IndexAny(locale, ".@"); i >= 0 {
		locale = locale[:i]
	}
	if locale == "" || strings.EqualFold(locale, "C") || strings.EqualFold(locale, "POSIX") {
		return "", false
	}

	parts := strings.Split(strings.ReplaceAll(locale, "-", "_"), "_")
	if len(parts) > 3 || len(parts[0]) < 2 || len(parts[0]) > 3 || !asciiLetters(parts[0]) {
		return "", false
	}
	parts[0] = strings.ToLower(parts[0])

	for i := 1; i < len(parts); i++ {
		if parts[i] == "" || !asciiLettersOrDigits(parts[i]) {
			return "", false
		}
		switch {
		case len(parts[i]) == 2 && asciiLetters(parts[i]):
			parts[i] = strings.ToUpper(parts[i])
		case len(parts[i]) == 4 && asciiLetters(parts[i]):
			parts[i] = strings.ToUpper(parts[i][:1]) + strings.ToLower(parts[i][1:])
		}
	}

	return strings.Join(parts, "_"), true
}

func asciiLetters(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') {
			return false
		}
	}
	return true
}

func asciiLettersOrDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') {
			return false
		}
	}
	return true
}
