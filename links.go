// Copyright (c) the go-pdfkit/html2pdf authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package html2pdf

import (
	"net/url"
	"strings"

	"github.com/go-webengine/engine/dom"
	"github.com/go-webengine/engine/layout"
)

// linkRun is one clickable rectangle: the atoms of one <a href> anchor on
// one line, in viewport px. A link wrapped over three lines is three runs,
// so the clickable area follows the text rather than covering the block
// between the lines the way a single bounding box would.
type linkRun struct {
	x, y, w, h float64
	uri        string // external target (http/https); empty for an in-document link
	dest       string // in-document target: the id of the element it points to
}

// anchorTarget is where an <a href> resolves to, or nothing worth linking.
type anchorTarget struct {
	uri, dest string
	ok        bool
}

// resolveAnchor turns a raw href into a target. A fragment ("#intro"), or a
// URL that is the document's own with a fragment, becomes an in-document
// jump when an element with that id exists; an http(s) URL becomes a URI
// link, resolved against base; anything else (javascript:, mailto:, tel:,
// data:, a fragment nobody anchors) is dropped — a link that goes nowhere is
// worse than plain text.
func resolveAnchor(base *url.URL, raw string, ids map[string]struct{}) anchorTarget {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return anchorTarget{}
	}
	if strings.HasPrefix(raw, "#") {
		if _, ok := ids[raw[1:]]; ok {
			return anchorTarget{dest: raw[1:], ok: true}
		}
		return anchorTarget{}
	}
	ref, err := url.Parse(raw)
	if err != nil {
		return anchorTarget{}
	}
	abs := ref
	if base != nil {
		abs = base.ResolveReference(ref)
	}
	if abs.Fragment != "" && base != nil {
		self := *abs
		self.Fragment = ""
		here := *base
		here.Fragment = ""
		if self.String() == here.String() {
			if _, ok := ids[abs.Fragment]; ok {
				return anchorTarget{dest: abs.Fragment, ok: true}
			}
			return anchorTarget{}
		}
	}
	if abs.Scheme != "http" && abs.Scheme != "https" {
		return anchorTarget{}
	}
	return anchorTarget{uri: abs.String(), ok: true}
}

// anchorFor walks up from an inline atom's node to the nearest enclosing
// <a> that carries a non-empty href, or nil.
func anchorFor(n *dom.Node) *dom.Node {
	for ; n != nil; n = n.Parent {
		if n.Type != dom.Element || n.Tag != "a" {
			continue
		}
		if href, ok := n.Attribute("href"); ok && strings.TrimSpace(href) != "" {
			return n
		}
	}
	return nil
}

// itemBox returns an inline atom's painted rectangle: a word is its advance
// by its line height, an image its bitmap; a <br> or an empty atom is nothing.
func itemBox(it *layout.InlineItem) (x, y, w, h float64, ok bool) {
	if it.LineBreak {
		return 0, 0, 0, 0, false
	}
	w, h = it.Width, it.LineHeight
	if it.Image != nil {
		h = it.ImgH
	}
	if w <= 0 || h <= 0 {
		return 0, 0, 0, 0, false
	}
	return it.X, it.Y, w, h, true
}

// collectLinks returns every anchor's clickable runs in document order, one
// per line the anchor's atoms appear on. An anchor that lays out no atom at
// all — a link whose only content is an empty styled block, Hacker News's
// vote arrows being the canonical case (30 of its 228 links) — gets the
// rectangle of the outermost box laid out under it instead, as a browser's
// print does; a box that straddles a page break is clipped to the page it
// starts on (see exporter.addLinks).
func collectLinks(root *layout.Box, base *url.URL, ids map[string]struct{}) []linkRun {
	var out []linkRun
	covered := map[*dom.Node]bool{} // anchors that produced at least one run
	targets := map[*dom.Node]anchorTarget{}
	target := func(a *dom.Node) anchorTarget {
		if t, ok := targets[a]; ok {
			return t
		}
		raw, _ := a.Attribute("href")
		t := resolveAnchor(base, raw, ids)
		targets[a] = t
		return t
	}
	var walk func(b *layout.Box)
	walk = func(b *layout.Box) {
		if b == nil {
			return
		}
		for _, line := range b.Lines {
			var cur *dom.Node
			var run linkRun
			flush := func() {
				if cur != nil {
					if t := target(cur); t.ok {
						run.uri, run.dest = t.uri, t.dest
						out = append(out, run)
					}
					covered[cur] = true
				}
				cur = nil
			}
			for _, it := range line.Items {
				x, y, w, h, ok := itemBox(it)
				if !ok {
					continue
				}
				a := anchorFor(it.Node)
				if a != cur {
					flush()
					if a != nil {
						cur, run = a, linkRun{x: x, y: y, w: w, h: h}
					}
					continue
				}
				if a == nil {
					continue
				}
				// Same anchor, same line: grow the run to cover this atom too.
				x1, y1 := run.x+run.w, run.y+run.h
				if x < run.x {
					run.x = x
				}
				if y < run.y {
					run.y = y
				}
				if x+w > x1 {
					x1 = x + w
				}
				if y+h > y1 {
					y1 = y + h
				}
				run.w, run.h = x1-run.x, y1-run.y
			}
			flush()
		}
		for _, c := range b.Children {
			walk(c)
		}
	}
	walk(root)

	// Fallback for the atom-less anchors: pre-order, so the outermost box
	// under an anchor is the one taken and its descendants are then skipped.
	var boxes func(b *layout.Box)
	boxes = func(b *layout.Box) {
		if b == nil {
			return
		}
		if b.Node != nil && b.W > 0 && b.H > 0 {
			if a := anchorFor(b.Node); a != nil && !covered[a] {
				covered[a] = true
				if t := target(a); t.ok {
					out = append(out, linkRun{x: b.X, y: b.Y, w: b.W, h: b.H, uri: t.uri, dest: t.dest})
				}
			}
		}
		for _, c := range b.Children {
			boxes(c)
		}
	}
	boxes(root)
	return out
}

