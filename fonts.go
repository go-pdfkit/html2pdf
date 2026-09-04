// Copyright (c) the go-pdfkit/html2pdf authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package html2pdf

import (
	"github.com/go-opentype/fonts/gomono"
	"github.com/go-opentype/fonts/inter"
	"github.com/go-opentype/fonts/lora"
	"github.com/go-pdfkit/pdfkit"
	"github.com/go-webengine/engine/css"
)

// fontSet resolves the (family, bold, italic) requests the layout produced to
// loaded pdfkit fonts. Mono has only a regular face, matching paint.Fonts'
// own fallback (see engine's paint/fonts.go): the family ships no bold or
// italic style, so both requests render in the upright regular face.
type fontSet struct {
	sans, sansB, sansI, sansBI     *pdfkit.Font
	serif, serifB, serifI, serifBI *pdfkit.Font
	mono                           *pdfkit.Font
}

// loadFonts embeds the three families go-webengine's own paint package
// bundles, so glyph metrics always match what the layout pass measured
// against.
func loadFonts() (*fontSet, error) {
	fs := &fontSet{}
	for _, pair := range []struct {
		dst **pdfkit.Font
		b   []byte
	}{
		{&fs.sans, inter.TTF}, {&fs.sansB, inter.BoldTTF}, {&fs.sansI, inter.ItalicTTF}, {&fs.sansBI, inter.BoldItalicTTF},
		{&fs.serif, lora.TTF}, {&fs.serifB, lora.BoldTTF}, {&fs.serifI, lora.ItalicTTF}, {&fs.serifBI, lora.BoldItalicTTF},
		{&fs.mono, gomono.TTF},
	} {
		f, err := pdfkit.LoadFont(pair.b)
		if err != nil {
			return nil, err
		}
		*pair.dst = f
	}
	return fs, nil
}

// pick returns the loaded face matching a CSS font-family/weight/style
// request.
func (fs *fontSet) pick(fam css.FontFamily, bold, italic bool) *pdfkit.Font {
	switch fam {
	case css.Serif:
		switch {
		case bold && italic:
			return fs.serifBI
		case bold:
			return fs.serifB
		case italic:
			return fs.serifI
		default:
			return fs.serif
		}
	case css.Mono:
		return fs.mono
	default:
		switch {
		case bold && italic:
			return fs.sansBI
		case bold:
			return fs.sansB
		case italic:
			return fs.sansI
		default:
			return fs.sans
		}
	}
}
