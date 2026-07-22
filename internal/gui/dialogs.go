package gui

// Dialogs abstracts native OS dialogs so the Wails-free facade can offer a
// folder picker without importing the Wails runtime (which needs the Wails
// context). The Wails shell injects an implementation through Deps.Dialogs
// (see gui/main.go); tests use a fake.
type Dialogs interface {
	// PickDirectory opens a native folder picker and returns the chosen absolute
	// path, or "" if the user cancelled.
	PickDirectory(title, defaultDir string) (string, error)
}

// PickScriptDir opens a native folder picker for adding a scripts directory and
// returns the chosen absolute path, or "" if cancelled or no backend is wired.
// The returned path is absolute and needs no ~/env expansion.
func (a *GuiApp) PickScriptDir() (string, error) {
	if a.deps.Dialogs == nil {
		return "", nil
	}
	return a.deps.Dialogs.PickDirectory("Select a scripts directory", "")
}
