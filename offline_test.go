// Copyright (c) the go-pdfkit/html2pdf authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package html2pdf

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// recorder is a loopback server that counts every request it receives and
// serves a stylesheet, an @import target, a PNG and an SVG, and a redirect.
type recorder struct {
	*httptest.Server
	mu   sync.Mutex
	hits []string
}

func newRecorder(t testing.TB) *recorder {
	t.Helper()
	r := &recorder{}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		r.mu.Lock()
		r.hits = append(r.hits, req.URL.Path)
		r.mu.Unlock()
		switch req.URL.Path {
		case "/s.css":
			w.Header().Set("Content-Type", "text/css")
			w.Write([]byte("@import url('/imp.css');\np{color:#f00}"))
		case "/imp.css":
			w.Header().Set("Content-Type", "text/css")
			w.Write([]byte("p{margin:0}"))
		case "/i.png":
			w.Header().Set("Content-Type", "image/png")
			w.Write(raw(fxPNGAlpha))
		case "/i.svg":
			w.Header().Set("Content-Type", "image/svg+xml")
			w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><rect width="10" height="10"/></svg>`))
		case "/r":
			http.Redirect(w, req, "/i.png", http.StatusFound)
		default:
			http.NotFound(w, req)
		}
	})
	r.Server = httptest.NewServer(mux)
	t.Cleanup(r.Close)
	return r
}

func (r *recorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.hits)
}

func (r *recorder) reset() {
	r.mu.Lock()
	r.hits = nil
	r.mu.Unlock()
}

// offlineOpts is Options{Fetch: Offline} plus a refusal counter.
func offlineOpts(base string) (Options, *atomic.Int64) {
	var n atomic.Int64
	return Options{BaseURL: base, Fetch: Offline, OnRefused: func(string) { n.Add(1) }}, &n
}

// Every kind of sub-resource: with Offline the server sees nothing; the same
// document without Offline reaches it (the control) for the kinds the engine
// does load. A kind the engine never loads (a script, a frame, a font) must
// stay at zero in the control too — if that changes, the control fails and
// the Offline claim has to be re-examined, not silently kept.
func TestOfflineMakesNoRequest(t *testing.T) {
	srv := newRecorder(t)
	abs := srv.URL
	cases := []struct {
		name   string
		html   string
		loaded bool // the default options fetch it (control must see a hit)
	}{
		{"link stylesheet, absolute", `<link rel=stylesheet href="` + abs + `/s.css"><p>x</p>`, true},
		{"link stylesheet, relative to BaseURL", `<link rel=stylesheet href="/s.css"><p>x</p>`, true},
		{"link stylesheet with @import", `<link rel=stylesheet href="` + abs + `/s.css"><p>x</p>`, true},
		{"link media=print", `<link rel=stylesheet media=print href="/s.css"><p>x</p>`, true},
		{"img, absolute", `<img src="` + abs + `/i.png">`, true},
		{"img, relative to BaseURL", `<img src="i.png">`, true},
		{"img svg", `<img src="` + abs + `/i.svg">`, true},
		{"img through a redirect", `<img src="` + abs + `/r">`, true},
		// html2pdf paints no CSS background images (the engine loads them for
		// its raster painter only), so the control sees nothing; Offline must
		// stay at zero should that ever change.
		{"css background-image", `<div style="width:20px;height:20px;background:url(` + abs + `/i.png)">x</div>`, false},
		{"css background in a style element", `<style>div{width:20px;height:20px;background-image:url("` + abs + `/i.png")}</style><div>x</div>`, false},
		{"inline svg image href", `<svg width=10 height=10><image href="` + abs + `/i.png" width=10 height=10/></svg>`, false},
		{"svg data URI with an external image", `<img src="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' xmlns:xlink='http://www.w3.org/1999/xlink' width='10' height='10'%3E%3Cimage xlink:href='` + abs + `/i.png' width='10' height='10'/%3E%3C/svg%3E">`, false},
		{"srcset and picture", `<picture><source srcset="` + abs + `/i.png"><img srcset="` + abs + `/i.png 2x" alt=a></picture>`, false},
		{"style @import", `<style>@import url("` + abs + `/s.css");</style><p>x</p>`, false},
		{"@font-face", `<style>@font-face{font-family:F;src:url("` + abs + `/f.woff2")}p{font-family:F}</style><p>x</p>`, false},
		{"iframe", `<iframe src="` + abs + `/frame.html"></iframe>`, false},
		{"object", `<object data="` + abs + `/o.png" type="image/png"></object>`, false},
		{"embed", `<embed src="` + abs + `/e.png">`, false},
		{"script src", `<script src="` + abs + `/a.js"></script><p>x</p>`, false},
		{"video poster", `<video poster="` + abs + `/i.png" src="` + abs + `/v.mp4"></video>`, false},
		{"favicon link", `<link rel=icon href="` + abs + `/i.png"><p>x</p>`, false},
		{"meta refresh", `<meta http-equiv=refresh content="0;url=` + abs + `/x"><p>x</p>`, false},
		{"base href (ignored: BaseURL decides)", `<base href="http://127.0.0.1:1/"><img src="i.png">`, true},
		{"form action and anchors", `<form action="` + abs + `/post"><input type=submit></form><a href="` + abs + `/a">a</a>`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			doc := "<html><head></head><body>" + c.html + "</body></html>"

			srv.reset()
			opts, refused := offlineOpts(abs + "/")
			exportBytes(t, doc, opts)
			if n := srv.count(); n != 0 {
				t.Fatalf("Offline: the server saw %d request(s): %v", n, srv.hits)
			}
			if c.loaded && refused.Load() == 0 {
				t.Errorf("Offline: no refusal counted for a document that asks for a resource")
			}

			srv.reset()
			exportBytes(t, doc, Options{BaseURL: abs + "/"})
			got := srv.count()
			if c.loaded && got == 0 {
				t.Errorf("control: the default options made no request; the test cannot see a fetch")
			}
			if !c.loaded && got != 0 {
				t.Logf("control: the engine now fetches this kind (%d hit(s): %v); move it to loaded", got, srv.hits)
				t.Fail()
			}
		})
	}
}

// The count of refusals equals the number of requests the default options
// would have sent for the same document.
func TestOfflineCountsRefusals(t *testing.T) {
	srv := newRecorder(t)
	doc := `<html><head><link rel=stylesheet href="/s.css"></head><body>
<img src="/i.png"><img src="/i.svg"><div style="width:5px;height:5px;background:url(/r)">x</div></body></html>`
	exportBytes(t, doc, Options{BaseURL: srv.URL + "/"})
	want := srv.count()
	if want < 3 {
		t.Fatalf("control: %d hits, want at least 3", want)
	}
	srv.reset()
	var urls []string
	var mu sync.Mutex
	exportBytes(t, doc, Options{BaseURL: srv.URL + "/", Fetch: Offline, OnRefused: func(u string) {
		mu.Lock()
		urls = append(urls, u)
		mu.Unlock()
	}})
	if srv.count() != 0 {
		t.Fatalf("Offline: %d hits", srv.count())
	}
	// the stylesheet is refused, so its @import is never reached: 1 sheet + 3 images
	// the default also fetched the sheet's @import, which Offline never reaches
	if len(urls) != want-1 || want != 4 {
		t.Errorf("refusals: %d %v (default hits %d), want 3 refusals (the sheet and two images) and 4 hits", len(urls), urls, want)
	}
	for _, u := range urls {
		if !strings.HasPrefix(u, srv.URL) {
			t.Errorf("refused URL %q does not name the server", u)
		}
	}
}

// A data: image still renders under Offline, and is not counted as a refusal.
func TestOfflineKeepsDataImages(t *testing.T) {
	doc := `<html><body><img src="` + dataURI("image/png", fxPNGAlpha) + `"><img src="` + dataURI("image/jpeg", fxJPEG) + `"></body></html>`
	opts, refused := offlineOpts("")
	pdf := exportBytes(t, doc, opts)
	if n := bytes.Count(pdf, []byte("/Subtype /Image")); n < 2 {
		t.Errorf("images embedded: %d, want 2", n)
	}
	if n := refused.Load(); n != 0 {
		t.Errorf("refusals for data: images: %d, want 0", n)
	}
	if want := exportBytes(t, doc, Options{}); !bytes.Equal(pdf, want) {
		t.Error("Offline changed the output of a document with only data: images")
	}
}

// A missing resource is left out, never an error: the page still exports.
func TestOfflineRefusalIsNotAnError(t *testing.T) {
	opts, _ := offlineOpts("https://example.invalid/")
	doc, err := Export(`<link rel=stylesheet href=/a.css><p>kept text</p><img src=/a.png>`, opts)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	var buf bytes.Buffer
	if err := doc.Write(&buf); err != nil {
		t.Fatal(err)
	}
	if bytes.Count(buf.Bytes(), []byte("/Subtype /Image")) != 0 {
		t.Error("an image was embedded from nowhere")
	}
}

// A file: BaseURL does not turn a relative src into a filesystem read, with
// or without Offline: the engine's FileRoot stays empty.
func TestFileBaseURLReadsNothing(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.png"), raw(fxPNGAlpha), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.css"), []byte("p{display:none}"), 0o600); err != nil {
		t.Fatal(err)
	}
	base := (&url.URL{Scheme: "file", Path: dir + "/"}).String()
	doc := `<html><head><link rel=stylesheet href="a.css"></head><body><p>t</p><img src="a.png"><img src="` + base + `a.png"><img src="file://localhost` + filepath.ToSlash(dir) + `/a.png"></body></html>`
	for name, opts := range map[string]Options{
		"default": {BaseURL: base},
		"offline": {BaseURL: base, Fetch: Offline},
	} {
		pdf := exportBytes(t, doc, opts)
		if n := bytes.Count(pdf, []byte("/Subtype /Image")); n != 0 {
			t.Errorf("%s: a file: image was read (%d embedded)", name, n)
		}
	}
}

// A custom Fetch is honoured and sees only http(s) URLs.
func TestFetchHookIsUsed(t *testing.T) {
	var got []string
	var mu sync.Mutex
	opts := Options{BaseURL: "https://example.test/", Fetch: func(_ context.Context, u string) ([]byte, string, error) {
		mu.Lock()
		got = append(got, u)
		mu.Unlock()
		return raw(fxPNGAlpha), "image/png", nil
	}}
	pdf := exportBytes(t, `<html><body><img src="/x.png"><img src="`+dataURI("image/png", fxPNGAlpha)+`"></body></html>`, opts)
	if len(got) != 1 || got[0] != "https://example.test/x.png" {
		t.Errorf("Fetch saw %v, want only the one http(s) URL", got)
	}
	if n := bytes.Count(pdf, []byte("/Subtype /Image")); n != 2 {
		t.Errorf("images embedded: %d, want 2", n)
	}
	// an error that is not ErrOffline is not a counted refusal
	var n atomic.Int64
	opts.Fetch = func(context.Context, string) ([]byte, string, error) { return nil, "", errors.New("boom") }
	opts.OnRefused = func(string) { n.Add(1) }
	exportBytes(t, `<img src="/x.png">`, opts)
	if n.Load() != 0 {
		t.Error("a plain error was counted as a refusal")
	}
	// a wrapped ErrOffline is
	opts.Fetch = func(context.Context, string) ([]byte, string, error) {
		return nil, "", errors.Join(errors.New("policy"), ErrOffline)
	}
	exportBytes(t, `<img src="/x.png">`, opts)
	if n.Load() != 1 {
		t.Errorf("wrapped ErrOffline: %d refusals, want 1", n.Load())
	}
}

func TestOfflineFetcherRefusesEverything(t *testing.T) {
	for _, u := range []string{"http://127.0.0.1/", "https://x/", "data:text/plain,hi", "file:///etc/passwd", "", "ftp://x"} {
		body, ct, err := Offline(context.Background(), u)
		if body != nil || ct != "" || !errors.Is(err, ErrOffline) {
			t.Errorf("Offline(%q) = %v, %q, %v", u, body, ct, err)
		}
	}
}

// Hostile URL shapes, in every place that takes one: none reaches the server.
func TestOfflineHostileURLs(t *testing.T) {
	srv := newRecorder(t)
	u, _ := url.Parse(srv.URL)
	hostport := u.Host
	hostile := []string{
		srv.URL + "/i.png",
		srv.URL + "/r",
		"//" + hostport + "/i.png",
		"HTTP://" + hostport + "/i.png",
		" \t" + srv.URL + "/i.png",
		"http://user:pw@" + hostport + "/i.png",
		"http://" + hostport + "@" + hostport + "/i.png",
		"http:" + hostport + "/i.png",
		"http:/" + hostport + "/i.png",
		"http:\\\\" + hostport + "\\i.png",
		"http://" + strings.Replace(hostport, "127.0.0.1", "[::ffff:127.0.0.1]", 1) + "/i.png",
		"http://localhost:" + u.Port() + "/i.png",
		"http://2130706433:" + u.Port() + "/i.png",
		"http://0x7f.1:" + u.Port() + "/i.png",
		"http://" + hostport + "/%2e%2e/i.png",
		"http://" + hostport + "/i.png#frag",
		"http://" + hostport + "/i.png?" + strings.Repeat("a", 70000),
		"../i.png",
		"/i.png",
		"i.png",
		"?x",
		"file:///etc/passwd",
		"FILE:///etc/passwd",
		"file://" + hostport + "/etc/passwd",
		"javascript:alert(1)",
		"ftp://" + hostport + "/i.png",
		"gopher://" + hostport + "/",
		"ws://" + hostport + "/",
		"blob:" + srv.URL + "/x",
		"data:image/png,not-a-png",
		"data:text/css,@import url(" + srv.URL + "/s.css);",
		"data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg'><image href='" + srv.URL + "/i.png'/></svg>",
		"",
		"\x00" + srv.URL,
	}
	for i, h := range hostile {
		doc := "<html><head><link rel=stylesheet href=\"" + h + "\"><style>@import url(\"" + h + "\");div{background:url(\"" + h + "\")}</style></head><body><div>x</div><img src=\"" + h + "\"></body></html>"
		opts, _ := offlineOpts(srv.URL + "/")
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("case %d (%q): panic: %v", i, truncate(h), r)
				}
			}()
			exportBytes(t, doc, opts)
		}()
		if n := srv.count(); n != 0 {
			t.Errorf("case %d (%q): %d request(s): %v", i, truncate(h), n, srv.hits)
			srv.reset()
		}
	}
}

func truncate(s string) string {
	if len(s) > 60 {
		return s[:60] + "..."
	}
	return s
}

// FuzzOfflineMakesNoRequest: whatever string sits in a URL position, under
// Offline the loopback server is never reached.
func FuzzOfflineMakesNoRequest(f *testing.F) {
	srv := newRecorder(f)
	for _, s := range []string{srv.URL + "/i.png", "/s.css", "//" + srv.Listener.Addr().String() + "/i.png", "i.png", "file:///etc/hosts", "data:,x", "http://[::1]/", "\x00", "http://", ""} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, ref string) {
		if len(ref) > 4096 {
			t.Skip()
		}
		doc := "<html><head><link rel=stylesheet href=\"" + attr(ref) + "\"></head><body><img src=\"" + attr(ref) + "\"><div style=\"background:url('" + attr(ref) + "')\">x</div></body></html>"
		srv.reset()
		opts, _ := offlineOpts(srv.URL + "/")
		d, err := Export(doc, opts)
		if err != nil {
			return
		}
		var buf bytes.Buffer
		_ = d.Write(&buf)
		if n := srv.count(); n != 0 {
			t.Fatalf("%d request(s) for %q: %v", n, ref, srv.hits)
		}
	})
}

// attr makes s safe inside a double-quoted attribute and a single-quoted CSS
// string, so the fuzzer explores URLs, not markup injection.
func attr(s string) string {
	return strings.NewReplacer(`"`, "%22", "'", "%27", "\\", "%5C", "&", "%26", "<", "%3C", ")", "%29").Replace(s)
}
