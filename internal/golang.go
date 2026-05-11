package internal

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
)

type GoAdapter struct{}

func init() {
	Register(&GoAdapter{})
}

func (a *GoAdapter) Name() string      { return "go" }
func (a *GoAdapter) Extension() string { return ".go" }
func (a *GoAdapter) Markers() []string { return []string{"main.go"} }

type goImport struct {
	name string // alias: "", "_", ".", or actual alias
	path string // quoted path, e.g., `"fmt"`
}

func (a *GoAdapter) Merge(files []string) ([]byte, error) {
	var packageName string
	var bodies [][]byte
	seen := map[string]struct{}{}
	var imports []goImport

	for i, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, file, data, parser.ParseComments)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
		if i == 0 {
			packageName = f.Name.Name
		}

		for _, imp := range f.Imports {
			name := ""
			if imp.Name != nil {
				name = imp.Name.Name
			}
			key := name + "\x00" + imp.Path.Value
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			imports = append(imports, goImport{name: name, path: imp.Path.Value})
		}

		bodies = append(bodies, stripBoilerplate(data, fset, f))
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "package %s\n\n", packageName)
	if len(imports) > 0 {
		buf.WriteString("import (\n")
		for _, imp := range imports {
			buf.WriteByte('\t')
			if imp.name != "" {
				buf.WriteString(imp.name)
				buf.WriteByte(' ')
			}
			buf.WriteString(imp.path)
			buf.WriteByte('\n')
		}
		buf.WriteString(")\n\n")
	}
	for _, body := range bodies {
		body = bytes.TrimSpace(body)
		if len(body) == 0 {
			continue
		}
		buf.Write(body)
		buf.WriteString("\n\n")
	}
	return buf.Bytes(), nil
}

type byteSpan struct{ start, end int }

func stripBoilerplate(data []byte, fset *token.FileSet, f *ast.File) []byte {
	spans := []byteSpan{lineSpan(data, fset.Position(f.Package).Offset)}

	for _, decl := range f.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.IMPORT {
			continue
		}
		start := fset.Position(gen.Pos()).Offset
		end := fset.Position(gen.End()).Offset
		if end < len(data) && data[end] == '\n' {
			end++
		}
		spans = append(spans, byteSpan{start, end})
	}

	for _, cg := range f.Comments {
		start := fset.Position(cg.Pos()).Offset
		end := fset.Position(cg.End()).Offset
		spans = append(spans, commentSpan(data, start, end))
	}

	sort.Slice(spans, func(a, b int) bool {
		if spans[a].start == spans[b].start {
			return spans[a].end > spans[b].end
		}
		return spans[a].start < spans[b].start
	})

	var out bytes.Buffer
	pos := 0
	for _, s := range spans {
		if s.start < pos {
			if s.end > pos {
				pos = s.end
			}
			continue
		}
		out.Write(data[pos:s.start])
		pos = s.end
	}
	out.Write(data[pos:])
	return out.Bytes()
}

func lineSpan(data []byte, start int) byteSpan {
	end := start
	if nl := bytes.IndexByte(data[start:], '\n'); nl >= 0 {
		end = start + nl + 1
	} else {
		end = len(data)
	}
	return byteSpan{start, end}
}

func commentSpan(data []byte, start, end int) byteSpan {
	lineStart := start
	onlyWs := true
	for lineStart > 0 && data[lineStart-1] != '\n' {
		if data[lineStart-1] != ' ' && data[lineStart-1] != '\t' {
			onlyWs = false
			break
		}
		lineStart--
	}
	if onlyWs {
		lineEnd := end
		for lineEnd < len(data) && (data[lineEnd] == ' ' || data[lineEnd] == '\t') {
			lineEnd++
		}
		if lineEnd < len(data) && data[lineEnd] == '\n' {
			lineEnd++
		}
		return byteSpan{lineStart, lineEnd}
	}
	trimStart := start
	for trimStart > 0 && (data[trimStart-1] == ' ' || data[trimStart-1] == '\t') {
		trimStart--
	}
	return byteSpan{trimStart, end}
}
