//go:build tools

// Package corpus's harness tools that are run, not imported: recorded here
// so go.mod pins their versions and `go run` resolves them offline.
package corpus

import _ "github.com/go-pdfkit/conformance/cmd/judges"
