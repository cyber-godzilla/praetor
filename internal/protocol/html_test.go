package protocol

import (
	"testing"
)

func TestParseHTML_PlainText(t *testing.T) {
	result := ParseHTML("Hello world")
	if result.Text != "Hello world" {
		t.Errorf("expected 'Hello world', got %q", result.Text)
	}
	if len(result.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(result.Segments))
	}
	seg := result.Segments[0]
	if seg.Text != "Hello world" {
		t.Errorf("expected segment text 'Hello world', got %q", seg.Text)
	}
	if seg.Bold || seg.Italic || seg.Underline || seg.Color != "" {
		t.Error("expected no styling on plain text segment")
	}
	if result.ClearPage {
		t.Error("expected ClearPage to be false")
	}
}

func TestParseHTML_Bold(t *testing.T) {
	result := ParseHTML("<b>bold text</b>")
	if result.Text != "bold text" {
		t.Errorf("expected 'bold text', got %q", result.Text)
	}
	if len(result.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(result.Segments))
	}
	if !result.Segments[0].Bold {
		t.Error("expected Bold to be true")
	}
}

func TestParseHTML_Italic(t *testing.T) {
	result := ParseHTML("<i>italic text</i>")
	if result.Text != "italic text" {
		t.Errorf("expected 'italic text', got %q", result.Text)
	}
	if len(result.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(result.Segments))
	}
	if !result.Segments[0].Italic {
		t.Error("expected Italic to be true")
	}
}

func TestParseHTML_FontColor(t *testing.T) {
	result := ParseHTML("<font color=\"#FF0000\">red text</font>")
	if result.Text != "red text" {
		t.Errorf("expected 'red text', got %q", result.Text)
	}
	if len(result.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(result.Segments))
	}
	if result.Segments[0].Color != "#FF0000" {
		t.Errorf("expected color '#FF0000', got %q", result.Segments[0].Color)
	}
}

func TestParseHTML_XchCmd_Underline(t *testing.T) {
	result := ParseHTML("<xch_cmd>clickable</xch_cmd>")
	if result.Text != "clickable" {
		t.Errorf("expected 'clickable', got %q", result.Text)
	}
	if len(result.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(result.Segments))
	}
	if !result.Segments[0].Underline {
		t.Error("expected Underline to be true")
	}
}

func TestParseHTML_XchPageClear(t *testing.T) {
	result := ParseHTML("<xch_page clear=\"text\">content</xch_page>")
	if !result.ClearPage {
		t.Error("expected ClearPage to be true")
	}
}

func TestParseHTML_NestedTags(t *testing.T) {
	result := ParseHTML("<b><font color=\"#00FF00\">bold green</font></b>")
	if result.Text != "bold green" {
		t.Errorf("expected 'bold green', got %q", result.Text)
	}
	if len(result.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(result.Segments))
	}
	seg := result.Segments[0]
	if !seg.Bold {
		t.Error("expected Bold to be true")
	}
	if seg.Color != "#00FF00" {
		t.Errorf("expected color '#00FF00', got %q", seg.Color)
	}
}

func TestParseHTML_MixedContent(t *testing.T) {
	result := ParseHTML("normal <b>bold</b> normal")
	if result.Text != "normal bold normal" {
		t.Errorf("expected 'normal bold normal', got %q", result.Text)
	}
	if len(result.Segments) != 3 {
		t.Fatalf("expected 3 segments, got %d: %+v", len(result.Segments), result.Segments)
	}
	if result.Segments[0].Bold {
		t.Error("first segment should not be bold")
	}
	if result.Segments[0].Text != "normal " {
		t.Errorf("first segment text: expected 'normal ', got %q", result.Segments[0].Text)
	}
	if !result.Segments[1].Bold {
		t.Error("second segment should be bold")
	}
	if result.Segments[1].Text != "bold" {
		t.Errorf("second segment text: expected 'bold', got %q", result.Segments[1].Text)
	}
	if result.Segments[2].Bold {
		t.Error("third segment should not be bold")
	}
	if result.Segments[2].Text != " normal" {
		t.Errorf("third segment text: expected ' normal', got %q", result.Segments[2].Text)
	}
}

