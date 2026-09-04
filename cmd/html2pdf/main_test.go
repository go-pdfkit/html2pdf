// Copyright (c) the go-pdfkit/html2pdf authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunMissingIn(t *testing.T) {
	if code := run(nil, os.Stderr); code != 2 {
		t.Errorf("run(nil) = %d, want 2", code)
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
