// Copyright (c) the go-pdfkit/html2pdf authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestRunMissingIn(t *testing.T) {
	if code := run(nil, os.Stderr); code != 2 {
		t.Errorf("run(nil) = %d, want 2", code)
	}
}

func TestRunRejectsBothInAndURL(t *testing.T) {
	if code := run([]string{"-in", "x.html", "-url", "http://127.0.0.1:1/"}, os.Stderr); code != 2 {
		t.Errorf("run(-in and -url) = %d, want 2", code)
	}
}

func TestRunURLFetchesAndRenders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/page" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			// A relative image that 404s: the base URL must resolve it (so the
			// fetch is attempted against this server) and its failure must be
			// silent, as on the raster canvas.
			w.Write([]byte(`<html><body><h1>fetched</h1><img src="missing.png"><p>body text</p></body></html>`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	out := filepath.Join(t.TempDir(), "out.pdf")
	if code := run([]string{"-url", srv.URL + "/page", "-out", out}, os.Stderr); code != 0 {
		t.Fatalf("run(-url) = %d, want 0", code)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if len(data) < 5 || string(data[:5]) != "%PDF-" {
		t.Errorf("output is not a PDF")
	}
}

func TestRunURLFetchFailure(t *testing.T) {
	// A closed port: the fetch fails fast and the exit code says so.
	if code := run([]string{"-url", "http://127.0.0.1:1/nothing", "-out", filepath.Join(t.TempDir(), "o.pdf"), "-timeout", "5s"}, os.Stderr); code != 1 {
		t.Errorf("run(-url unreachable) = %d, want 1", code)
	}
}

func TestRunUnreadableIn(t *testing.T) {
	if code := run([]string{"-in", "/nonexistent/file.html"}, os.Stderr); code != 1 {
		t.Errorf("run(missing file) = %d, want 1", code)
	}
}

func TestRunEndToEnd(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.html")
	out := filepath.Join(dir, "out.pdf")
	if err := os.WriteFile(in, []byte(`<html><body>hello</body></html>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := run([]string{"-in", in, "-out", out, "-margin", "15"}, os.Stderr); code != 0 {
		t.Fatalf("run() = %d, want 0", code)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if len(data) < 4 || string(data[:5]) != "%PDF-" {
		t.Errorf("output is not a PDF")
	}
}

func TestRunBadFlag(t *testing.T) {
	if code := run([]string{"-nosuchflag"}, os.Stderr); code != 2 {
		t.Errorf("run(bad flag) = %d, want 2", code)
	}
}

func TestRunUnwritableOut(t *testing.T) {
	dir := t.TempDir()
	in := filepath.Join(dir, "in.html")
	os.WriteFile(in, []byte(`<html><body>x</body></html>`), 0o644)
	if code := run([]string{"-in", in, "-out", filepath.Join(dir, "nodir", "out.pdf")}, os.Stderr); code != 1 {
		t.Errorf("run(unwritable out) = %d, want 1", code)
	}
}
