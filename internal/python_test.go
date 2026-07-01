package internal

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestPyModuleName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"import sys", "sys"},
		{"import os.path", "os.path"},
		{"from funcs import foo", "funcs"},
		{"from . import foo", "."},
		{"import numpy as np", "numpy"},
		{"", ""},
	}
	for _, tt := range tests {
		got := pyModuleName(tt.input)
		if got != tt.want {
			t.Errorf("pyModuleName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestStripPyInlineComment(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"x = 1", "x = 1"},
		{"x = 1  # note", "x = 1"},
		{`x = "a#b"`, `x = "a#b"`},
		{"x = 'a#b'", "x = 'a#b'"},
		{`x = "a" # c`, `x = "a"`},
		{"# standalone", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := stripPyInlineComment(tt.input)
		if got != tt.want {
			t.Errorf("stripPyInlineComment(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParsePythonFile(t *testing.T) {
	noLocals := map[string]struct{}{}
	withLocals := map[string]struct{}{"funcs": {}, "classes": {}}

	tests := []struct {
		name         string
		src          string
		localMods    map[string]struct{}
		wantImports  []string
		bodyContains []string
		bodyAbsent   []string
	}{
		{
			name:        "non-local import kept",
			src:         "import sys\n",
			localMods:   noLocals,
			wantImports: []string{"import sys"},
		},
		{
			name:      "local import stripped",
			src:       "from funcs import foo\n",
			localMods: withLocals,
		},
		{
			name:        "import with alias not treated as local",
			src:         "import numpy as np\n",
			localMods:   noLocals,
			wantImports: []string{"import numpy as np"},
		},
		{
			name:         "line comment stripped",
			src:          "# comment\nx = 1\n",
			localMods:    noLocals,
			bodyContains: []string{"x = 1"},
			bodyAbsent:   []string{"# comment"},
		},
		{
			name:         "multi-line triple-double-quote docstring stripped",
			src:          "\"\"\"\ndoc\n\"\"\"\nx = 1\n",
			localMods:    noLocals,
			bodyContains: []string{"x = 1"},
			bodyAbsent:   []string{"doc"},
		},
		{
			name:         "single-line triple-double-quote stripped",
			src:          "\"\"\"doc\"\"\"\nx = 1\n",
			localMods:    noLocals,
			bodyContains: []string{"x = 1"},
			bodyAbsent:   []string{"doc"},
		},
		{
			name:         "multi-line triple-single-quote docstring stripped",
			src:          "'''\ndoc\n'''\nx = 1\n",
			localMods:    noLocals,
			bodyContains: []string{"x = 1"},
			bodyAbsent:   []string{"doc"},
		},
		{
			name:         "inline comment stripped from body line",
			src:          "x = 1 # note\n",
			localMods:    noLocals,
			bodyContains: []string{"x = 1"},
			bodyAbsent:   []string{"# note"},
		},
		{
			name:      "empty input",
			src:       "",
			localMods: noLocals,
		},
		{
			name:         "code after closed docstring preserved",
			src:          "\"\"\"\ndoc\n\"\"\"\ndef f(): pass\n",
			localMods:    noLocals,
			bodyContains: []string{"def f(): pass"},
			bodyAbsent:   []string{"doc"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			imports, body := parsePythonFile([]byte(tt.src), tt.localMods)
			if len(imports) != len(tt.wantImports) {
				t.Fatalf("imports = %v, want %v", imports, tt.wantImports)
			}
			for i, imp := range imports {
				if imp != tt.wantImports[i] {
					t.Errorf("imports[%d] = %q, want %q", i, imp, tt.wantImports[i])
				}
			}
			for _, s := range tt.bodyContains {
				if !bytes.Contains(body, []byte(s)) {
					t.Errorf("body should contain %q:\n%s", s, body)
				}
			}
			for _, s := range tt.bodyAbsent {
				if bytes.Contains(body, []byte(s)) {
					t.Errorf("body should NOT contain %q:\n%s", s, body)
				}
			}
		})
	}
}

func TestPythonAdapterMerge(t *testing.T) {
	a := &PythonAdapter{}

	t.Run("solution.py body appears last", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "solution.py", "x = 'main'\n")
		writeFile(t, dir, "helper.py", "y = 'helper'\n")
		files := []string{
			filepath.Join(dir, "solution.py"),
			filepath.Join(dir, "helper.py"),
		}
		got, err := a.Merge(files)
		if err != nil {
			t.Fatal(err)
		}
		s := string(got)
		if strings.Index(s, "y = 'helper'") > strings.Index(s, "x = 'main'") {
			t.Errorf("helper body should precede solution body in:\n%s", s)
		}
	})

	t.Run("local imports stripped", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "solution.py", "from helper import foo\ndef main(): foo()\n")
		writeFile(t, dir, "helper.py", "def foo(): pass\n")
		files := []string{
			filepath.Join(dir, "solution.py"),
			filepath.Join(dir, "helper.py"),
		}
		got, err := a.Merge(files)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(got), "from helper import") {
			t.Errorf("local import not stripped:\n%s", got)
		}
	})

	t.Run("non-local imports deduplicated", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "solution.py", "import sys\nx = 1\n")
		writeFile(t, dir, "helper.py", "import sys\ny = 2\n")
		files := []string{
			filepath.Join(dir, "solution.py"),
			filepath.Join(dir, "helper.py"),
		}
		got, err := a.Merge(files)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Count(string(got), "import sys") != 1 {
			t.Errorf("expected 'import sys' exactly once:\n%s", got)
		}
	})

	t.Run("comments stripped", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "solution.py", "x = 1\n")
		writeFile(t, dir, "helper.py", "# line comment\n\"\"\"\ndoc\n\"\"\"\ny = 2  # inline\n")
		files := []string{
			filepath.Join(dir, "solution.py"),
			filepath.Join(dir, "helper.py"),
		}
		got, err := a.Merge(files)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range []string{"# line comment", "doc", "# inline"} {
			if strings.Contains(string(got), c) {
				t.Errorf("comment %q not stripped:\n%s", c, got)
			}
		}
	})

	t.Run("single file", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, "solution.py", "x = 1\n")
		got, err := a.Merge([]string{filepath.Join(dir, "solution.py")})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(got), "x = 1") {
			t.Errorf("expected body in:\n%s", got)
		}
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := a.Merge([]string{"/no/such/file.py"})
		if err == nil {
			t.Error("expected error for missing file")
		}
	})
}
