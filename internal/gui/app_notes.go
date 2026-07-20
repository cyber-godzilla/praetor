package gui

import (
	"fmt"

	"github.com/cyber-godzilla/praetor/internal/notes"
)

// ListNotes returns every note's title + preview, most-recently-edited first.
func (a *GuiApp) ListNotes() ([]notes.Summary, error) {
	return a.deps.Notes.List()
}

// GetNote returns a note by case-insensitive title, or an error if none exists.
func (a *GuiApp) GetNote(title string) (notes.Note, error) {
	n, ok, err := a.deps.Notes.Get(title)
	if err != nil {
		return notes.Note{}, err
	}
	if !ok {
		return notes.Note{}, fmt.Errorf("no note titled %q", title)
	}
	return n, nil
}

// SaveNote creates or updates a note. originalTitle is "" for a new note.
func (a *GuiApp) SaveNote(originalTitle, title, body string) error {
	return a.deps.Notes.Save(originalTitle, title, body)
}

// DeleteNote removes a note by title, erroring if it does not exist.
func (a *GuiApp) DeleteNote(title string) error {
	ok, err := a.deps.Notes.Delete(title)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("no note titled %q", title)
	}
	return nil
}
