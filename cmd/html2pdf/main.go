// Copyright (c) the go-pdfkit/html2pdf authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

// Command html2pdf renders a static HTML file, or a fetched page, to a vector
// PDF.
//
//	html2pdf -in report.html -out report.pdf
//	html2pdf -url https://example.org/report -out report.pdf
//
// With -url the page is fetched through go-webengine's own client (charset
// decoding, redirects) and its final URL becomes the base for relative image
// sources; with -in, pass -base for the same when the file has relative
// images.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/go-pdfkit/html2pdf"
	"github.com/go-webengine/engine"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

func run(args []string, stderr *os.File) int {
	fs := flag.NewFlagSet("html2pdf", flag.ContinueOnError)
	media := fs.String("media", "print", `CSS medium the page is styled for: "print" (the page's @media print rules apply, its screen-only ones do not) or "screen"`)
	imageDPI := fs.Float64("image-dpi", 0, "cap on embedded bitmap density at painted size (0 = keep every fetched pixel; 150 is a print value)")
	fs.SetOutput(stderr)
	in := fs.String("in", "", "input HTML file (one of -in / -url is required)")
	url := fs.String("url", "", "fetch this page and render it (one of -in / -url is required)")
	base := fs.String("base", "", "base URL for relative image sources when using -in")
	out := fs.String("out", "out.pdf", "output PDF path")
	marginMm := fs.Float64("margin", 20, "page margin, in millimetres, on all four sides")
	timeout := fs.Duration("timeout", 45*time.Second, "fetch timeout for -url")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if (*in == "") == (*url == "") {
		fmt.Fprintln(stderr, "html2pdf: exactly one of -in or -url is required")
		return 2
	}

	var src string
	baseURL := *base
	if *in != "" {
		b, err := os.ReadFile(*in)
		if err != nil {
			fmt.Fprintf(stderr, "html2pdf: read: %v\n", err)
			return 1
		}
		src = string(b)
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), *timeout)
		defer cancel()
		page, err := engine.New().Fetch(ctx, *url)
		if err != nil {
			fmt.Fprintf(stderr, "html2pdf: fetch: %v\n", err)
			return 1
		}
		src, baseURL = page.HTML, page.URL
	}

	doc, err := html2pdf.Export(src, html2pdf.Options{MarginMm: *marginMm, BaseURL: baseURL, Media: *media, ImageDPI: *imageDPI})
	if err != nil {
		fmt.Fprintf(stderr, "html2pdf: %v\n", err)
		return 1
	}

	f, err := os.Create(*out)
	if err != nil {
		fmt.Fprintf(stderr, "html2pdf: create: %v\n", err)
		return 1
	}
	defer f.Close()
	if err := doc.Write(f); err != nil {
		fmt.Fprintf(stderr, "html2pdf: write: %v\n", err)
		return 1
	}
	return 0
}
