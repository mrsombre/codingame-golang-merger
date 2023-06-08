package internal

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Merger struct {
	tree         *ast.File
	addedImports map[string]ast.Spec
	addedConsts  map[string]ast.Spec
	addedTypes   map[string]ast.Spec
	addedVars    map[string]ast.Spec
	addedFunc    map[string]*ast.FuncDecl
	specialFunc  map[string]bool
}

func NewMerger() *Merger {
	merger := &Merger{
		tree: &ast.File{
			Name: ast.NewIdent("main"),
		},
		addedImports: make(map[string]ast.Spec),
		addedConsts:  make(map[string]ast.Spec),
		addedTypes:   make(map[string]ast.Spec),
		addedVars:    make(map[string]ast.Spec),
		addedFunc:    make(map[string]*ast.FuncDecl),
		specialFunc:  map[string]bool{`init`: true, `main`: true},
	}

	return merger
}

func (m *Merger) ParseDir(dirName, sourceName string) error {
	fileInfo, err := os.ReadDir(dirName)
	if err != nil {
		panic(err)
	}

	for _, f := range fileInfo {
		if f.IsDir() {
			continue
		}

		file, _ := f.Info()
		filename := file.Name()
		if strings.HasSuffix(filename, "_test.go") {
			continue
		}
		if !strings.HasSuffix(filename, ".go") {
			continue
		}
		if filename == sourceName {
			continue
		}

		path := filepath.Join(dirName, filename)
		if err := m.parseFile(path); err != nil {
			return err
		}
	}

	return nil
}

func (m *Merger) parseFile(path string) error {
	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return err
	}

	for i, decl := range file.Decls {
		if gen, ok := decl.(*ast.GenDecl); ok {
			if gen.Tok == token.PACKAGE {
				file.Decls = append(file.Decls[:i], file.Decls[i+1:]...)
				break
			}
		}
	}

	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.GenDecl:
			m.parseGenDecl(decl)
		case *ast.FuncDecl:
			name := decl.Name.Name
			if _, ok := m.specialFunc[name]; !ok {
				name = fset.Position(decl.Pos()).String()
			}
			m.addedFunc[name] = decl
		}
	}

	return nil
}

func (m *Merger) parseGenDecl(decl *ast.GenDecl) {
	switch decl.Tok {
	case token.IMPORT:
		for _, spec := range decl.Specs {
			if v, ok := spec.(*ast.ImportSpec); ok {
				m.addedImports[v.Path.Value] = spec
			}
		}
	case token.CONST:
		for _, spec := range decl.Specs {
			if v, ok := spec.(*ast.ValueSpec); ok {
				for _, name := range v.Names {
					m.addedConsts[name.Name] = spec
				}
			}
		}
	case token.TYPE:
		for _, spec := range decl.Specs {
			if t, ok := spec.(*ast.TypeSpec); ok {
				m.addedTypes[t.Name.Name] = spec
			}
		}
	case token.VAR:
		for _, spec := range decl.Specs {
			if v, ok := spec.(*ast.ValueSpec); ok {
				for _, name := range v.Names {
					m.addedVars[name.Name] = spec
				}
			}
		}
	}
}

func (m *Merger) buildGenDecl() {
	var specs []ast.Spec

	specs = make([]ast.Spec, 0, len(m.addedImports))
	for _, spec := range m.addedImports {
		specs = append(specs, spec)
	}
	if len(specs) > 0 {
		m.tree.Decls = append(m.tree.Decls, &ast.GenDecl{
			Tok:   token.IMPORT,
			Specs: specs,
		})
	}

	specs = make([]ast.Spec, 0, len(m.addedConsts))
	for _, spec := range m.addedConsts {
		specs = append(specs, spec)
	}
	if len(specs) > 0 {
		m.tree.Decls = append(m.tree.Decls, &ast.GenDecl{
			Tok:   token.CONST,
			Specs: specs,
		})
	}

	specs = make([]ast.Spec, 0, len(m.addedVars))
	for _, spec := range m.addedVars {
		specs = append(specs, spec)
	}
	if len(specs) > 0 {
		m.tree.Decls = append(m.tree.Decls, &ast.GenDecl{
			Tok:   token.VAR,
			Specs: specs,
		})
	}

	specs = make([]ast.Spec, 0, len(m.addedTypes))
	for _, spec := range m.addedTypes {
		specs = append(specs, spec)
	}
	if len(specs) > 0 {
		m.tree.Decls = append(m.tree.Decls, &ast.GenDecl{
			Tok:   token.TYPE,
			Specs: specs,
		})
	}
}

func (m *Merger) sortAddedFuncs() []*ast.FuncDecl {
	keys := make([]string, 0, len(m.addedFunc))
	for k := range m.addedFunc {
		if _, ok := m.specialFunc[k]; ok {
			continue
		}
		keys = append(keys, k)
	}

	sort.Strings(keys)

	sortedFuncs := make([]*ast.FuncDecl, len(keys))
	for i, k := range keys {
		sortedFuncs[i] = m.addedFunc[k]
	}

	for k := range m.specialFunc {
		if _, ok := m.addedFunc[k]; !ok {
			continue
		}
		sortedFuncs = append(sortedFuncs, m.addedFunc[k])
	}

	return sortedFuncs
}

func (m *Merger) WriteToFile(sourceName string) error {
	source, err := os.Create(sourceName)
	if err != nil {
		return err
	}
	defer func(source *os.File) {
		_ = source.Close()
	}(source)

	m.buildGenDecl()

	for _, decl := range m.sortAddedFuncs() {
		m.tree.Decls = append(m.tree.Decls, decl)
	}

	if err := printer.Fprint(source, token.NewFileSet(), m.tree); err != nil {
		return err
	}

	return nil
}
