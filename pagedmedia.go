// Copyright (c) the go-pdfkit/html2pdf authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package html2pdf

// Paged media: @page margin boxes and target-counter().
//
// Sources, all read from the CSS the page was styled with:
//
//   - CSS Paged Media Module Level 3 (https://www.w3.org/TR/css-page-3/),
//     section 5 (page-margin boxes: @top-left … @bottom-right, their width
//     distribution, default text-align and vertical-align) and section 6.1
//     (the automatic "page" and "pages" counters).
//   - CSS Generated Content Module Level 3
//     (https://www.w3.org/TR/css-content-3/), section 2.6.1 target-counter()
//     — GCPM 3 defers cross-references to it — and section 2.5 leader().
//
// The engine does not model either (its @page reader skips margin boxes, its
// ::before/::after content accepts only a plain string), so both live here,
// where the pagination is known: margin boxes are painted per page by the
// exporter, and target-counter() is resolved in a second layout pass — the
// pages of every id are measured, then each reference is handed to the engine
// as an ordinary string-content rule (see targetCounterSheet).
//
// Supported: the six top and bottom boxes; content made of strings,
// counter(page) and counter(pages); font-size, font-family, font-weight,
// font-style, color and text-align on a box; the unqualified @page rule (and
// the ones inside an applying @media / @layer). Not supported: the corner and
// the side boxes, :first/:left/:right/named page rules, counter styles other
// than decimal, target-counters() and target-text(), and leader() — accepted
// and ignored, so a right-aligned number is what prints.

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/go-pdfkit/pdfkit"
	"github.com/go-webengine/engine/css"
	"github.com/go-webengine/engine/dom"
)

// maxPassCount bounds the layout passes target-counter() may need: a number's
// width can move a line break and so a page, which can change a number.
const maxPassCount = 4

// maxSheetBytes bounds the rules target-counter() generates (one per rule,
// selector and target): a hostile stylesheet cannot make the sheet grow
// without limit.
const maxSheetBytes = 8 << 20

// ---- a small CSS block scanner -------------------------------------------

// stripCSSComments removes /* … */ comments outside strings.
func stripCSSComments(s string) string {
	if !strings.Contains(s, "/*") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '"' || c == '\'':
			j := skipString(s, i)
			b.WriteString(s[i:j])
			i = j - 1
		case c == '/' && i+1 < len(s) && s[i+1] == '*':
			end := strings.Index(s[i+2:], "*/")
			if end < 0 {
				return b.String()
			}
			i += end + 3
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// skipString returns the index after the string literal starting at s[i].
func skipString(s string, i int) int {
	q := s[i]
	for j := i + 1; j < len(s); j++ {
		switch s[j] {
		case '\\':
			j++
		case q:
			return j + 1
		case '\n':
			return j
		}
	}
	return len(s)
}

// matchBrace returns the index of the '}' closing the '{' at s[open].
func matchBrace(s string, open int) (int, bool) {
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '"', '\'':
			i = skipString(s, i) - 1
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i, true
			}
		}
	}
	return 0, false
}

type cssBlock struct{ prelude, body string }

// topBlocks splits src (comments already stripped) into its top-level
// "prelude { body }" blocks. A statement ending in ';' (an @import) before a
// block is dropped from that block's prelude.
func topBlocks(src string) []cssBlock {
	var out []cssBlock
	i := 0
	for i < len(src) {
		brace := -1
		for j := i; j < len(src); j++ {
			if c := src[j]; c == '"' || c == '\'' {
				j = skipString(src, j) - 1
			} else if c == '{' {
				brace = j
				break
			}
		}
		if brace < 0 {
			break
		}
		end, ok := matchBrace(src, brace)
		if !ok {
			break
		}
		prelude := strings.TrimSpace(src[i:brace])
		if semi := strings.LastIndexByte(prelude, ';'); semi >= 0 {
			prelude = strings.TrimSpace(prelude[semi+1:])
		}
		out = append(out, cssBlock{prelude, src[brace+1 : end]})
		i = end + 1
	}
	return out
}

