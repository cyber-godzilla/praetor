package notes

import (
	"os"
	"path/filepath"
	"strings"
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

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSave_CreateGetRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	if err := s.Save("", "Combat: Round 1", "sweep\nthen retreat"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	n, ok, _ := s.Get("combat: round 1")
	if !ok || n.Title != "Combat: Round 1" || n.Body != "sweep\nthen retreat" {
		t.Fatalf("round trip failed: %+v ok=%v", n, ok)
	}
	// Filename is a safe slug; the colon/space never reach the filesystem.
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || strings.ContainsAny(entries[0].Name(), `:\/`) {
		t.Errorf("unsafe or missing filename: %v", entries)
	}
	if got := read(t, filepath.Join(dir, entries[0].Name())); got != "Combat: Round 1\nsweep\nthen retreat" {
		t.Errorf("file content = %q", got)
	}
}

func TestSave_RejectsEmptyOrNewlineTitle(t *testing.T) {
	s := New(t.TempDir())
	if err := s.Save("", "   ", "x"); err == nil {
		t.Error("empty title should error")
	}
	if err := s.Save("", "a\nb", "x"); err == nil {
		t.Error("newline title should error")
	}
}

func TestSave_RejectsDuplicateTitle(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	_ = s.Save("", "Loot", "a")
	if err := s.Save("", "loot", "b"); err == nil {
		t.Error("duplicate (case-insensitive) title should error")
	}
}

func TestSave_UpdateInPlace(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	_ = s.Save("", "Notes", "v1")
	if err := s.Save("Notes", "Notes", "v2"); err != nil {
		t.Fatalf("update: %v", err)
	}
	n, _, _ := s.Get("Notes")
	if n.Body != "v2" {
		t.Errorf("body = %q, want v2", n.Body)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("update should not create a second file: %v", entries)
	}
}

func TestSave_RenameRemovesOldFile(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	_ = s.Save("", "Old Title", "body")
	if err := s.Save("Old Title", "New Title", "body"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if _, ok, _ := s.Get("Old Title"); ok {
		t.Error("old title should be gone after rename")
	}
	if _, ok, _ := s.Get("New Title"); !ok {
		t.Error("new title should exist after rename")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("rename should leave exactly one file: %v", entries)
	}
}

func TestSave_RenameToOtherExistingTitleRejected(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	_ = s.Save("", "A", "a")
	_ = s.Save("", "B", "b")
	if err := s.Save("A", "B", "a2"); err == nil {
		t.Error("renaming A to an existing title B should error")
	}
}

func TestDelete(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	_ = s.Save("", "Gone", "x")
	ok, err := s.Delete("gone")
	if err != nil || !ok {
		t.Fatalf("delete: ok=%v err=%v", ok, err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("file should be removed: %v", entries)
	}
	if ok, _ := s.Delete("missing"); ok {
		t.Error("deleting a missing note should return ok=false")
	}
}

func TestSave_NoTmpLeftBehind(t *testing.T) {
	dir := t.TempDir()
	_ = New(dir).Save("", "T", "body")
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}

func TestSlug_WindowsReservedNamesPrefixed(t *testing.T) {
	for _, r := range []string{"con", "prn", "aux", "nul", "com1", "com9", "lpt1", "lpt9"} {
		if got := slug(r); got != "note-"+r {
			t.Errorf("slug(%q) = %q, want note-%s (Windows reserved)", r, got, r)
		}
	}
	if got := slug("CON"); got != "note-con" { // slug lowercases first
		t.Errorf("slug(CON) = %q, want note-con", got)
	}
	for _, r := range []string{"com", "com10", "console", "lpt", "con1"} { // lookalikes, not reserved
		if got := slug(r); got != r {
			t.Errorf("slug(%q) = %q, want unchanged", r, got)
		}
	}
}

func TestSave_CollisionSuffix(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	// Two different titles that slug to the same base ("foo").
	if err := s.Save("", "Foo!", "a"); err != nil {
		t.Fatal(err)
	}
	if err := s.Save("", "Foo?", "b"); err != nil {
		t.Fatal(err)
	}
	if n, ok, _ := s.Get("Foo!"); !ok || n.Body != "a" {
		t.Errorf("Foo! = %+v ok=%v", n, ok)
	}
	if n, ok, _ := s.Get("Foo?"); !ok || n.Body != "b" {
		t.Errorf("Foo? = %+v ok=%v", n, ok)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 2 {
		t.Errorf("want 2 files (base + collision suffix), got %v", entries)
	}
}
