package internal

import (
	"os"
	"path/filepath"
	"sort"
)

type Adapter interface {
	Name() string
	Extension() string
	Markers() []string
	Merge(files []string) ([]byte, error)
}

var adapters = map[string]Adapter{}

func Register(a Adapter) {
	adapters[a.Name()] = a
}

func Get(name string) (Adapter, bool) {
	a, ok := adapters[name]
	return a, ok
}

func Names() []string {
	names := make([]string, 0, len(adapters))
	for name := range adapters {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func Detect(dir string) (Adapter, bool) {
	for _, name := range Names() {
		a := adapters[name]
		for _, marker := range a.Markers() {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return a, true
			}
		}
	}
	return nil, false
}