// walkRules calls fn for every style rule and @page rule of the sources,
// descending into the @media blocks that apply to m and into @layer blocks.
func walkRules(sources []string, m css.Media, fn func(prelude, body string)) {
	var walk func(src string, depth int)
	walk = func(src string, depth int) {
		for _, b := range topBlocks(src) {
			lower := strings.ToLower(b.prelude)
			switch {
			case strings.HasPrefix(lower, "@media"):
				if depth < 8 && css.MediaAppliesTo(strings.TrimSpace(b.prelude[len("@media"):]), m) {
					walk(b.body, depth+1)
				}
			case strings.HasPrefix(lower, "@layer"):
				if depth < 8 {
					walk(b.body, depth+1)
				}
			default:
				fn(b.prelude, b.body)
			}
		}
	}
	for _, s := range sources {
		walk(stripCSSComments(s), 0)
	}
}

// ---- content values -------------------------------------------------------

type partKind uint8

const (
	partLit    partKind = iota // a string
	partPage                   // counter(page)
	partPages                  // counter(pages)
	partTarget                 // target-counter(attr(NAME url), page)
)

type contentPart struct {
	kind partKind
	lit  string // partLit: the text
	attr string // partTarget: the attribute that holds the "#id"
}

// parseContent reads a `content` value into its parts. leader() and the
// functions it does not know are skipped; none/normal give no parts.
func parseContent(v string) []contentPart {
	var parts []contentPart
	i := 0
	for i < len(v) {
		c := v[i]
		switch {
		case c == '"' || c == '\'':
			end := skipString(v, i)
			lit := v[i+1 : end]
			if end > i+1 && end-1 < len(v) && v[end-1] == c {
				lit = v[i+1 : end-1]
			}
			parts = append(parts, contentPart{kind: partLit, lit: unescapeCSS(lit)})
			i = end
		case isIdentStart(c):
			j := i
			for j < len(v) && (isIdentStart(v[j]) || v[j] >= '0' && v[j] <= '9') {
				j++
			}
			name := strings.ToLower(v[i:j])
			if j < len(v) && v[j] == '(' {
				end := matchParen(v, j)
				arg := v[j+1 : end]
				parts = appendFunc(parts, name, arg)
				i = min(end+1, len(v))
			} else {
				i = j
			}
		default:
			i++
		}
	}
	return parts
}

func isIdentStart(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '-' || c == '_'
}

// matchParen returns the index of the ')' closing the '(' at s[open], or the
// end of s.
func matchParen(s string, open int) int {
	depth := 0
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '"', '\'':
			i = skipString(s, i) - 1
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return len(s)
}

func appendFunc(parts []contentPart, name, arg string) []contentPart {
	switch name {
	case "counter":
		switch strings.ToLower(strings.TrimSpace(strings.SplitN(arg, ",", 2)[0])) {
		case "page":
			return append(parts, contentPart{kind: partPage})
		case "pages":
			return append(parts, contentPart{kind: partPages})
		}
	case "target-counter":
		// target-counter( attr(NAME url), page ) — CSS Content 3 §2.6.1.
		first, rest, ok := strings.Cut(arg, ",")
		first = strings.TrimSpace(first)
		if !ok || !strings.HasPrefix(strings.ToLower(first), "attr(") || !strings.HasSuffix(first, ")") {
			return parts
		}
		if strings.ToLower(strings.TrimSpace(strings.SplitN(rest, ",", 2)[0])) != "page" {
			return parts
		}
		fields := strings.Fields(first[len("attr(") : len(first)-1])
		if len(fields) == 0 {
			return parts
		}
		return append(parts, contentPart{kind: partTarget, attr: strings.ToLower(fields[0])})
	}
	return parts
}

// unescapeCSS decodes CSS escapes in a string body (CSS Syntax 3 §4.3.7).
func unescapeCSS(s string) string {
	if !strings.Contains(s, "\\") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		if s[i] == '\n' {
			continue
		}
		j := i
		for j < len(s) && j < i+6 && isHex(s[j]) {
			j++
		}
		if j == i {
			b.WriteByte(s[i]) // an escaped ordinary character
			continue
		}
		n, _ := strconv.ParseUint(s[i:j], 16, 32)
		r := rune(n)
		if r == 0 || !utf8.ValidRune(r) {
			r = utf8.RuneError
		}
		b.WriteRune(r)
		if j < len(s) && (s[j] == ' ' || s[j] == '\n' || s[j] == '\t') {
			j++
		}
		i = j - 1
	}
	return b.String()
}

