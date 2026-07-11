package ui

import (
	"testing"

	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

func segmentStyles(t *testing.T, src string) []widget.RichTextStyle {
	t.Helper()
	segs := highlightSegments(src)
	styles := make([]widget.RichTextStyle, 0, len(segs))
	for _, s := range segs {
		ts, ok := s.(*widget.TextSegment)
		if !ok {
			t.Fatalf("unexpected segment type %T", s)
		}
		styles = append(styles, ts.Style)
	}
	return styles
}

func TestHighlightSegmentsClassification(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want widget.RichTextStyle
	}{
		{"keyword", "Function", styleKeyword},
		{"type", "Character", styleType},
		{"string", `"hello"`, styleString},
		{"comment", "// a comment", styleComment},
		{"number", "42", styleNumber},
		{"directive", "#include \"totvs.ch\"", styleDirective},
		{"plain identifier", "cMyVar", stylePlain},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			styles := segmentStyles(t, c.src)
			if len(styles) != 1 {
				t.Fatalf("expected 1 segment for %q, got %d", c.src, len(styles))
			}
			if styles[0].ColorName != c.want.ColorName {
				t.Errorf("%q: got color %q, want %q", c.src, styles[0].ColorName, c.want.ColorName)
			}
		})
	}
}

func TestHighlightSegmentsPreservesText(t *testing.T) {
	src := `Function Test()
    Local cVar := "abc" // note
Return`
	var rebuilt string
	for _, s := range highlightSegments(src) {
		rebuilt += s.(*widget.TextSegment).Text
	}
	if rebuilt != src {
		t.Errorf("highlightSegments lost or altered text:\ngot:  %q\nwant: %q", rebuilt, src)
	}
}

func TestHighlightSegmentsEmpty(t *testing.T) {
	segs := highlightSegments("")
	if len(segs) != 1 {
		t.Fatalf("expected 1 segment for empty input, got %d", len(segs))
	}
	if segs[0].(*widget.TextSegment).Style.ColorName != theme.ColorNameForeground {
		t.Errorf("empty input should use plain style")
	}
}
