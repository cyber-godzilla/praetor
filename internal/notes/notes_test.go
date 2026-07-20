package notes

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func write(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestList_RecencyOrderAndPreview(t *testing.T) {
	dir := t.TempDir()
	older := write(t, dir, "a.txt", "Alpha\nbody one")
	newer := write(t, dir, "b.txt", "Beta\n  multi   line\nbody two")
	// Make "Alpha" older so "Beta" sorts first.
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(older, old, old); err != nil {
		t.Fatal(err)
	}
	_ = newer

	s := New(dir)
	got, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 notes, got %d: %+v", len(got), got)
	}
	if got[0].Title != "Beta" || got[1].Title != "Alpha" {
		t.Errorf("recency order wrong: %+v", got)
	}
	if got[0].Preview != "multi line body two" {
		t.Errorf("preview whitespace not collapsed: %q", got[0].Preview)
	}
}

func TestList_TruncatesPreviewByRunes(t *testing.T) {
	dir := t.TempDir()
	body := ""
	for i := 0; i < 150; i++ {
		body += "é" // multibyte; 150 runes > 100
	}
	write(t, dir, "n.txt", "T\n"+body)
	got, err := New(dir).List()
	if err != nil {
		t.Fatal(err)
	}
	if r := []rune(got[0].Preview); len(r) != 101 || r[100] != '…' {
		t.Errorf("want 100 runes + ellipsis, got %d runes", len(r))
	}
}

func TestList_SkipsEmptyAndMissingDir(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "empty.txt", "")
	write(t, dir, "blank.txt", "   \n")
	write(t, dir, "good.txt", "Good\nx")
	got, err := New(dir).List()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Title != "Good" {
		t.Errorf("want only Good, got %+v", got)
	}
	// A missing directory is simply "no notes", not an error.
	got2, err := New(filepath.Join(dir, "does-not-exist")).List()
	if err != nil || len(got2) != 0 {
		t.Errorf("missing dir: want empty/no-error, got %+v / %v", got2, err)
	}
}

func TestGet_CaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "n.txt", "Combat Notes\nline1\nline2")
	n, ok, err := New(dir).Get("combat notes")
	if err != nil || !ok {
		t.Fatalf("Get: ok=%v err=%v", ok, err)
	}
	if n.Title != "Combat Notes" || n.Body != "line1\nline2" {
		t.Errorf("wrong note: %+v", n)
	}
	if _, ok, _ := New(dir).Get("nope"); ok {
		t.Error("missing title should return ok=false")
	}
}