func isHex(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

// ---- margin boxes ---------------------------------------------------------

var marginBoxNames = []string{"top-left", "top-center", "top-right", "bottom-left", "bottom-center", "bottom-right"}

// pageBoxDecls collects, per margin box name, the declarations of every
// unqualified @page rule in the sources (later rules win per property).
func pageBoxDecls(sources []string, m css.Media) map[string]map[string]string {
	out := map[string]map[string]string{}
	walkRules(sources, m, func(prelude, body string) {
		if strings.ToLower(prelude) != "@page" {
			return
		}
		for _, nb := range topBlocks(body) {
			name := strings.ToLower(nb.prelude)
			if !strings.HasPrefix(name, "@") {
				continue
			}
			name = name[1:]
			known := false
			for _, k := range marginBoxNames {
				known = known || k == name
			}
			if !known {
				continue
			}
			if out[name] == nil {
				out[name] = map[string]string{}
			}
			for _, d := range css.ParseDeclarations(nb.body) {
				out[name][d.Property] = d.Value
			}
		}
	})
	return out
}

// marginBox is one resolved page-margin box.
type marginBox struct {
	parts  []contentPart
	sizePt float64
	color  pdfkit.RGB
	fam    css.FontFamily
	bold   bool
	italic bool
	align  string // left, center, right
}

// buildMarginBoxes resolves the declarations into boxes; a box whose
// computed content is none (absent, normal, none) is not generated (§5.2).
func buildMarginBoxes(decls map[string]map[string]string) map[string]*marginBox {
	out := map[string]*marginBox{}
	for name, d := range decls {
		parts := parseContent(d["content"])
		if len(parts) == 0 {
			continue
		}
		mb := &marginBox{parts: parts, sizePt: 12, fam: css.Sans}
		if v, ok := d["font-size"]; ok {
			if pt, ok := lengthPt(v); ok && pt > 0 {
				mb.sizePt = pt
			}
		}
		if v, ok := d["color"]; ok {
			if c, ok := parseColor(v); ok {
				mb.color = c
			}
		}
		fam := strings.ToLower(d["font-family"])
		switch {
		case strings.Contains(fam, "mono") || strings.Contains(fam, "courier"):
			mb.fam = css.Mono
		case strings.Contains(fam, "serif") && !strings.Contains(fam, "sans") || strings.Contains(fam, "times") || strings.Contains(fam, "georgia"):
			mb.fam = css.Serif
		}
		w := strings.ToLower(strings.TrimSpace(d["font-weight"]))
		if n, err := strconv.Atoi(w); err == nil {
			mb.bold = n >= 600
		} else {
			mb.bold = w == "bold" || w == "bolder"
		}
		mb.italic = strings.Contains(strings.ToLower(d["font-style"]), "italic") || strings.Contains(strings.ToLower(d["font-style"]), "oblique")
		// Default text-align per box: CSS Paged Media 3 §6.2, Table 2.
		switch {
		case strings.HasSuffix(name, "-left"):
			mb.align = "left"
		case strings.HasSuffix(name, "-right"):
			mb.align = "right"
		default:
			mb.align = "center"
		}
		switch a := strings.ToLower(strings.TrimSpace(d["text-align"])); a {
		case "left", "center", "right":
			mb.align = a
		case "start":
			mb.align = "left"
		case "end":
			mb.align = "right"
		}
		out[name] = mb
	}
	return out
}

// text evaluates the content for page n of total.
func (mb *marginBox) text(n, total int) string {
	var b strings.Builder
	for _, p := range mb.parts {
		switch p.kind {
		case partLit:
			b.WriteString(p.lit)
		case partPage:
			b.WriteString(strconv.Itoa(n))
		case partPages:
			b.WriteString(strconv.Itoa(total))
		}
	}
	return b.String()
}

// lengthPt reads a font-size length in points: pt, px, mm, cm, in, pc.
func lengthPt(v string) (float64, bool) {
	v = strings.ToLower(strings.TrimSpace(v))
	for _, u := range []struct {
		suffix string
		pt     float64
	}{{"pt", 1}, {"px", pxToPt}, {"mm", 72 / 25.4}, {"cm", 72 / 2.54}, {"in", 72}, {"pc", 12}} {
		if strings.HasSuffix(v, u.suffix) {
			f, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(v, u.suffix)), 64)
			return f * u.pt, err == nil
		}
	}
	return 0, false
}

var namedColors = map[string]pdfkit.RGB{
	"black": {}, "white": {R: 1, G: 1, B: 1}, "gray": {R: .5, G: .5, B: .5}, "grey": {R: .5, G: .5, B: .5},
	"red": {R: 1}, "blue": {B: 1}, "green": {G: .5}, "silver": {R: .75, G: .75, B: .75},
}

