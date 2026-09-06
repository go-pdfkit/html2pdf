package pdfstat

import (
	"bytes"
	"testing"

	"github.com/go-pdfkit/pdfkit"
)

// Three link annotations across two pages count as three whether the
// writer packed them into object streams (PDF 1.5) or wrote them bare —
// the two shapes html2pdf and Chrome produce.
func TestCountLinksThroughObjectStreamsAndClassic(t *testing.T) {
	for _, objstm := range []bool{true, false} {
		doc := pdfkit.New(pdfkit.Options{Compress: true, ObjectStreams: objstm})
		p1 := doc.AddPage(pdfkit.A4)
		p1.AddLink(pdfkit.Rect{X: 10, Y: 10, Width: 50, Height: 10}, "https://example.com/a")
		p1.AddLink(pdfkit.Rect{X: 10, Y: 30, Width: 50, Height: 10}, "https://example.com/b")
		p2 := doc.AddPage(pdfkit.A4)
		p2.AddLink(pdfkit.Rect{X: 10, Y: 10, Width: 50, Height: 10}, "https://example.com/c")
		doc.AddPage(pdfkit.A4) // a page with no annotations
		var buf bytes.Buffer
		if err := doc.Write(&buf); err != nil {
			t.Fatal(err)
		}
		if n := CountLinks(buf.Bytes()); n != 3 {
			t.Errorf("object streams %v: CountLinks = %d, want 3", objstm, n)
		}
	}
	if n := CountLinks([]byte("not a pdf")); n != -1 {
		t.Errorf("garbage: %d, want -1", n)
	}
}
