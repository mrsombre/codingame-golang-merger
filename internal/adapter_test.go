package internal

import (
	"os"
	"path/filepath"
	"testing"
)

// stubAdapter is shared across all internal test files.
type stubAdapter struct {
	name    string
	ext     string
	markers []string
}

func (s *stubAdapter) Name() string      { return s.name }
func (s *stubAdapter) Extension() string { return s.ext }
func (s *stubAdapter) Markers() []string { return s.markers }
func (s *stubAdapter) Merge(_ []string) ([]byte, error) { return nil, nil }

// saveRegistry snapshots the global adapters map and returns a restore func.
func saveRegistry() func() {
	saved := make(map[string]Adapter, len(adapters))
	for k, v := range adapters {
		saved[k] = v
	}
	return func() { adapters = saved }
}

func TestRegisterGet(t *testing.T) {
	restore := saveRegistry()
	t.Cleanup(restore)
	adapters = map[string]Adapter{}

	Register(&stubAdapter{name: "x", ext: ".a"})

	t.Run("found", func(t *testing.T) {
		_, ok := Get("x")
		if !ok {
			t.Error("Get returned ok=false for registered adapter")
		}
	})
	t.Run("not found", func(t *testing.T) {
		_, ok := Get("y")
		if ok {
			t.Error("Get returned ok=true for unregistered adapter")
		}
	})
	t.Run("overwrite", func(t *testing.T) {
		Register(&stubAdapter{name: "x", ext: ".b"})
		got, _ := Get("x")
		if got.Extension() != ".b" {
			t.Errorf("extension = %q, want .b", got.Extension())
		}
	})
}

func TestNames(t *testing.T) {
	restore := saveRegistry()
	t.Cleanup(restore)

	tests := []struct {
		name  string
		setup []string
		want  []string
	}{
		{"empty", nil, []string{}},
		{"single", []string{"x"}, []string{"x"}},
		{"sorted", []string{"z", "a", "m"}, []string{"a", "m", "z"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapters = map[string]Adapter{}
			for _, n := range tt.setup {
				Register(&stubAdapter{name: n})
			}
			got := Names()
			if len(got) != len(tt.want) {
				t.Fatalf("Names() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestDetect(t *testing.T) {
	restore := saveRegistry()
	t.Cleanup(restore)
	adapters = map[string]Adapter{}
	Register(&stubAdapter{name: "go", ext: ".go", markers: []string{"main.go"}})
	Register(&stubAdapter{name: "python", ext: ".py", markers: []string{"solution.py"}})

	t.Run("go marker", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "main.go"), nil, 0o644)
		a, ok := Detect(dir)
		if !ok || a.Name() != "go" {
			t.Errorf("Detect() = (%v, %v), want (go, true)", a, ok)
		}
	})
	t.Run("python marker", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "solution.py"), nil, 0o644)
		a, ok := Detect(dir)
		if !ok || a.Name() != "python" {
			t.Errorf("Detect() = (%v, %v), want (python, true)", a, ok)
		}
	})
	t.Run("no marker", func(t *testing.T) {
		_, ok := Detect(t.TempDir())
		if ok {
			t.Error("Detect() = true, want false")
		}
	})
	t.Run("go wins when both present", func(t *testing.T) {
		// Names() is sorted: "go" < "python", so go adapter is checked first.
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "main.go"), nil, 0o644)
		os.WriteFile(filepath.Join(dir, "solution.py"), nil, 0o644)
		a, ok := Detect(dir)
		if !ok || a.Name() != "go" {
			t.Errorf("Detect() = (%v, %v), want (go, true)", a, ok)
		}
	})
}
