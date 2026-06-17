//go:build ignore

package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
)

func main() {
	fset := token.NewFileSet()
	src, err := os.ReadFile("internal/hcloud/client.go")
	if err != nil {
		panic(err)
	}
	f, err := parser.ParseFile(fset, "internal/hcloud/client.go", src, 0)
	if err != nil {
		panic(err)
	}

	var methods []struct{ name, params, results string }
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name.Name != "Interface" {
				continue
			}
			it, ok := ts.Type.(*ast.InterfaceType)
			if !ok {
				continue
			}
			for _, m := range it.Methods.List {
				if len(m.Names) == 0 {
					continue
				}
				fn, ok := m.Type.(*ast.FuncType)
				if !ok {
					continue
				}
				methods = append(methods, struct{ name, params, results string }{
					name:    m.Names[0].Name,
					params:  fieldListString(src, fn.Params),
					results: resultListString(src, fn.Results),
				})
			}
		}
	}

	var b strings.Builder
	b.WriteString(`package hcloud

import (
	"time"

	"github.com/armanfeyzi/hcloud-operator/internal/metrics"
)

// instrumentedClient wraps an Interface and records Prometheus metrics for each API call.
type instrumentedClient struct {
	inner Interface
}

// Instrument returns c wrapped with API call metrics. Passing nil returns nil.
// Wrapping an already-instrumented client is a no-op.
func Instrument(c Interface) Interface {
	if c == nil {
		return nil
	}
	if _, ok := c.(*instrumentedClient); ok {
		return c
	}
	return &instrumentedClient{inner: c}
}

func recordAPI(op string, err error, start time.Time) {
	result := "success"
	if err != nil {
		result = "error"
	}
	metrics.RecordAPI(op, result, time.Since(start))
}

`)
	for _, m := range methods {
		args := callArgNames(m.params)
		b.WriteString(fmt.Sprintf("func (c *instrumentedClient) %s(%s) %s {\n", m.name, m.params, m.results))
		b.WriteString("\tstart := time.Now()\n")
		if m.results == "error" {
			b.WriteString(fmt.Sprintf("\terr := c.inner.%s(%s)\n", m.name, args))
			b.WriteString(fmt.Sprintf("\trecordAPI(%q, err, start)\n", m.name))
			b.WriteString("\treturn err\n")
		} else {
			b.WriteString(fmt.Sprintf("\tv, err := c.inner.%s(%s)\n", m.name, args))
			b.WriteString(fmt.Sprintf("\trecordAPI(%q, err, start)\n", m.name))
			b.WriteString("\treturn v, err\n")
		}
		b.WriteString("}\n\n")
	}

	if err := os.WriteFile("internal/hcloud/instrumented.go", []byte(b.String()), 0o644); err != nil {
		panic(err)
	}
	fmt.Printf("wrote %d methods to internal/hcloud/instrumented.go\n", len(methods))
}

func fieldListString(src []byte, fl *ast.FieldList) string {
	if fl == nil {
		return ""
	}
	var parts []string
	for _, f := range fl.List {
		ty := string(src[f.Type.Pos()-1 : f.Type.End()-1])
		if len(f.Names) == 0 {
			parts = append(parts, ty)
			continue
		}
		for _, n := range f.Names {
			parts = append(parts, n.Name+" "+ty)
		}
	}
	return strings.Join(parts, ", ")
}

func resultListString(src []byte, fl *ast.FieldList) string {
	if fl == nil {
		return ""
	}
	if len(fl.List) == 1 && len(fl.List[0].Names) == 0 {
		ty := string(src[fl.List[0].Type.Pos()-1 : fl.List[0].Type.End()-1])
		if ty == "error" {
			return "error"
		}
	}
	var parts []string
	for _, f := range fl.List {
		ty := string(src[f.Type.Pos()-1 : f.Type.End()-1])
		parts = append(parts, ty)
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

func callArgNames(params string) string {
	if params == "" {
		return ""
	}
	var names []string
	for _, p := range strings.Split(params, ", ") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		fields := strings.Fields(p)
		if len(fields) >= 2 {
			names = append(names, fields[0])
		}
	}
	return strings.Join(names, ", ")
}
