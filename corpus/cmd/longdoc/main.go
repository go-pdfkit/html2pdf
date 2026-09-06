// Command longdoc writes a deterministic, self-contained long HTML document —
// headings, prose, tables, code blocks, styled inline runs, inline CSS only,
// no external resource — so a renderer can be timed on pure layout+paint
// with nothing on the network for either side of a comparison.
//
//	go run ./cmd/longdoc -out fixtures/longdoc.html -sections 180
package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"strings"
)

var words = strings.Fields("the protocol semantics field value request response origin server " +
	"client cache intermediary representation resource method status header content " +
	"negotiation connection message framing conformance requirement notation syntax " +
	"length error handling version identifier scheme")

const head = `<!doctype html><html><head><meta charset="utf-8"><title>longdoc bench</title><style>
body{font-family:sans-serif;font-size:14px;line-height:1.5;margin:0;color:#1c2024}
h1{font-size:28px}h2{font-size:20px;margin-top:28px;border-bottom:1px solid #bfc1b7}
p{margin:0 0 10px}table{border-collapse:collapse;width:100%;margin:12px 0;font-size:13px}
td,th{border-bottom:1px solid #d9dad2;padding:6px 8px;text-align:left}th{background:#f5f6f1}
pre{background:#f5f6f1;border:1px solid #d9dad2;padding:8px;font-size:12px;font-family:monospace}
.tag{background:#ffe08a;border:1px solid #b5711a;padding:1px 5px}code{background:#e8eefc;padding:0 3px}
</style></head><body><h1>Long document benchmark fixture</h1>
`

func main() {
	out := flag.String("out", "fixtures/longdoc.html", "output path")
	sections := flag.Int("sections", 180, "number of sections")
	seed := flag.Int64("seed", 9110, "random seed (the fixture is deterministic for a given seed)")
	flag.Parse()

	r := rand.New(rand.NewSource(*seed))
	para := func(n int) string {
		w := make([]string, n)
		for i := range w {
			w[i] = words[r.Intn(len(words))]
		}
		s := strings.Join(w, " ")
		return strings.ToUpper(s[:1]) + s[1:] + "."
	}

	var b strings.Builder
	b.WriteString(head)
	for s := 1; s <= *sections; s++ {
		title := para(3)
		fmt.Fprintf(&b, "<h2>%d. Section %s</h2>\n", s, title[:len(title)-1])
		for i := 0; i < 3; i++ {
			fmt.Fprintf(&b, "<p>%s A <span class='tag'>tag %d</span> and <code>field-%d</code>. %s</p>\n", para(70), s, s, para(30))
		}
		b.WriteString("<table><tr><th>Field</th><th>Value</th><th>Note</th></tr>")
		for row := 0; row < 6; row++ {
			fmt.Fprintf(&b, "<tr><td>field-%d-%d</td><td>%d</td><td>%s</td></tr>", s, row, 1+r.Intn(9999), para(6))
		}
		b.WriteString("</table>\n")
		fmt.Fprintf(&b, "<pre>GET /resource/%d HTTP/1.1\nHost: example.org\nAccept: text/html\n\n%s</pre>\n", s, para(12))
	}
	b.WriteString("</body></html>\n")

	if err := os.WriteFile(*out, []byte(b.String()), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "longdoc: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "wrote %s (%d bytes, %d sections)\n", *out, b.Len(), *sections)
}