// parseColor reads #rgb, #rrggbb, rgb(r, g, b) and a few names.
func parseColor(v string) (pdfkit.RGB, bool) {
	v = strings.ToLower(strings.TrimSpace(v))
	if c, ok := namedColors[v]; ok {
		return c, true
	}
	switch {
	case strings.HasPrefix(v, "#"):
		h := v[1:]
		if len(h) == 3 {
			h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
		}
		if len(h) != 6 {
			return pdfkit.RGB{}, false
		}
		n, err := strconv.ParseUint(h, 16, 32)
		if err != nil {
			return pdfkit.RGB{}, false
		}
		return pdfkit.RGB{R: float64(n>>16&255) / 255, G: float64(n>>8&255) / 255, B: float64(n&255) / 255}, true
	case strings.HasPrefix(v, "rgb(") && strings.HasSuffix(v, ")"):
		f := strings.FieldsFunc(v[4:len(v)-1], func(r rune) bool { return r == ',' || r == ' ' || r == '/' })
		if len(f) < 3 {
			return pdfkit.RGB{}, false
		}
		var c [3]float64
		for i := range c {
			n, err := strconv.ParseFloat(strings.TrimSuffix(f[i], "%"), 64)
			if err != nil {
				return pdfkit.RGB{}, false
			}
			if strings.HasSuffix(f[i], "%") {
				n *= 2.55
			}
			c[i] = min(max(n, 0), 255) / 255
		}
		return pdfkit.RGB{R: c[0], G: c[1], B: c[2]}, true
	}
	return pdfkit.RGB{}, false
}

// paintMarginBoxes draws the margin boxes of page n of total on e's page.
// The three boxes of a row share the row's width as CSS Paged Media 3 §5.3.2
// has it: the centre box is centred on the content area, the left and right
// boxes take what is left on each side, each aligned by its text-align.
func (e *exporter) paintMarginBoxes(boxes map[string]*marginBox, n, total int) {
	if len(boxes) == 0 {
		return
	}
	x0, x1 := e.marginLeftPt, e.pageWPt-e.marginRightPt
	for _, row := range []string{"top", "bottom"} {
		var mid float64 // vertical middle of the margin band
		if row == "top" {
			mid = e.pageHPt - e.marginTopPt/2
		} else {
			mid = e.marginBottomPt / 2
		}
		left, center, right := boxes[row+"-left"], boxes[row+"-center"], boxes[row+"-right"]
		texts := map[*marginBox]string{}
		widths := map[*marginBox]float64{}
		for _, mb := range []*marginBox{left, center, right} {
			if mb != nil {
				texts[mb] = mb.text(n, total)
				widths[mb] = e.textWidthPt(mb, texts[mb])
			}
		}
		cs, ce := (x0+x1)/2, (x0+x1)/2 // the gap between the left and right slots
		if center != nil {
			cs, ce = (x0+x1)/2-widths[center]/2, (x0+x1)/2+widths[center]/2
		}
		for _, p := range []struct {
			mb         *marginBox
			start, end float64
		}{{left, x0, cs}, {center, cs, ce}, {right, ce, x1}} {
			if p.mb == nil || texts[p.mb] == "" {
				continue
			}
			w := widths[p.mb]
			x := p.start
			switch p.mb.align {
			case "center":
				x = (p.start+p.end)/2 - w/2
			case "right":
				x = p.end - w
			}
			e.drawBoxText(p.mb, texts[p.mb], x, mid-p.mb.sizePt*0.35)
		}
	}
}

func (e *exporter) boxWeight(mb *marginBox) int {
	if mb.bold {
		return 700
	}
	return 400
}

// textWidthPt measures s in the box's face, in PDF points.
func (e *exporter) textWidthPt(mb *marginBox, s string) float64 {
	return e.measure.Measure(s, mb.fam, mb.sizePt/pxToPt, e.boxWeight(mb), mb.italic) * pxToPt
}

// drawBoxText paints s at baseline (x, y), walking the same fallback runs
// paintLine does.
func (e *exporter) drawBoxText(mb *marginBox, s string, x, y float64) {
	e.p.SetFillColor(mb.color)
	for _, run := range e.measure.Runs(s, mb.fam, e.boxWeight(mb), mb.italic) {
		f := e.fonts.pick(mb.fam, mb.bold, mb.italic)
		if run.Fallback {
			f = e.fonts.fallback(mb.bold, mb.italic)
		}
		e.p.SetFont(f, mb.sizePt)
		_ = e.p.Text(x, y, run.Text)
		x += e.measure.Measure(run.Text, mb.fam, mb.sizePt/pxToPt, e.boxWeight(mb), mb.italic) * pxToPt
	}
}

