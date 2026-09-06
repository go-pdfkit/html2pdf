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

// streamRe brackets a stream's data. The end-of-line before "endstream" is
// matched as a bare "\n" on purpose: an optional "\r" there would eat a data
// byte whenever the flate stream happens to end in 0x0D (three of the 105
// streams in one corpus file), truncating the adler32 trailer — zlib then
// reports "unexpected EOF" after having inflated everything. A "\r" that
// really is line ending (a CRLF writer) is trailing garbage zlib ignores.
var streamRe = regexp.MustCompile(`(?s)stream\r?\n(.*?)\nendstream`)

// Inflated returns b followed by the inflated body of every zlib stream in
// it; a stream that does not inflate at all (an image, a font, anything not
// flate) is skipped, and one that fails part-way contributes what it gave.
func Inflated(b []byte) []byte {
	out := append([]byte(nil), b...)
	for _, m := range streamRe.FindAllSubmatch(b, -1) {
		r, err := zlib.NewReader(bytes.NewReader(m[1]))
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(r) // a trailer error still returns the data read
		out = append(out, '\n')
		out = append(out, body...)
		r.Close()
	}
	return out
}

// CountLinks returns the number of /Subtype /Link annotations, packed or not.
func CountLinks(b []byte) int {
	x := Inflated(b)
	return bytes.Count(x, []byte("/Subtype /Link")) + bytes.Count(x, []byte("/Subtype/Link"))
}
