// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package vulncheck provides an analysis command
// that runs vulnerability analysis using data from
// golang.org/x/vuln/vulncheck.
// This package requires go1.18 or newer.
package vulncheck

import (
	"context"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/gopls/internal/govulncheck"
	"golang.org/x/tools/gopls/internal/lsp/command"
	"golang.org/x/tools/gopls/internal/lsp/source"
)

// Govulncheck runs the in-process govulncheck implementation.
// With go1.18+, this is swapped with the real implementation.
var Govulncheck func(ctx context.Context, cfg *packages.Config, patterns string) (res command.VulncheckResult, _ error) = nil

var Main func(cfg packages.Config, patterns ...string) error = nil

func AnalyzeVulnerableImports(ctx context.Context, snapshot source.Snapshot, modfile source.FileHandle) (*govulncheck.Result, error) {
	panic("not implemented")
}