// ---- target-counter() -----------------------------------------------------

// usesTargetCounter reports whether any source mentions target-counter().
func usesTargetCounter(sources []string) bool {
	for _, s := range sources {
		if strings.Contains(s, "target-counter(") {
			return true
		}
	}
	return false
}

// fragmentValues returns the distinct values of attribute attr, among the
// elements of root, that are a "#id" fragment.
func fragmentValues(root *dom.Node, attr string) []string {
	seen := map[string]bool{}
	var out []string
	var walk func(n *dom.Node, depth int)
	walk = func(n *dom.Node, depth int) {
		if n == nil || depth > 4096 {
			return
		}
		if v, ok := n.Attr[attr]; ok && len(v) > 1 && v[0] == '#' && !seen[v] && !strings.ContainsAny(v, "\"\\\n") {
			seen[v] = true
			out = append(out, v)
		}
		for _, c := range n.Children {
			walk(c, depth+1)
		}
	}
	walk(root, 0)
	return out
}

// pseudoIndex returns where the ::before/::after (or :before/:after) of one
// selector starts, or -1.
func pseudoIndex(sel string) int {
	lower := strings.ToLower(sel)
	idx := -1
	for _, p := range []string{"::after", "::before", ":after", ":before"} {
		if i := strings.LastIndex(lower, p); i > idx {
			idx = i
		}
	}
	if idx > 0 && sel[idx-1] == ':' {
		idx-- // the "::" form: the attribute selector goes before both colons
	}
	return idx
}

// targetCounterSheet turns every `content` that uses target-counter() into
// ordinary rules the engine understands. For a rule such as
//
//	a.toc::after { content: leader('.') target-counter(attr(href url), page) }
//
// and each `<a href="#sec">` in the document whose id has a known page, it
// emits `a.toc[href="#sec"]::after { content: "7" }`: the attribute selector
// wins over the original, which the engine drops as unsupported content.
// pageOf says on which page (1-based) an id sits.
func targetCounterSheet(sources []string, m css.Media, root *dom.Node, pageOf func(id string) (int, bool)) string {
	var out strings.Builder
	frags := map[string][]string{} // attribute -> its "#id" values, walked once
	walkRules(sources, m, func(prelude, body string) {
		if strings.HasPrefix(prelude, "@") || !strings.Contains(body, "target-counter(") || out.Len() > maxSheetBytes {
			return
		}
		for _, d := range css.ParseDeclarations(body) {
			if d.Property != "content" || !strings.Contains(d.Value, "target-counter(") {
				continue
			}
			parts := parseContent(d.Value)
			for _, sel := range splitSelectors(prelude) {
				at := pseudoIndex(sel)
				if at < 0 {
					continue
				}
				attr := ""
				for _, p := range parts {
					if p.kind == partTarget {
						attr = p.attr
						break
					}
				}
				if attr == "" {
					continue
				}
				vals, seen := frags[attr]
				if !seen {
					vals = fragmentValues(root, attr)
					frags[attr] = vals
				}
				for _, v := range vals {
					if out.Len() > maxSheetBytes {
						return
					}
					n, ok := pageOf(v[1:])
					if !ok {
						continue
					}
					var lit strings.Builder
					for _, p := range parts {
						switch p.kind {
						case partLit:
							lit.WriteString(p.lit)
						case partTarget:
							lit.WriteString(strconv.Itoa(n))
						}
					}
					out.WriteString(sel[:at] + `[` + attr + `="` + v + `"]` + sel[at:] + `{content:"` + escapeCSSString(lit.String()) + "\"}\n")
				}
			}
		}
	})
	return out.String()
}

// splitSelectors splits a selector list at its top-level commas.
func splitSelectors(prelude string) []string {
	var out []string
	depth, start := 0, 0
	for i := 0; i < len(prelude); i++ {
		switch prelude[i] {
		case '"', '\'':
			i = skipString(prelude, i) - 1
		case '(', '[':
			depth++
		case ')', ']':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(prelude[start:i]))
				start = i + 1
			}
		}
	}
	return append(out, strings.TrimSpace(prelude[start:]))
}

func escapeCSSString(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '"', '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\n':
			b.WriteString(`\A `)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
