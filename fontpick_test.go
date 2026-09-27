// Copyright (c) the go-pdfkit/html2pdf authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package html2pdf

import (
	"bytes"
	"testing"

	"github.com/go-webengine/engine/css"
)

// A page that NAMES its typefaces must still get the right generic bucket for
// each. css.FontFamily used to be an enum of three values, so comparing it was
// comparing the bucket; since it became {Names, Generic} a declaration like
// `Spectral, Georgia, serif` matches none of the bare buckets, and every serif
// and monospace run on the page was being embedded in the sans face.
func TestPickUsesTheGenericBucketNotTheWholeFamily(t *testing.T) {
	fs, err := loadFonts()
	if err != nil {
		t.Fatalf("loadFonts: %v", err)
	}
	named := func(names string, g css.Generic) css.FontFamily {
		return css.FontFamily{Names: names, Generic: g}
	}
	if got := fs.pick(named("spectral,georgia", css.GenericSerif), false, false); got != fs.serif {
		t.Error("a named serif family must still pick the serif face")
	}
	if got := fs.pick(named("ibm plex mono,menlo", css.GenericMono), false, false); got != fs.mono {
		t.Error("a named monospace family must still pick the mono face")
	}
	if got := fs.pick(named("ibm plex sans", css.GenericSans), true, true); got != fs.sansBI {
		t.Error("a named sans family must still pick the bold-italic sans face")
	}
	// The bare buckets keep working, and the styles still resolve.
	if fs.pick(css.Serif, true, false) != fs.serifB || fs.pick(css.Serif, false, true) != fs.serifI {
		t.Error("the bare serif bucket lost its styles")
	}
	if fs.pick(css.Mono, true, true) != fs.mono {
		t.Error("mono has one face for every style")
	}
}

// End to end: a document naming a serif and a monospace family must embed
// three families, not one.
func TestExportEmbedsAllThreeBucketsForNamedFamilies(t *testing.T) {
	pdf := exportBytes(t, `<html><body>
<p style="font-family: Spectral, Georgia, serif">serif text</p>
<p style="font-family: 'IBM Plex Mono', Menlo, monospace">mono text</p>
<p style="font-family: 'IBM Plex Sans', Helvetica, sans-serif">sans text</p>
</body></html>`, Options{})
	for _, want := range []string{"Lora", "GoMono", "Inter"} {
		if !bytes.Contains(pdf, []byte(want)) {
			t.Errorf("the file names no %s face — a named family took the wrong bucket", want)
		}
	}
}
