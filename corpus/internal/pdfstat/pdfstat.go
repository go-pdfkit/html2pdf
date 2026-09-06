// Package pdfstat counts things in a PDF — link annotations above all —
// through go-pdfkit/reader, the org's own parser: cross-reference streams,
// object streams and filters are its job, so a document written with PDF
// 1.5 object streams (where the annotation dictionaries live packed and
// compressed) counts the same as a classic one, and a count is a count of
// objects on pages rather than of a byte pattern in the file.
package pdfstat

import "github.com/go-pdfkit/reader"

// CountLinks returns the number of /Subtype /Link annotations reachable
// from the page tree, or -1 when the bytes are not a PDF the reader opens.
func CountLinks(b []byte) int {
	d, err := reader.Open(b)
	if err != nil {
		return -1
	}
	n := 0
	for i := 1; i <= d.PageCount(); i++ { // Page is 1-based
		pg, err := d.Page(i)
		if err != nil {
			continue
		}
		annots, err := d.Resolve(pg.Get("Annots"))
		if err != nil {
			continue
		}
		arr, ok := reader.ToArray(annots)
		if !ok {
			continue
		}
		for _, a := range arr {
			o, err := d.Resolve(a)
			if err != nil {
				continue
			}
			if ad, ok := reader.ToDict(o); ok {
				if st, _ := reader.ToName(ad.Get("Subtype")); st == "Link" {
					n++
				}
			}
		}
	}
	return n
}
