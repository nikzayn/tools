// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package misc

import (
	"fmt"
	"strings"
	"testing"

	"golang.org/x/tools/gopls/internal/lsp/fake"
	"golang.org/x/tools/gopls/internal/lsp/protocol"
	. "golang.org/x/tools/gopls/internal/lsp/regtest"
	"golang.org/x/tools/internal/testenv"
)

func TestHoverUnexported(t *testing.T) {
	const proxy = `
-- golang.org/x/structs@v1.0.0/go.mod --
module golang.org/x/structs

go 1.12

-- golang.org/x/structs@v1.0.0/types.go --
package structs

type Mixed struct {
	// Exported comment
	Exported   int
	unexported string
}

func printMixed(m Mixed) {
	println(m)
}
`
	const mod = `
-- go.mod --
module mod.com

go 1.12

require golang.org/x/structs v1.0.0
-- go.sum --
golang.org/x/structs v1.0.0 h1:Ito/a7hBYZaNKShFrZKjfBA/SIPvmBrcPCBWPx5QeKk=
golang.org/x/structs v1.0.0/go.mod h1:47gkSIdo5AaQaWJS0upVORsxfEr1LL1MWv9dmYF3iq4=
-- main.go --
package main

import "golang.org/x/structs"

func main() {
	var m structs.Mixed
	_ = m.Exported
}
`

	// TODO: use a nested workspace folder here.
	WithOptions(
		ProxyFiles(proxy),
	).Run(t, mod, func(t *testing.T, env *Env) {
		env.OpenFile("main.go")
		mixedLoc := env.RegexpSearch("main.go", "Mixed")
		got, _ := env.Hover(mixedLoc)
		if !strings.Contains(got.Value, "unexported") {
			t.Errorf("Workspace hover: missing expected field 'unexported'. Got:\n%q", got.Value)
		}

		cacheLoc := env.GoToDefinition(mixedLoc)
		cacheFile := env.Sandbox.Workdir.URIToPath(cacheLoc.URI)
		argLoc := env.RegexpSearch(cacheFile, "printMixed.*(Mixed)")
		got, _ = env.Hover(argLoc)
		if !strings.Contains(got.Value, "unexported") {
			t.Errorf("Non-workspace hover: missing expected field 'unexported'. Got:\n%q", got.Value)
		}

		exportedFieldLoc := env.RegexpSearch("main.go", "Exported")
		got, _ = env.Hover(exportedFieldLoc)
		if !strings.Contains(got.Value, "comment") {
			t.Errorf("Workspace hover: missing comment for field 'Exported'. Got:\n%q", got.Value)
		}
	})
}

func TestHoverIntLiteral(t *testing.T) {
	// TODO(rfindley): this behavior doesn't actually make sense for vars. It is
	// misleading to format their value when it is (of course) variable.
	//
	// Instead, we should allow hovering on numeric literals.
	t.Skip("golang/go#58220: broken due to new hover logic")

	const source = `
-- main.go --
package main

var (
	bigBin = 0b1001001
)

var hex = 0xe34e

func main() {
}
`
	Run(t, source, func(t *testing.T, env *Env) {
		env.OpenFile("main.go")
		hexExpected := "58190"
		got, _ := env.Hover(env.RegexpSearch("main.go", "hex"))
		if got != nil && !strings.Contains(got.Value, hexExpected) {
			t.Errorf("Hover: missing expected field '%s'. Got:\n%q", hexExpected, got.Value)
		}

		binExpected := "73"
		got, _ = env.Hover(env.RegexpSearch("main.go", "bigBin"))
		if got != nil && !strings.Contains(got.Value, binExpected) {
			t.Errorf("Hover: missing expected field '%s'. Got:\n%q", binExpected, got.Value)
		}
	})
}

// Tests that hovering does not trigger the panic in golang/go#48249.
func TestPanicInHoverBrokenCode(t *testing.T) {
	// Note: this test can not be expressed as a marker test, as it must use
	// content without a trailing newline.
	const source = `
-- main.go --
package main

type Example struct`
	Run(t, source, func(t *testing.T, env *Env) {
		env.OpenFile("main.go")
		env.Editor.Hover(env.Ctx, env.RegexpSearch("main.go", "Example"))
	})
}

func TestHoverRune_48492(t *testing.T) {
	const files = `
-- go.mod --
module mod.com

go 1.18
-- main.go --
package main
`
	Run(t, files, func(t *testing.T, env *Env) {
		env.OpenFile("main.go")
		env.EditBuffer("main.go", fake.NewEdit(0, 0, 1, 0, "package main\nfunc main() {\nconst x = `\nfoo\n`\n}"))
		env.Editor.Hover(env.Ctx, env.RegexpSearch("main.go", "foo"))
	})
}

