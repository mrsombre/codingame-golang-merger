package internal

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

type PythonAdapter struct{}

func init() {
	Register(&PythonAdapter{})
}

func (a *PythonAdapter) Name() string      { return "python" }
func (a *PythonAdapter) Extension() string { return ".py" }
func (a *PythonAdapter) Markers() []string { return []string{"solution.py"} }

func (a *PythonAdapter) Merge(files []string) ([]byte, error) {
	localModules := map[string]struct{}{}
	for _, f := range files {
		mod := strings.TrimSuffix(filepath.Base(f), ".py")
		localModules[mod] = struct{}{}
	}

	// Move solution.py to last: helpers must be defined before the game loop runs.
	if len(files) > 1 {
		files = append(files[1:], files[0])
	}

	var importLines []string
	seenImports := map[string]struct{}{}
	var bodies [][]byte

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		fileImports, body := parsePythonFile(data, localModules)
		for _, imp := range fileImports {
			if _, ok := seenImports[imp]; !ok {
				seenImports[imp] = struct{}{}
				importLines = append(importLines, imp)
			}
		}
		bodies = append(bodies, body)
	}

	var buf bytes.Buffer
	for _, imp := range importLines {
		buf.WriteString(imp)
		buf.WriteByte('\n')
	}
	if len(importLines) > 0 {
		buf.WriteByte('\n')
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

// parsePythonFile strips local imports and comments, and returns non-local imports separately.
func parsePythonFile(data []byte, localModules map[string]struct{}) (imports []string, body []byte) {
	lines := bytes.Split(data, []byte("\n"))
	var bodyLines [][]byte
	inTriple := false
	var tripleMarker string

	for _, rawLine := range lines {
		stripped := strings.TrimSpace(string(rawLine))

		if inTriple {
			if strings.Contains(stripped, tripleMarker) {
				inTriple = false
			}
			continue
		}

		// Standalone triple-quoted string (docstring / block comment).
		if strings.HasPrefix(stripped, `"""`) || strings.HasPrefix(stripped, `'''`) {
			marker := stripped[:3]
			rest := stripped[3:]
			if !strings.Contains(rest, marker) {
				inTriple = true
				tripleMarker = marker
			}
			continue
		}

		// Pure line comment.
		if strings.HasPrefix(stripped, "#") {
			continue
		}

		// Import line.
		if strings.HasPrefix(stripped, "import ") || strings.HasPrefix(stripped, "from ") {
			mod := pyModuleName(stripped)
			if _, local := localModules[mod]; local {
				continue
			}
			imports = append(imports, stripped)
			continue
		}

		bodyLines = append(bodyLines, []byte(stripPyInlineComment(string(rawLine))))
	}

	body = bytes.Join(bodyLines, []byte("\n"))
	return
}

func pyModuleName(line string) string {
	fields := strings.Fields(line)
	if len(fields) >= 2 {
		return fields[1]
	}
	return ""
}

// stripPyInlineComment removes a trailing # comment, respecting string literals.
func stripPyInlineComment(line string) string {
	inSingle := false
	inDouble := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case c == '\'' && !inDouble:
			inSingle = !inSingle
		case c == '"' && !inSingle:
			inDouble = !inDouble
		case c == '#' && !inSingle && !inDouble:
			return strings.TrimRight(line[:i], " \t")
		}
	}
	return line
}
