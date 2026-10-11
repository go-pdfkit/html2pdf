// Copyright (c) the go-pdfkit/html2pdf authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package main

import (
	"context"
	"os/exec"
	"sort"
	"strings"
	"unicode"

	"github.com/go-webengine/engine"
	"github.com/go-webengine/engine/css"
	"github.com/go-webengine/engine/dom"
)

// gluedWords counts the words of the PDF's extracted text that are two of
// the page's words run together — "PuissanceIT" where the page says
// "Puissance IT" — the signature of a space lost in layout: a table column
// narrower than its longest word, a measured advance shorter than the drawn
// one. Both defects were found on a real report before this census existed,
// each by hand; it names them in one run, and a raster comparison never
// would, since the raster draws the same overlap.
//
// Two of the page's words form a candidate pair when whitespace separates
// them in the source, or when they belong to different block-level boxes (a
// table cell's last word and the next cell's first). An inline boundary
// with no whitespace — `<b>a</b>b`, `kW<sup>‡</sup>` — is not a pair: the
// PDF is right to join those. Returns the count and the glued words.
func gluedWords(e *engine.Engine, doc *engine.Document, pdf string) (int, string) {
	media := css.Media{Type: css.Print, Width: 1024}
	sheets := e.LoadStylesheets(context.Background(), doc, media)
	sm := css.CascadeMedia(doc.Root, media, sheets)

	words := map[string]bool{}
	pairs := map[string]bool{}
	// joins are the pairs the page itself runs together somewhere — an
	// inline boundary with no whitespace. A word that is such a join is
	// never counted, even if the same two words also stand apart elsewhere
	// (RFC 9110 has "obs-text )" in its ABNF and "obs-text</a>)" in prose:
	// the PDF's "obs-text)" is the prose, correctly joined).
	joins := map[string]bool{}
	var prevWord string
	var prevBlock *dom.Node
	prevEndsSpace := false

	var walk func(n *dom.Node, block *dom.Node, hidden bool)
	walk = func(n *dom.Node, block *dom.Node, hidden bool) {
		if n.Type == dom.Element {
			st := sm[n]
			if st != nil && st.Display == css.DisplayNone {
				hidden = true
			}
			if n.Tag == "script" || n.Tag == "style" || n.Tag == "noscript" || n.Tag == "template" {
				return
			}
			if st == nil || st.Display != css.DisplayInline {
				block = n
			}
		}
		if n.Type == dom.Text && !hidden {
			text := normalizeSpaces(n.Text)
			toks := strings.Fields(text)
			if len(toks) > 0 {
				startsSpace := unicode.IsSpace([]rune(text)[0])
				if prevWord != "" {
					if prevEndsSpace || startsSpace || block != prevBlock {
						pairs[prevWord+toks[0]] = true
					} else {
						joins[prevWord+toks[0]] = true
					}
				}
				for i, t := range toks {
					words[t] = true
					if i > 0 {
						pairs[toks[i-1]+t] = true
					}
				}
				prevWord = toks[len(toks)-1]
				prevBlock = block
				prevEndsSpace = unicode.IsSpace([]rune(text)[len([]rune(text))-1])
			}
		}
		for _, c := range n.Children {
			walk(c, block, hidden)
		}
	}
	walk(doc.Root, doc.Root, false)

	out, err := exec.Command("pdftotext", "-enc", "UTF-8", pdf, "-").Output()
	if err != nil {
		return -1, ""
	}
	seen := map[string]bool{}
	var glued []string
	for _, t := range strings.Fields(normalizeSpaces(string(out))) {
		if !words[t] && pairs[t] && !joins[t] && !seen[t] {
			seen[t] = true
			glued = append(glued, t)
		}
	}
	sort.Strings(glued)
	return len(glued), strings.Join(glued, " ")
}

// normalizeSpaces turns the no-break spaces (U+00A0, U+202F, U+2007) into
// plain spaces so a word split on them the way a browser shows it is one
// word here too, and the PDF's extracted text — which carries them as
// written — tokenises the same way.
func normalizeSpaces(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case ' ', ' ', ' ':
			return ' '
		}
		return r
	}, s)
}