func TestHoverImport(t *testing.T) {
	const packageDoc1 = "Package lib1 hover documentation"
	const packageDoc2 = "Package lib2 hover documentation"
	tests := []struct {
		hoverPackage string
		want         string
		wantError    bool
	}{
		{
			"mod.com/lib1",
			packageDoc1,
			false,
		},
		{
			"mod.com/lib2",
			packageDoc2,
			false,
		},
		{
			"mod.com/lib3",
			"",
			false,
		},
		{
			"mod.com/lib4",
			"",
			true,
		},
	}
	source := fmt.Sprintf(`
-- go.mod --
module mod.com

go 1.12
-- lib1/a.go --
// %s
package lib1

const C = 1

-- lib1/b.go --
package lib1

const D = 1

-- lib2/a.go --
// %s
package lib2

const E = 1

-- lib3/a.go --
package lib3

const F = 1

-- main.go --
package main

import (
	"mod.com/lib1"
	"mod.com/lib2"
	"mod.com/lib3"
	"mod.com/lib4"
)

func main() {
	println("Hello")
}
	`, packageDoc1, packageDoc2)
	Run(t, source, func(t *testing.T, env *Env) {
		env.OpenFile("main.go")
		for _, test := range tests {
			got, _, err := env.Editor.Hover(env.Ctx, env.RegexpSearch("main.go", test.hoverPackage))
			if test.wantError {
				if err == nil {
					t.Errorf("Hover(%q) succeeded unexpectedly", test.hoverPackage)
				}
			} else if !strings.Contains(got.Value, test.want) {
				t.Errorf("Hover(%q): got:\n%q\nwant:\n%q", test.hoverPackage, got.Value, test.want)
			}
		}
	})
}

// for x/tools/gopls: unhandled named anchor on the hover #57048
func TestHoverTags(t *testing.T) {
	const source = `
-- go.mod --
module mod.com

go 1.19

-- lib/a.go --

// variety of execution modes.
//
// # Test package setup
//
// The regression test package uses a couple of uncommon patterns to reduce
package lib

-- a.go --
	package main
	import "mod.com/lib"

	const A = 1

}
`
	Run(t, source, func(t *testing.T, env *Env) {
		t.Run("tags", func(t *testing.T) {
			env.OpenFile("a.go")
			z := env.RegexpSearch("a.go", "lib")
			t.Logf("%#v", z)
			got, _ := env.Hover(env.RegexpSearch("a.go", "lib"))
			if strings.Contains(got.Value, "{#hdr-") {
				t.Errorf("Hover: got {#hdr- tag:\n%q", got)
			}
		})
	})
}

// This is a regression test for Go issue #57625.
func TestHoverModMissingModuleStmt(t *testing.T) {
	const source = `
-- go.mod --
go 1.16
`
	Run(t, source, func(t *testing.T, env *Env) {
		env.OpenFile("go.mod")
		env.Hover(env.RegexpSearch("go.mod", "go")) // no panic
	})
}

func TestHoverCompletionMarkdown(t *testing.T) {
	testenv.NeedsGo1Point(t, 19)
	const source = `
-- go.mod --
module mod.com
go 1.19
-- main.go --
package main
// Just says [hello].
//
// [hello]: https://en.wikipedia.org/wiki/Hello
func Hello() string {
	Hello() //Here
    return "hello"
}
`
	Run(t, source, func(t *testing.T, env *Env) {
		// Hover, Completion, and SignatureHelp should all produce markdown
		// check that the markdown for SignatureHelp and Completion are
		// the same, and contained in that for Hover (up to trailing \n)
		env.OpenFile("main.go")
		loc := env.RegexpSearch("main.go", "func (Hello)")
		hover, _ := env.Hover(loc)
		hoverContent := hover.Value

		loc = env.RegexpSearch("main.go", "//Here")
		loc.Range.Start.Character -= 3 // Hello(_) //Here
		completions := env.Completion(loc)
		signatures := env.SignatureHelp(loc)

		if len(completions.Items) != 1 {
			t.Errorf("got %d completions, expected 1", len(completions.Items))
		}
		if len(signatures.Signatures) != 1 {
			t.Errorf("got %d signatures, expected 1", len(signatures.Signatures))
		}
		item := completions.Items[0].Documentation.Value
		var itemContent string
		if x, ok := item.(protocol.MarkupContent); !ok || x.Kind != protocol.Markdown {
			t.Fatalf("%#v is not markdown", item)
		} else {
			itemContent = strings.Trim(x.Value, "\n")
		}
		sig := signatures.Signatures[0].Documentation.Value
		var sigContent string
		if x, ok := sig.(protocol.MarkupContent); !ok || x.Kind != protocol.Markdown {
			t.Fatalf("%#v is not markdown", item)
		} else {
			sigContent = x.Value
		}
		if itemContent != sigContent {
			t.Errorf("item:%q not sig:%q", itemContent, sigContent)
		}
		if !strings.Contains(hoverContent, itemContent) {
			t.Errorf("hover:%q does not containt sig;%q", hoverContent, sigContent)
		}
	})
}
