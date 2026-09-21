//go:build linux

package main

/*
#cgo !webkit2_41 pkg-config: webkit2gtk-4.0
#cgo webkit2_41 pkg-config: webkit2gtk-4.1
#include <stdlib.h>
#include <webkit2/webkit2.h>

static void praetor_enable_spellcheck(const char *language) {
	WebKitWebContext *context = webkit_web_context_get_default();
	const gchar *languages[] = { language, NULL };
	webkit_web_context_set_spell_checking_languages(context, languages);
	webkit_web_context_set_spell_checking_enabled(context, TRUE);
}
*/
import "C"

import "unsafe"

// WebKitGTK does not honor an element's spellcheck attribute until its shared
// web context has spellchecking enabled and at least one language configured.
func enablePlatformSpellcheck(language string) {
	cLanguage := C.CString(language)
	defer C.free(unsafe.Pointer(cLanguage))
	C.praetor_enable_spellcheck(cLanguage)
}