// anchorPoint is where an id'd element sits, in viewport px: the top-left a
// viewer scrolls to for an in-document link.
type anchorPoint struct{ x, y float64 }

// collectIDs maps every element id (and legacy <a name>) that has a
// position to that position: a block element's box top-left, or for an
// inline element — which has no box of its own — the first atom it
// produced. Walked in document order, so the first occurrence of a
// duplicated id wins, as in a browser.
func collectIDs(root *layout.Box) map[string]anchorPoint {
	ids := map[string]anchorPoint{}
	record := func(n *dom.Node, x, y float64) {
		if n == nil || n.Type != dom.Element {
			return
		}
		if id := n.ID(); id != "" {
			if _, seen := ids[id]; !seen {
				ids[id] = anchorPoint{x, y}
			}
		}
		if n.Tag == "a" {
			if name, ok := n.Attribute("name"); ok && name != "" {
				if _, seen := ids[name]; !seen {
					ids[name] = anchorPoint{x, y}
				}
			}
		}
	}
	var walk func(b *layout.Box)
	walk = func(b *layout.Box) {
		if b == nil {
			return
		}
		record(b.Node, b.X, b.Y)
		for _, line := range b.Lines {
			for _, it := range line.Items {
				x, y, _, _, ok := itemBox(it)
				if !ok {
					continue
				}
				for n := it.Node; n != nil; n = n.Parent {
					record(n, x, y)
				}
			}
		}
		for _, c := range b.Children {
			walk(c)
		}
	}
	walk(root)
	return ids
}

// heading is one <h1>…<h6> with its text and top, for the document outline.
type heading struct {
	level int
	title string
	y     float64
}

// collectHeadings returns the headings in document order. A heading's title
// is the text of every atom inside its box, words joined by single spaces;
// one with no text (an icon-only heading) is skipped, since a bookmark with
// no label helps nobody.
func collectHeadings(root *layout.Box) []heading {
	var out []heading
	var walk func(b *layout.Box)
	walk = func(b *layout.Box) {
		if b == nil {
			return
		}
		if b.Node != nil && b.Node.Type == dom.Element && len(b.Node.Tag) == 2 && b.Node.Tag[0] == 'h' &&
			b.Node.Tag[1] >= '1' && b.Node.Tag[1] <= '6' {
			if title := strings.Join(boxWords(b), " "); title != "" {
				out = append(out, heading{level: int(b.Node.Tag[1] - '0'), title: title, y: b.Y})
			}
			return
		}
		for _, c := range b.Children {
			walk(c)
		}
	}
	walk(root)
	return out
}

// boxWords returns the non-empty atom texts under a box, in order.
func boxWords(b *layout.Box) []string {
	var words []string
	var walk func(b *layout.Box)
	walk = func(b *layout.Box) {
		if b == nil {
			return
		}
		for _, line := range b.Lines {
			for _, it := range line.Items {
				if t := strings.TrimSpace(it.Text); t != "" {
					words = append(words, t)
				}
			}
		}
		for _, c := range b.Children {
			walk(c)
		}
	}
	walk(b)
	return words
}

// pageIndexOf returns the 0-based page whose slice [tops[i], tops[i+1])
// holds y.
func pageIndexOf(y float64, tops []float64) int {
	idx := 0
	for i, t := range tops {
		if y >= t {
			idx = i
		}
	}
	return idx
}
