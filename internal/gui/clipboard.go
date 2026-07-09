package gui

// Clipboard abstracts the host clipboard so the Wails-free facade can offer
// copy/paste without importing the Wails runtime. The Wails shell injects an
// implementation through Deps.Clipboard (see gui/main.go).
type Clipboard interface {
	GetText() (string, error)
	SetText(text string) error
}

// ClipboardGet returns the current clipboard text. Returns "" when no clipboard
// backend is wired (e.g. tests, or before startup injects one).
func (a *GuiApp) ClipboardGet() (string, error) {
	if a.deps.Clipboard == nil {
		return "", nil
	}
	return a.deps.Clipboard.GetText()
}

// ClipboardSet writes text to the clipboard. No-op when no backend is wired.
func (a *GuiApp) ClipboardSet(text string) error {
	if a.deps.Clipboard == nil {
		return nil
	}
	return a.deps.Clipboard.SetText(text)
}
