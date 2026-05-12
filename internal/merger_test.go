package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFile creates a file in dir with the given content and returns its path.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFindFiles(t *testing.T) {
	t.Run("empty dir", func(t *testing.T) {
		got, err := FindFiles(t.TempDir(), ".go")
		if err != nil || len(got) != 0 {
			t.Errorf("FindFiles() = (%v, %v), want ([], nil)", got, err)
		}
	})
	t.Run("sorted order", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "b.go"), nil, 0o644)
		os.WriteFile(filepath.Join(dir, "a.go"), nil, 0o644)
		got, _ := FindFiles(dir, ".go")
		if len(got) != 2 || filepath.Base(got[0]) != "a.go" || filepath.Base(got[1]) != "b.go" {
			t.Errorf("expected [a.go, b.go], got %v", got)
		}
	})
	t.Run("test file excluded", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "a.go"), nil, 0o644)
		os.WriteFile(filepath.Join(dir, "a_test.go"), nil, 0o644)
		got, _ := FindFiles(dir, ".go")
		if len(got) != 1 {
			t.Errorf("expected 1 file, got %v", got)
		}
	})
	t.Run("subdir skipped", func(t *testing.T) {
		dir := t.TempDir()
		os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
		os.WriteFile(filepath.Join(dir, "a.go"), nil, 0o644)
		os.WriteFile(filepath.Join(dir, "sub", "b.go"), nil, 0o644)
		got, _ := FindFiles(dir, ".go")
		if len(got) != 1 {
			t.Errorf("expected 1 file, got %v", got)
		}
	})
	t.Run("wrong extension excluded", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "a.go"), nil, 0o644)
		os.WriteFile(filepath.Join(dir, "b.py"), nil, 0o644)
		got, _ := FindFiles(dir, ".go")
		if len(got) != 1 {
			t.Errorf("expected 1 file, got %v", got)
		}
	})
	t.Run("non-existent dir", func(t *testing.T) {
		_, err := FindFiles("/no/such/path/xyz123", ".go")
		if err == nil {
			t.Error("expected error for non-existent dir")
		}
	})
}

func TestPromoteEntry(t *testing.T) {
	tests := []struct {
		name  string
		files []string
		entry string
		want0 string
		wantN int
	}{
		{"already first", []string{"/d/main.go", "/d/a.go", "/d/b.go"}, "main.go", "main.go", 3},
		{"middle", []string{"/d/a.go", "/d/main.go", "/d/b.go"}, "main.go", "main.go", 3},
		{"last", []string{"/d/a.go", "/d/b.go", "/d/main.go"}, "main.go", "main.go", 3},
		{"not found unchanged", []string{"/d/a.go", "/d/b.go"}, "main.go", "a.go", 2},
		{"single element", []string{"/d/main.go"}, "main.go", "main.go", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := promoteEntry(tt.files, tt.entry)
			if len(got) != tt.wantN {
				t.Fatalf("len=%d, want %d", len(got), tt.wantN)
			}
			if filepath.Base(got[0]) != tt.want0 {
				t.Errorf("got[0] base = %q, want %q", filepath.Base(got[0]), tt.want0)
			}
		})
	}
}

func TestResolveOutput(t *testing.T) {
	tests := []struct {
		output string
		ext    string
		want   string
	}{
		{"out.ext", ".go", "out.go"},
		{"/tmp/result.ext", ".py", "/tmp/result.py"},
		{"out.txt", ".go", "out.txt"},
		{"out", ".go", "out"},
	}
	for _, tt := range tests {
		t.Run(tt.output, func(t *testing.T) {
			got := resolveOutput(tt.output, &stubAdapter{ext: tt.ext})
			if got != tt.want {
				t.Errorf("resolveOutput(%q, %q) = %q, want %q", tt.output, tt.ext, got, tt.want)
			}
		})
	}
}

func TestResolveAdapter(t *testing.T) {
	// Relies on the real registry populated by init() in golang.go and python.go.
	t.Run("explicit go", func(t *testing.T) {
		a, err := resolveAdapter("", "go")
		if err != nil || a.Name() != "go" {
			t.Errorf("got (%v, %v), want (go adapter, nil)", a, err)
		}
	})
	t.Run("explicit unknown", func(t *testing.T) {
		_, err := resolveAdapter("", "lolcode")
		if err == nil || !strings.Contains(err.Error(), "unknown language") {
			t.Errorf("expected 'unknown language' error, got %v", err)
		}
	})
	t.Run("auto-detect via go marker", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "main.go"), nil, 0o644)
		a, err := resolveAdapter(dir, "")
		if err != nil || a.Name() != "go" {
			t.Errorf("got (%v, %v), want (go adapter, nil)", a, err)
		}
	})
	t.Run("auto-detect fails on empty dir", func(t *testing.T) {
		_, err := resolveAdapter(t.TempDir(), "")
		if err == nil {
			t.Error("expected error, got nil")
		}
	})
}
