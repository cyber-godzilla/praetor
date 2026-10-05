//go:build !linux || !cgo

package main

// WebView2 and WKWebView honor the textarea's spellcheck attribute directly.
func enablePlatformSpellcheck(string) {}
