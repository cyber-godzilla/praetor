// Package notes implements a UI-agnostic freeform-note store: one plaintext
// file per note, format `title\nbody`, with safe slug filenames, atomic writes,
// and recency-ordered listing. GUI-only feature; the terminal client never
// wires this up.
package notes

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Summary is a note's title plus a short preview of its body.
type Summary struct {
	Title   string `json:"title"`
	Preview string `json:"preview"`
}

// Note is a full note: title and freeform body.
type Note struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// Store manages note files in a single directory. Safe for concurrent use.
type Store struct {
	dir string
	mu  sync.Mutex
}

// New returns a Store rooted at dir. The directory is created lazily on write.
func New(dir string) *Store { return &Store{dir: dir} }

type noteFile struct {
	path  string
	title string
	body  string
	mtime time.Time
}

// readAll loads every well-formed note file. A missing directory yields no
// notes (not an error); unreadable or empty-title files are skipped.
func (s *Store) readAll() ([]noteFile, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []noteFile
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".txt") {
			continue // also excludes "<slug>.txt.tmp"
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		path := filepath.Join(s.dir, e.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		title, body := parseNote(string(raw))
		if strings.TrimSpace(title) == "" {
			continue
		}
		out = append(out, noteFile{path: path, title: title, body: body, mtime: info.ModTime()})
	}
	return out, nil
}

// parseNote splits raw file content into title (first line, CRLF-tolerant) and
// body (everything after the first newline).
func parseNote(raw string) (title, body string) {
	if i := strings.IndexByte(raw, '\n'); i >= 0 {
		return strings.TrimRight(raw[:i], "\r"), raw[i+1:]
	}
	return strings.TrimRight(raw, "\r"), ""
}

var wsRun = regexp.MustCompile(`\s+`)

// preview collapses whitespace and returns the first 100 runes of body, adding
// an ellipsis when truncated.
func preview(body string) string {
	collapsed := strings.TrimSpace(wsRun.ReplaceAllString(body, " "))
	if utf8.RuneCountInString(collapsed) <= 100 {
		return collapsed
	}
	return string([]rune(collapsed)[:100]) + "…"
}

// List returns every note's title + preview, most-recently-edited first.
func (s *Store) List() ([]Summary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	files, err := s.readAll()
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mtime.After(files[j].mtime) })
	out := make([]Summary, 0, len(files))
	for _, f := range files {
		out = append(out, Summary{Title: f.title, Preview: preview(f.body)})
	}
	return out, nil
}

// Get finds a note by case-insensitive title. ok is false if none matches.
func (s *Store) Get(title string) (Note, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	files, err := s.readAll()
	if err != nil {
		return Note{}, false, err
	}
	for _, f := range files {
		if strings.EqualFold(f.title, title) {
			return Note{Title: f.title, Body: f.body}, true, nil
		}
	}
	return Note{}, false, nil
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// slug produces a filesystem-safe base name from a title.
func slug(title string) string {
	out := nonSlug.ReplaceAllString(strings.ToLower(title), "-")
	out = strings.Trim(out, "-")
	if len(out) > 60 {
		out = strings.Trim(out[:60], "-")
	}
	if out == "" {
		out = "note"
	}
	return out
}

// Save creates or updates a note. originalTitle is "" for a new note, else the
// title the editor opened with (enables rename). It validates the title,
// enforces case-insensitive uniqueness against *other* notes, writes atomically,
// and removes the old file when a rename changes the slug.
func (s *Store) Save(originalTitle, title, body string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	title = strings.TrimSpace(title)
	if title == "" {
		return fmt.Errorf("note title cannot be empty")
	}
	if strings.ContainsRune(title, '\n') {
		return fmt.Errorf("note title cannot contain a newline")
	}

	files, err := s.readAll()
	if err != nil {
		return err
	}

	// Locate the note being edited, if any.
	var current *noteFile
	if strings.TrimSpace(originalTitle) != "" {
		for i := range files {
			if strings.EqualFold(files[i].title, originalTitle) {
				current = &files[i]
				break
			}
		}
	}

	// Reject a title already held by a *different* note.
	for i := range files {
		if strings.EqualFold(files[i].title, title) {
			if current == nil || files[i].path != current.path {
				return fmt.Errorf("a note titled %q already exists", title)
			}
		}
	}

	keepBase := ""
	if current != nil {
		keepBase = filepath.Base(current.path)
	}
	targetPath := filepath.Join(s.dir, s.uniqueFileName(title, files, keepBase))
	if err := s.atomicWrite(targetPath, title+"\n"+body); err != nil {
		return err
	}
	if current != nil && current.path != targetPath {
		// Best-effort: the new file is already written, so a failed removal of
		// the old file is non-fatal — but log it, since it leaves a stale
		// duplicate note behind.
		if err := os.Remove(current.path); err != nil {
			log.Printf("[NOTES] removing old note file after rename: %v", err)
		}
	}
	return nil
}

// Delete removes the note with the given case-insensitive title.
func (s *Store) Delete(title string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	files, err := s.readAll()
	if err != nil {
		return false, err
	}
	for _, f := range files {
		if strings.EqualFold(f.title, title) {
			if err := os.Remove(f.path); err != nil {
				return false, err
			}
			return true, nil
		}
	}
	return false, nil
}

// uniqueFileName returns a "<slug>.txt" name, collision-suffixed, excluding the
// note's own current file (keepBase) so an in-place update reuses its name.
func (s *Store) uniqueFileName(title string, files []noteFile, keepBase string) string {
	base := slug(title)
	name := base + ".txt"
	if name == keepBase {
		return name
	}
	taken := make(map[string]bool)
	for _, f := range files {
		if b := filepath.Base(f.path); b != keepBase {
			taken[b] = true
		}
	}
	for i := 2; taken[name]; i++ {
		name = fmt.Sprintf("%s-%d.txt", base, i)
	}
	return name
}

// atomicWrite writes content to path via a temp file + rename.
func (s *Store) atomicWrite(path, content string) error {
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