func TestParseHTML_EmptyInput(t *testing.T) {
	result := ParseHTML("")
	if result.Text != "" {
		t.Errorf("expected empty text, got %q", result.Text)
	}
	if len(result.Segments) != 0 {
		t.Errorf("expected no segments, got %d", len(result.Segments))
	}
	if result.ClearPage {
		t.Error("expected ClearPage to be false")
	}
}

func TestParseHTML_UnknownTagStripped(t *testing.T) {
	result := ParseHTML("<span>text inside span</span>")
	if result.Text != "text inside span" {
		t.Errorf("expected 'text inside span', got %q", result.Text)
	}
}

func TestParseHTML_SelfClosingTag(t *testing.T) {
	result := ParseHTML("line one<br/>line two")
	if result.Text != "line oneline two" {
		t.Errorf("expected 'line oneline two', got %q", result.Text)
	}
}

func TestParseHTML_DeeplyNested(t *testing.T) {
	result := ParseHTML("<b><i><font color=\"#123456\">deep</font></i></b>")
	if result.Text != "deep" {
		t.Errorf("expected 'deep', got %q", result.Text)
	}
	if len(result.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(result.Segments))
	}
	seg := result.Segments[0]
	if !seg.Bold || !seg.Italic || seg.Color != "#123456" {
		t.Errorf("expected bold+italic+color, got bold=%v italic=%v color=%q",
			seg.Bold, seg.Italic, seg.Color)
	}
}

func TestParseHTML_XchPageNoClear(t *testing.T) {
	// xch_page without clear="text" should not set ClearPage
	result := ParseHTML("<xch_page>stuff</xch_page>")
	if result.ClearPage {
		t.Error("expected ClearPage to be false without clear=\"text\"")
	}
}

func TestParseHTML_NamedColor(t *testing.T) {
	result := ParseHTML("<font color=\"darkblue\">dark blue text</font>")
	if len(result.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(result.Segments))
	}
	if result.Segments[0].Color != "#2244aa" {
		t.Errorf("expected color '#2244aa' for darkblue, got %q", result.Segments[0].Color)
	}
}

func TestParseHTML_NamedColorCaseInsensitive(t *testing.T) {
	result := ParseHTML("<font color=\"DarkBlue\">text</font>")
	if len(result.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(result.Segments))
	}
	if result.Segments[0].Color != "#2244aa" {
		t.Errorf("expected color '#2244aa', got %q", result.Segments[0].Color)
	}
}

func TestParseHTML_NamedColorIndigo(t *testing.T) {
	result := ParseHTML("<font color=\"indigo\">indigo text</font>")
	if len(result.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(result.Segments))
	}
	// Indigo (#4B0082) is dark, gets brightened for visibility.
	if result.Segments[0].Color == "" {
		t.Error("indigo should have a color, got empty")
	}
}

func TestParseHTML_NamedColorUnknown(t *testing.T) {
	result := ParseHTML("<font color=\"notacolor\">text</font>")
	if len(result.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(result.Segments))
	}
	// Unknown named color should result in no color set.
	if result.Segments[0].Color != "" {
		t.Errorf("expected no color for unknown name, got %q", result.Segments[0].Color)
	}
}

func TestParseHTML_HexColorStillWorks(t *testing.T) {
	result := ParseHTML("<font color=\"#FF0000\">red text</font>")
	if len(result.Segments) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(result.Segments))
	}
	if result.Segments[0].Color != "#FF0000" {
		t.Errorf("expected color '#FF0000', got %q", result.Segments[0].Color)
	}
}

func TestParseHTML_HTMLEntities(t *testing.T) {
	result := ParseHTML("&lt;hello&gt; &amp; &quot;world&quot;")
	if result.Text != "<hello> & \"world\"" {
		t.Errorf("expected decoded entities, got %q", result.Text)
	}
}
