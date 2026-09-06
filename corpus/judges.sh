#!/bin/sh
# Judge every corpus PDF with every reader on this machine, through
# go-pdfkit/conformance's cmd/judges — the org's harness, moved there from
# this repository (conformance #27) so that render, ops and gotex use the
# same one. pdf.js runs under node from the harness's own scripts, which
# need `npm ci` in a WRITABLE copy of conformance's judges/ directory: the
# Go module cache is read-only, so the copy lives in .judges-node/ (ignored).
set -eu
cd "$(dirname "$0")"
MOD=$(go list -m -f '{{.Dir}}' github.com/go-pdfkit/conformance)
NODE=.judges-node
if [ ! -f "$NODE/package-lock.json" ] || ! cmp -s "$MOD/judges/package-lock.json" "$NODE/package-lock.json"; then
  rm -rf "$NODE" && mkdir -p "$NODE" && cp "$MOD"/judges/*.mjs "$MOD"/judges/package.json "$MOD"/judges/package-lock.json "$NODE"/
  (cd "$NODE" && npm ci --silent)
fi
exec go run github.com/go-pdfkit/conformance/cmd/judges \
  -pdfs 'out/*.pdf,out/bench/*.pdf' -out out/judges -report JUDGES.md -results judges.json \
  -nodedir "$NODE" -pdfium "${PDFIUM_TEST:-/Users/Shared/pdfiumbuild/pdfium-checkout/pdfium/out/xfa/pdfium_test}" "$@"
