// Package pdfstat counts things in a PDF's bytes without a full parser —
// link annotations above all. It looks inside every FlateDecode stream it
// can inflate, so a document written with PDF 1.5 object streams (where the
// annotation dictionaries live packed and compressed) counts the same as a
// classic one.
package pdfstat

import (
	"bytes"
	"compress/zlib"
	"io"
	"regexp"
)

var streamRe = regexp.MustCompile(`(?s)stream\r?\n(.*?)\r?\nendstream`)

// Inflated returns b followed by the inflated body of every zlib stream in
// it; a stream that does not inflate (an image, a font, anything not flate)
// is skipped.
func Inflated(b []byte) []byte {
	out := append([]byte(nil), b...)
	for _, m := range streamRe.FindAllSubmatch(b, -1) {
		r, err := zlib.NewReader(bytes.NewReader(m[1]))
		if err != nil {
			continue
		}
		if body, err := io.ReadAll(r); err == nil {
			out = append(out, '\n')
			out = append(out, body...)
		}
		r.Close()
	}
	return out
}

// CountLinks returns the number of /Subtype /Link annotations, packed or not.
func CountLinks(b []byte) int {
	x := Inflated(b)
	return bytes.Count(x, []byte("/Subtype /Link")) + bytes.Count(x, []byte("/Subtype/Link"))
}
