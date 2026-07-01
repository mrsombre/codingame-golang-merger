package internal

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

func mustParseGo(t *testing.T, src string) ([]byte, *token.FileSet, *ast.File) {
	t.Helper()
	data := []byte(src)
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", data, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	return data, fset, f
}

func TestLineSpan(t *testing.T) {
	tests := []struct {
		name  string
		data  string
		start int
		want  byteSpan
	}{
		{"newline in data", "package main\nfoo", 0, byteSpan{0, 13}},
		{"no trailing newline", "package main", 0, byteSpan{0, 12}},
		{"start mid-data", "abc\ndef", 4, byteSpan{4, 7}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lineSpan([]byte(tt.data), tt.start)
			if got != tt.want {
				t.Errorf("lineSpan(%q, %d) = %v, want %v", tt.data, tt.start, got, tt.want)
			}
		})
	}
}

func TestStripBoilerplate(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		contains []string
		absent   []string
	}{
		{
			name:   "package decl stripped",
			src:    "package main\nfunc f() {}",
			absent: []string{"package"},
		},
		{
			name:     "single import stripped",
			src:      `package main` + "\n" + `import "fmt"` + "\n" + `func f() {}`,
			contains: []string{"func f()"},
			absent:   []string{"import", `"fmt"`},
		},
		{
			name:     "import block stripped",
			src:      "package main\nimport (\n\t\"fmt\"\n\t\"os\"\n)\nfunc f() {}",
			contains: []string{"func f()"},
			absent:   []string{"import", `"fmt"`, `"os"`},
		},
		{
			name:     "line comment stripped",
			src:      "package main\n// line comment\nfunc f() {}",
			contains: []string{"func f()"},
			absent:   []string{"// line comment"},
		},
		{
			name:     "block comment stripped",
			src:      "package main\n/* block comment */\nfunc f() {}",
			contains: []string{"func f()"},
			absent:   []string{"/* block comment */"},
		},
		{
			name:     "inline line comment stripped",
			src:      "package main\nfunc f() {} // trailing",
			contains: []string{"func f()"},
			absent:   []string{"// trailing"},
		},
		{
			name:     "inline block comment stripped",
			src:      "package main\nfunc f(/* note */ x int) {}",
			contains: []string{"func f("},
			absent:   []string{"/* note */"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, fset, f := mustParseGo(t, tt.src)
			got := stripBoilerplate(data, fset, f)
			for _, s := range tt.contains {
				if !bytes.Contains(got, []byte(s)) {
					t.Errorf("expected %q in output:\n%s", s, got)
				}
			}
			for _, s := range tt.absent {
				if bytes.Contains(got, []byte(s)) {
					t.Errorf("expected %q NOT in output:\n%s", s, got)
				}
			}
		})
	}
}

func TestGoAdapterMerge(t *testing.T) {
	a := &GoAdapter{}

	t.Run("single file", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "main.go", "package main\nimport \"fmt\"\nfunc main() { fmt.Println() }\n")
		got, err := a.Merge([]string{filepath.Join(dir, "main.go")})
		if err != nil {
			t.Fatal(err)
		}
		s := string(got)
		for _, want := range []string{"package main", `"fmt"`, "func main()"} {
			if !strings.Contains(s, want) {
				t.Errorf("expected %q in output:\n%s", want, s)
			}
		}
	})

	t.Run("dedup imports across files", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "main.go", "package main\nimport \"fmt\"\nfunc main() {}\n")
		writeFile(t, dir, "helper.go", "package main\nimport \"fmt\"\nfunc f() {}\n")
		files := []string{
			filepath.Join(dir, "main.go"),
			filepath.Join(dir, "helper.go"),
		}
		got, err := a.Merge(files)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Count(string(got), `"fmt"`) != 1 {
			t.Errorf(`expected "fmt" exactly once in:\n%s`, got)
		}
	})

	t.Run("aliased import kept alongside plain", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "main.go", "package main\nimport f \"fmt\"\nfunc main() { f.Println() }\n")
		writeFile(t, dir, "helper.go", "package main\nimport \"fmt\"\nfunc h() { fmt.Println() }\n")
		files := []string{
			filepath.Join(dir, "main.go"),
			filepath.Join(dir, "helper.go"),
		}
		got, err := a.Merge(files)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Count(string(got), `"fmt"`) != 2 {
			t.Errorf(`expected "fmt" twice (aliased + plain), got:\n%s`, got)
		}
	})

	t.Run("all comment forms stripped", func(t *testing.T) {
		dir := t.TempDir()
		src := "package main\n// line\n/* block */\nfunc f() {} // inline\n"
		writeFile(t, dir, "main.go", src)
		got, err := a.Merge([]string{filepath.Join(dir, "main.go")})
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range []string{"// line", "/* block */", "// inline"} {
			if strings.Contains(string(got), c) {
				t.Errorf("comment %q not stripped from:\n%s", c, got)
			}
		}
	})

	t.Run("formats merged source", func(t *testing.T) {
		dir := t.TempDir()
		src := "package main\nimport \"fmt\"\nfunc main(){\n    if true {\n        fmt.Println(\"x\")\n    }\n\n\n\n}\n"
		writeFile(t, dir, "main.go", src)
		got, err := a.Merge([]string{filepath.Join(dir, "main.go")})
		if err != nil {
			t.Fatal(err)
		}
		s := string(got)
		if strings.Contains(s, "\t") {
			t.Errorf("expected tabs removed from merged source:\n%s", s)
		}
		if strings.Contains(s, "\n\n") {
			t.Errorf("expected repeated blank lines collapsed in:\n%s", s)
		}
		if !strings.Contains(s, "\nif true {\nfmt.Println(\"x\")\n}\n") {
			t.Errorf("expected readable newlines to remain in:\n%s", s)
		}
	})

	t.Run("keeps raw string whitespace while compacting source", func(t *testing.T) {
		dir := t.TempDir()
		src := "package main\nfunc main(){\n\t_ = `a\tb\n\nc`\n\n\n}\n"
		writeFile(t, dir, "main.go", src)
		got, err := a.Merge([]string{filepath.Join(dir, "main.go")})
		if err != nil {
			t.Fatal(err)
		}
		s := string(got)
		if !strings.Contains(s, "`a\tb\n\nc`") {
			t.Errorf("expected raw string whitespace preserved in:\n%s", s)
		}
		if strings.Contains(strings.ReplaceAll(s, "`a\tb\n\nc`", "`raw`"), "\n\n") {
			t.Errorf("expected source blank lines collapsed outside raw strings:\n%s", s)
		}
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := a.Merge([]string{"/no/such/file.go"})
		if err == nil {
			t.Error("expected error for missing file")
		}
	})

	t.Run("invalid go source", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "bad.go", "not valid go !!!")
		_, err := a.Merge([]string{filepath.Join(dir, "bad.go")})
		if err == nil {
			t.Error("expected parse error")
		}
	})
}
