// Copyright (c) the go-pdfkit/html2pdf authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/go-webengine/engine"
	"github.com/go-webengine/engine/dom"
)

// gluedOf renders src with html2pdf and runs the census on the result.
func gluedOf(t *testing.T, src string) (int, string) {
	t.Helper()
	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Skip("pdftotext not installed")
	}
	pdf, err := renderWithRecover(src, "")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "page.pdf")
	if err := os.WriteFile(path, pdf, 0o644); err != nil {
		t.Fatal(err)
	}
	root, err := dom.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	return gluedWords(engine.New(), &engine.Document{Root: root, HTML: src}, path)
}

// TestGluedWordsNamesAWordDrawnIntoItsNeighbour: a negative margin pulls
// "IT" back over the end of "Puissance" — the geometry of a table column
// narrower than its word, reproduced without a table — and pdftotext reads
// one word where the page has two.
func TestGluedWordsNamesAWordDrawnIntoItsNeighbour(t *testing.T) {
	n, words := gluedOf(t, `<html><body><p>Puissance <span style="margin-left:-8px">IT</span> seul</p></body></html>`)
	if n != 1 || words != "PuissanceIT" {
		t.Fatalf("glued = %d %q, want 1 \"PuissanceIT\"", n, words)
	}
}

// TestGluedWordsIgnoresJoinsThePageAsks: an inline boundary with no
// whitespace — a superscript dagger, a bold prefix — is one word on the
// page too, so the PDF joining it is right; a no-break space is a space.
func TestGluedWordsIgnoresJoinsThePageAsks(t *testing.T) {
	n, words := gluedOf(t, `<html><body><p>6,0 kW<sup>‡</sup> et <b>ex</b>iste, <i>a</i> b ; 428&nbsp;630 € — Puissance&nbsp;IT.</p></body></html>`)
	if n != 0 {
		t.Fatalf("glued = %d %q, want 0", n, words)
	}
}

// TestGluedWordsForgivesAJoinThePageMakesElsewhere: RFC 9110 writes
// "obs-text )" in an ABNF block and "<a>obs-text</a>)" in prose; the PDF's
// "obs-text)" is the prose, correctly joined, and is not counted even
// though the same two words stand apart elsewhere.
func TestGluedWordsForgivesAJoinThePageMakesElsewhere(t *testing.T) {
	n, words := gluedOf(t, `<html><body><pre>x = ( VCHAR / obs-text )</pre><p>octets (i.e., <a href="#x">obs-text</a>) as</p></body></html>`)
	if n != 0 {
		t.Fatalf("glued = %d %q, want 0", n, words)
	}
}

// TestGluedWordsCrossesBlockBoundaries: the last word of one cell and the
// first of the next are a pair even with no whitespace between the cells in
// the source — that is exactly where a narrow column glues them.
func TestGluedWordsCrossesBlockBoundaries(t *testing.T) {
	// Both cells are laid out correctly here, so nothing is glued; the pair
	// exists all the same (a glued "MémoireCalcul" would be counted).
	n, words := gluedOf(t, `<html><body><table><tr><td>Mémoire</td><td>Calcul</td></tr></table></body></html>`)
	if n != 0 {
		t.Fatalf("glued = %d %q, want 0", n, words)
	}
}
