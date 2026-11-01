// Copyright (c) the go-pdfkit/html2pdf authors. All rights reserved.
// Use of this source code is governed by a BSD-3-Clause license that can be
// found in the LICENSE file at the root of this repository.

package html2pdf

import (
	"context"
	"errors"
)

// Fetcher fetches one http(s) sub-resource — a stylesheet, an @import target,
// an image — for Export, returning its body and Content-Type. It is the
// go-webengine engine's own Engine.Fetcher hook (engine.go in
// go-webengine/engine), so it sees every request Export would otherwise send
// itself, and none that the engine resolves without the network: a data: URL
// is decoded in place, and a file: URL is refused by the engine unless it
// is given a FileRoot, which Export never does.
//
// A Fetcher may be called from several goroutines at once (stylesheets and
// images are loaded concurrently).
type Fetcher func(ctx context.Context, rawurl string) (body []byte, contentType string, err error)

// ErrOffline is what Offline returns. A Fetcher that wraps it (errors.Is)
// is counted as a refusal by Options.OnRefused.
var ErrOffline = errors.New("html2pdf: network access refused (offline)")

// Offline is a Fetcher that refuses every request. Pass it as Options.Fetch to
// render with no network access at all: a stylesheet, an @import, an image or
// a CSS background that is not a data: URL is treated as missing — the layout
// carries on without it, Export does not fail — and Options.OnRefused is told
// which URL it was.
//
// Nothing is ever sent, so no redirect can be followed, and no DNS lookup is
// made. data: URLs never reach the Fetcher (the engine decodes them in
// place), so inline images still render. A file: URL is refused by the engine
// itself: html2pdf never sets the engine's FileRoot, so a BaseURL of
// file:///… cannot make a relative src read the filesystem.
func Offline(ctx context.Context, rawurl string) ([]byte, string, error) {
	return nil, "", ErrOffline
}

// fetcher adapts Options.Fetch for the engine, reporting refusals. It returns
// nil when the caller set no Fetch, leaving the engine's own client in place.
func (o Options) fetcher() Fetcher {
	if o.Fetch == nil {
		return nil
	}
	f, on := o.Fetch, o.OnRefused
	return func(ctx context.Context, u string) ([]byte, string, error) {
		body, ct, err := f(ctx, u)
		if err != nil && on != nil && errors.Is(err, ErrOffline) {
			on(u)
		}
		return body, ct, err
	}
}
