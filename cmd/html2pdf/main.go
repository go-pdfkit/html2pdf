// Copyright (c) the go-pdfkit/html2pdf authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

// Command html2pdf renders a static HTML file, or a fetched page, to a vector
// PDF.
//
//	html2pdf --in report.html --out report.pdf
//	html2pdf --url https://example.org/report --out report.pdf
//
// With --url the page is fetched through go-webengine's own client (charset
// decoding, redirects) and its final URL becomes the base for relative image
// sources; with --in, pass --base for the same when the file has relative
// images.
//
// The single-dash spellings of earlier releases (-in, -out, -url, -margin,
// -media, -image-dpi, -base, -timeout) still work: main rewrites them to their
// POSIX form before cobra sees the arguments.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/go-pdfkit/html2pdf"
	"github.com/go-webengine/engine"
	"github.com/spf13/cobra"
)

// osExit allows tests to override os.Exit.
var osExit = os.Exit

// usageError marks a command-line mistake (exit status 2), as opposed to a
// runtime failure (exit status 1).
type usageError struct{ msg string }

func (e *usageError) Error() string { return e.msg }

func main() {
	cmd := RootCmd()
	cmd.SetArgs(legacyArgs(os.Args[1:]))
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "html2pdf:", err)
		var ue *usageError
		if errors.As(err, &ue) {
			osExit(2)
			return
		}
		osExit(1)
	}
}

// legacyNames are the single-dash long flags of releases before the cobra port.
var legacyNames = map[string]bool{
	"in": true, "out": true, "url": true, "margin": true,
	"media": true, "image-dpi": true, "base": true, "timeout": true,
}

// legacyArgs rewrites "-in x" and "-in=x" to "--in x" and "--in=x" for the
// flags in legacyNames. Everything after a "--" is left alone, and so is a
// token that is not exactly a legacy name (so "-i", "-io" and "-m5" keep their
// POSIX shorthand meaning).
func legacyArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for i, a := range args {
		if a == "--" {
			return append(out, args[i:]...)
		}
		if len(a) > 2 && a[0] == '-' && a[1] != '-' {
			name, _, _ := strings.Cut(a[1:], "=")
			if legacyNames[name] {
				a = "-" + a
			}
		}
		out = append(out, a)
	}
	return out
}

// RootCmd returns the top-level cobra command. Tests drive it with SetArgs,
// SetOut and SetErr.
func RootCmd() *cobra.Command {
	var (
		in, url, base, out, media        string
		title, author, subject, keywords string
		marginMm, imageDPI               float64
		timeout                          time.Duration
	)
	cmd := &cobra.Command{
		Use:   "html2pdf",
		Short: "Render an HTML file or a fetched page to a vector PDF",
		Long: `html2pdf renders a static HTML file (--in) or a fetched page (--url) to a
vector PDF with go-webengine and go-pdfkit. Exactly one of --in and --url is
required.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if (in == "") == (url == "") {
				return &usageError{"exactly one of --in or --url is required"}
			}
			var src string
			baseURL := base
			if in != "" {
				b, err := os.ReadFile(in)
				if err != nil {
					return fmt.Errorf("read: %w", err)
				}
				src = string(b)
			} else {
				ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
				defer cancel()
				page, err := engine.New().Fetch(ctx, url)
				if err != nil {
					return fmt.Errorf("fetch: %w", err)
				}
				src, baseURL = page.HTML, page.URL
			}

			doc, err := html2pdf.Export(src, html2pdf.Options{
				MarginMm: marginMm, BaseURL: baseURL, Media: media, ImageDPI: imageDPI,
				Title: title, Author: author, Subject: subject, Keywords: keywords,
			})
			if err != nil {
				return err
			}
			f, err := os.Create(out)
			if err != nil {
				return fmt.Errorf("create: %w", err)
			}
			if err := doc.Write(f); err != nil {
				f.Close()
				return fmt.Errorf("write: %w", err)
			}
			if err := f.Close(); err != nil {
				return fmt.Errorf("write: %w", err)
			}
			return nil
		},
	}
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return &usageError{err.Error()} })
	f := cmd.Flags()
	f.StringVarP(&in, "in", "i", "", "input HTML file (one of --in / --url is required)")
	f.StringVarP(&url, "url", "u", "", "fetch this page and render it (one of --in / --url is required)")
	f.StringVarP(&base, "base", "b", "", "base URL for relative image sources when using --in")
	f.StringVarP(&out, "out", "o", "out.pdf", "output PDF path")
	f.Float64VarP(&marginMm, "margin", "m", 20, "page margin, in millimetres, on all four sides")
	f.StringVar(&media, "media", "print", `CSS medium the page is styled for: "print" (the page's @media print rules apply, its screen-only ones do not) or "screen"`)
	f.Float64Var(&imageDPI, "image-dpi", 0, "cap on embedded bitmap density at painted size (0 = keep every fetched pixel; 150 is a print value)")
	f.DurationVar(&timeout, "timeout", 45*time.Second, "fetch timeout for --url")
	f.StringVar(&title, "title", "", "PDF Title (default: the document's <title>)")
	f.StringVar(&author, "author", "", "PDF Author")
	f.StringVar(&subject, "subject", "", "PDF Subject")
	f.StringVar(&keywords, "keywords", "", "PDF Keywords")
	return cmd
}
