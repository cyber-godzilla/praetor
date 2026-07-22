package kitty

import (
	"image"
	"strings"
	"testing"
)

// TestEncodeIncludesPlacementID asserts that the kitty escape includes a
// placement id (p=N) tied to the image id. Without p=, every a=T emit
// creates a fresh placement at the cursor position even when the image
// id matches an existing image; the previous placements remain and
// stack at the same cell. During typing the View() pipeline fires the
// re-emit at high frequency, which manifests as a ~1px subpixel twitch
// as new placements overlay the old. With p=N matching i=N, re-emits
// replace the existing placement atomically.
func TestEncodeIncludesPlacementID(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	esc := Encode(img, 4, 1, 7)
	if !strings.Contains(esc, "i=7") {
		t.Fatalf("expected i=7 in escape, got: %q", esc)
	}
	if !strings.Contains(esc, "p=7") {
		t.Fatalf("expected p=7 in escape (placement id), got: %q", esc)
	}
}

// TestEncodeOmitsPlacementWhenNoID asserts that when imageID is 0 (let
// kitty auto-assign), neither i= nor p= is emitted — there's no stable
// id to anchor a placement to.
func TestEncodeOmitsPlacementWhenNoID(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	esc := Encode(img, 4, 1, 0)
	if strings.Contains(esc, "i=") {
		t.Fatalf("expected no i= when imageID=0, got: %q", esc)
	}
	if strings.Contains(esc, "p=") {
		t.Fatalf("expected no p= when imageID=0, got: %q", esc)
	}
}

func TestDeleteAll_DeletesAllImages(t *testing.T) {
	got := DeleteAll()
	// a=d (delete), d=A (all images + data), APC-wrapped, q=2 (quiet).
	want := "\033_Ga=d,d=A,q=2;\033\\"
	if got != want {
		t.Errorf("DeleteAll() = %q, want %q", got, want)
	}
}
