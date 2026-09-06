package pdfstat

import (
	"bytes"
	"compress/zlib"
	"testing"
)

func TestCountLinksLooksInsideFlatedStreams(t *testing.T) {
	var packed bytes.Buffer
	w := zlib.NewWriter(&packed)
	w.Write([]byte("1 0 obj << /Type /Annot /Subtype /Link >> 2 0 obj << /Subtype/Link >>"))
	w.Close()
	pdf := append([]byte("%PDF-1.5\n<< /Subtype /Link >>\n5 0 obj << /Type /ObjStm >> stream\n"), packed.Bytes()...)
	pdf = append(pdf, []byte("\nendstream\nendobj\n")...)
	if n := CountLinks(pdf); n != 3 {
		t.Fatalf("CountLinks = %d, want 3 (one bare, two packed)", n)
	}
}

// A CRLF before "endstream", and a stream whose data ends in 0x0D with an LF
// ending, must both count — the second is the one that lost 1 000 links.
func TestCountLinksSurvivesCarriageReturnsAroundTheData(t *testing.T) {
	var packed bytes.Buffer
	w := zlib.NewWriter(&packed)
	w.Write([]byte("<< /Subtype /Link >> << /Subtype /Link >>"))
	w.Close()
	data := packed.Bytes()
	crlf := append(append([]byte("stream\n"), data...), []byte("\r\nendstream")...)
	if n := CountLinks(crlf); n != 2 {
		t.Errorf("CRLF ending: %d links, want 2", n)
	}
	// Truncate the trailer by one byte as the old regex did when the data
	// ended in 0x0D: the body must still be counted.
	cut := append(append([]byte("stream\n"), data[:len(data)-1]...), []byte("\nendstream")...)
	if n := CountLinks(cut); n != 2 {
		t.Errorf("truncated trailer: %d links, want 2 (partial body kept)", n)
	}
}
