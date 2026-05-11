package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const placeholderExt = ".ext"

func FindFiles(dir, ext string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ext) {
			continue
		}
		if strings.HasSuffix(name, "_test"+ext) {
			continue
		}
		files = append(files, filepath.Join(dir, name))
	}
	sort.Strings(files)
	return files, nil
}

func promoteEntry(files []string, entry string) []string {
	for i, f := range files {
		if filepath.Base(f) != entry {
			continue
		}
		out := make([]string, 0, len(files))
		out = append(out, f)
		out = append(out, files[:i]...)
		out = append(out, files[i+1:]...)
		return out
	}
	return files
}

func resolveAdapter(source, lang string) (Adapter, error) {
	if lang != "" {
		a, ok := Get(lang)
		if !ok {
			return nil, fmt.Errorf("unknown language %q (available: %s)", lang, strings.Join(Names(), ", "))
		}
		return a, nil
	}
	a, ok := Detect(source)
	if !ok {
		return nil, fmt.Errorf("could not auto-detect language in %s", source)
	}
	return a, nil
}

func resolveOutput(output string, a Adapter) string {
	if filepath.Ext(output) == placeholderExt {
		return strings.TrimSuffix(output, placeholderExt) + a.Extension()
	}
	return output
}

func Run(source, output, lang string) (string, error) {
	a, err := resolveAdapter(source, lang)
	if err != nil {
		return "", err
	}

	files, err := FindFiles(source, a.Extension())
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", fmt.Errorf("no %s files found in %s", a.Extension(), source)
	}
	if markers := a.Markers(); len(markers) > 0 {
		files = promoteEntry(files, markers[0])
	}

	data, err := a.Merge(files)
	if err != nil {
		return "", err
	}

	out := resolveOutput(output, a)
	if err := os.WriteFile(out, data, 0o644); err != nil {
		return "", err
	}
	return out, nil
}
