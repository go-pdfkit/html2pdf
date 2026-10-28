// Copyright (c) the go-pdfkit/html2pdf authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package main

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/go-pdfkit/reader"
)

// exec runs the command with args and returns its error and combined output.
func exec(t *testing.T, args ...string) (error, string) {
	t.Helper()
	cmd := RootCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(legacyArgs(args))
	err := cmd.Execute()
	return err, buf.String()
}

func isUsage(err error) bool {
	var ue *usageError
	return errors.As(err, &ue)
}

func writeHTML(t *testing.T, body string) (dir, in string) {
	t.Helper()
	dir = t.TempDir()
	in = filepath.Join(dir, "in.html")
	if err := os.WriteFile(in, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, in
}

func isPDF(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if len(data) < 5 || string(data[:5]) != "%PDF-" {
		t.Fatalf("output is not a PDF")
	}
	return data
}

// infoField reads one text entry of the PDF information dictionary.
func infoField(t *testing.T, data []byte, key string) string {
	t.Helper()
	doc, err := reader.Open(data)
	if err != nil {
		t.Fatal(err)
	}
	obj, ok := doc.Trailer()["Info"]
	if !ok {
		t.Fatal("no /Info in trailer")
	}
	res, err := doc.Resolve(obj)
	if err != nil {
		t.Fatal(err)
	}
	info, ok := reader.ToDict(res)
	if !ok {
		t.Fatal("/Info is not a dictionary")
	}
	v, err := doc.Resolve(info[reader.Name(key)])
	if err != nil {
		t.Fatal(err)
	}
	s, _ := reader.TextString(v)
	return s
}

func TestMissingInAndURL(t *testing.T) {
	err, _ := exec(t)
	if !isUsage(err) {
		t.Errorf("no flags: err = %v, want usage error", err)
	}
}

func TestRejectsBothInAndURL(t *testing.T) {
	err, _ := exec(t, "--in", "x.html", "--url", "http://127.0.0.1:1/")
	if !isUsage(err) {
		t.Errorf("both: err = %v, want usage error", err)
	}
}

func TestBadFlagIsUsage(t *testing.T) {
	err, _ := exec(t, "--nosuchflag")
	if !isUsage(err) {
		t.Errorf("bad flag: err = %v, want usage error", err)
	}
}

func TestURLFetchesAndRenders(t *testing.T) {
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
	if err, msg := exec(t, "--url", srv.URL+"/page", "--out", out); err != nil {
		t.Fatalf("--url: %v\n%s", err, msg)
	}
	isPDF(t, out)
}

func TestURLFetchFailure(t *testing.T) {
	err, _ := exec(t, "--url", "http://127.0.0.1:1/nothing", "--out", filepath.Join(t.TempDir(), "o.pdf"), "--timeout", "5s")
	if err == nil || isUsage(err) {
		t.Errorf("unreachable URL: err = %v, want runtime error", err)
	}
}

func TestUnreadableIn(t *testing.T) {
	err, _ := exec(t, "--in", "/nonexistent/file.html")
	if err == nil || isUsage(err) {
		t.Errorf("missing file: err = %v, want runtime error", err)
	}
}

func TestUnwritableOut(t *testing.T) {
	dir, in := writeHTML(t, `<html><body>x</body></html>`)
	err, _ := exec(t, "--in", in, "--out", filepath.Join(dir, "nodir", "out.pdf"))
	if err == nil || isUsage(err) {
		t.Errorf("unwritable out: err = %v, want runtime error", err)
	}
}

func TestEndToEndPOSIXFlags(t *testing.T) {
	dir, in := writeHTML(t, `<html><body>hello</body></html>`)
	out := filepath.Join(dir, "out.pdf")
	if err, msg := exec(t, "--in", in, "--out", out, "--margin", "15", "--media", "screen", "--image-dpi", "150"); err != nil {
		t.Fatalf("%v\n%s", err, msg)
	}
	isPDF(t, out)
}

func TestShortFlags(t *testing.T) {
	dir, in := writeHTML(t, `<html><body>hello</body></html>`)
	out := filepath.Join(dir, "out.pdf")
	if err, msg := exec(t, "-i", in, "-o", out, "-m", "15"); err != nil {
		t.Fatalf("%v\n%s", err, msg)
	}
	isPDF(t, out)
}

// TestLegacySingleDashFlags keeps the spellings of the flag-package releases.
func TestLegacySingleDashFlags(t *testing.T) {
	dir, in := writeHTML(t, `<html><body>hello</body></html>`)
	out := filepath.Join(dir, "out.pdf")
	if err, msg := exec(t, "-in", in, "-out", out, "-margin", "15", "-media=print", "-image-dpi", "0", "-timeout", "5s"); err != nil {
		t.Fatalf("%v\n%s", err, msg)
	}
	isPDF(t, out)
}

func TestLegacyArgs(t *testing.T) {
	got := legacyArgs([]string{"-in", "a", "-out=b", "-i", "c", "-io", "-m5", "--in", "d", "--", "-in"})
	want := []string{"--in", "a", "--out=b", "-i", "c", "-io", "-m5", "--in", "d", "--", "-in"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("legacyArgs = %q, want %q", got, want)
	}
}

func TestMetadataFlags(t *testing.T) {
	dir, in := writeHTML(t, `<html><head><title>From HTML</title></head><body>hello</body></html>`)
	out := filepath.Join(dir, "out.pdf")
	if err, msg := exec(t, "--in", in, "--out", out, "--title", "Flag Title", "--author", "Ada Lovelace",
		"--subject", "A subject", "--keywords", "alpha, beta"); err != nil {
		t.Fatalf("%v\n%s", err, msg)
	}
	data := isPDF(t, out)
	for key, want := range map[string]string{
		"Title": "Flag Title", "Author": "Ada Lovelace", "Subject": "A subject", "Keywords": "alpha, beta",
	} {
		if got := infoField(t, data, key); got != want {
			t.Errorf("/%s = %q, want %q", key, got, want)
		}
	}
}

func TestTitleDefaultsToDocumentTitle(t *testing.T) {
	dir, in := writeHTML(t, `<html><head><title>From HTML</title></head><body>hello</body></html>`)
	out := filepath.Join(dir, "out.pdf")
	if err, msg := exec(t, "--in", in, "--out", out); err != nil {
		t.Fatalf("%v\n%s", err, msg)
	}
	if got := infoField(t, isPDF(t, out), "Title"); got != "From HTML" {
		t.Errorf("/Title = %q, want %q", got, "From HTML")
	}
}

func TestMainExitCodes(t *testing.T) {
	old, oldArgs := osExit, os.Args
	defer func() { osExit, os.Args = old, oldArgs }()
	var code int
	osExit = func(c int) { code = c }

	os.Args = []string{"html2pdf"}
	main()
	if code != 2 {
		t.Errorf("usage exit = %d, want 2", code)
	}
	code = 0
	os.Args = []string{"html2pdf", "-in", "/nonexistent/file.html"}
	main()
	if code != 1 {
		t.Errorf("runtime exit = %d, want 1", code)
	}
}
