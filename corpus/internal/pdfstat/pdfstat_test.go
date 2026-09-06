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
