package internal

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// findModuleRoot walks up from startDir looking for go.mod.
func findModuleRoot(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

// readModuleName parses go.mod to extract the module name.
func readModuleName(repoRoot string) (string, error) {
	f, err := os.Open(filepath.Join(repoRoot, "go.mod"))
	if err != nil {
		return "", err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")), nil
		}
	}
	return "", fmt.Errorf("module directive not found in go.mod")
}

// recvTypeName extracts the receiver type name from a method declaration.
func recvTypeName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}
	t := fn.Recv.List[0].Type
	if star, ok := t.(*ast.StarExpr); ok {
		t = star.X
	}
	if ident, ok := t.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

// funcKey returns a unique key for a function declaration.
// Methods are keyed as "RecvType.MethodName", standalone functions by name.
func funcKey(fn *ast.FuncDecl) string {
	recv := recvTypeName(fn)
	if recv == "" {
		return fn.Name.Name
	}
	return recv + "." + fn.Name.Name
}

// PackageSymbols holds all declarations from a parsed package.
type PackageSymbols struct {
	alias       string
	funcs       map[string]*ast.FuncDecl
	types       map[string]ast.Spec
	vars        map[string]ast.Spec
	consts      []*ast.GenDecl
	constNames  map[string]bool
	imports     map[string]ast.Spec
	typeMethods map[string][]string // type name → list of func keys
}

func parsePackageSymbols(dir, alias string) (*PackageSymbols, error) {
	pkg := &PackageSymbols{
		alias:       alias,
		funcs:       make(map[string]*ast.FuncDecl),
		types:       make(map[string]ast.Spec),
		vars:        make(map[string]ast.Spec),
		constNames:  make(map[string]bool),
		imports:     make(map[string]ast.Spec),
		typeMethods: make(map[string][]string),
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet()
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		path := filepath.Join(dir, name)
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return nil, err
		}

		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				switch d.Tok {
				case token.IMPORT:
					for _, spec := range d.Specs {
						if v, ok := spec.(*ast.ImportSpec); ok {
							pkg.imports[v.Path.Value] = spec
						}
					}
				case token.CONST:
					filtered := &ast.GenDecl{Tok: token.CONST, Lparen: d.Lparen, Rparen: d.Rparen}
					for _, spec := range d.Specs {
						if v, ok := spec.(*ast.ValueSpec); ok {
							dup := false
							for _, n := range v.Names {
								if pkg.constNames[n.Name] {
									dup = true
									break
								}
							}
							if !dup {
								for _, n := range v.Names {
									pkg.constNames[n.Name] = true
								}
								filtered.Specs = append(filtered.Specs, spec)
							}
						}
					}
					if len(filtered.Specs) > 0 {
						pkg.consts = append(pkg.consts, filtered)
					}
				case token.TYPE:
					for _, spec := range d.Specs {
						if t, ok := spec.(*ast.TypeSpec); ok {
							pkg.types[t.Name.Name] = spec
						}
					}
				case token.VAR:
					for _, spec := range d.Specs {
						if v, ok := spec.(*ast.ValueSpec); ok {
							pkg.vars[v.Names[0].Name] = spec
						}
					}
				}
			case *ast.FuncDecl:
				key := funcKey(d)
				pkg.funcs[key] = d
				if recv := recvTypeName(d); recv != "" {
					pkg.typeMethods[recv] = append(pkg.typeMethods[recv], key)
				}
			}
		}
	}

	return pkg, nil
}

// computeUsedSymbols finds symbols from a package that are directly used
// by looking for SelectorExpr nodes like alias.Symbol.
func computeUsedSymbols(m *Merger, alias string) map[string]bool {
	used := make(map[string]bool)

	visitor := func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == alias {
				used[sel.Sel.Name] = true
			}
		}
		return true
	}

	for _, fn := range m.addedFunc {
		ast.Inspect(fn, visitor)
	}
	for _, spec := range m.addedVars {
		ast.Inspect(spec, visitor)
	}
	for _, decl := range m.constDecls {
		ast.Inspect(decl, visitor)
	}

	return used
}

// expandTypeMethods marks all methods of used types as used.
func expandTypeMethods(pkg *PackageSymbols, used map[string]bool) bool {
	added := false
	for typeName, methods := range pkg.typeMethods {
		if !used[typeName] {
			continue
		}
		for _, key := range methods {
			if !used[key] {
				used[key] = true
				added = true
			}
		}
	}
	return added
}

// expandTransitive expands the used set to include transitive dependencies
// within the package.
func expandTransitive(pkg *PackageSymbols, used map[string]bool) map[string]bool {
	allSymbols := make(map[string]bool)
	for name := range pkg.funcs {
		allSymbols[name] = true
	}
	for name := range pkg.types {
		allSymbols[name] = true
	}
	for name := range pkg.vars {
		allSymbols[name] = true
	}
	for name := range pkg.constNames {
		allSymbols[name] = true
	}

	expandTypeMethods(pkg, used)

	for {
		newUsed := make(map[string]bool)
		for name := range used {
			inspectNode := func(n ast.Node) bool {
				if ident, ok := n.(*ast.Ident); ok {
					if allSymbols[ident.Name] && !used[ident.Name] {
						newUsed[ident.Name] = true
					}
				}
				return true
			}
			if fn, ok := pkg.funcs[name]; ok {
				ast.Inspect(fn, inspectNode)
			}
			if spec, ok := pkg.types[name]; ok {
				ast.Inspect(spec, inspectNode)
			}
		}
		if len(newUsed) == 0 {
			break
		}
		for name := range newUsed {
			used[name] = true
		}
		expandTypeMethods(pkg, used)
	}

	return used
}

// rewriteExpr replaces alias.Symbol selector expressions with just Symbol.
func rewriteExpr(expr ast.Expr, alias string) ast.Expr {
	if expr == nil {
		return nil
	}

	if sel, ok := expr.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == alias {
			return sel.Sel
		}
	}

	switch e := expr.(type) {
	case *ast.CallExpr:
		e.Fun = rewriteExpr(e.Fun, alias)
		for i, arg := range e.Args {
			e.Args[i] = rewriteExpr(arg, alias)
		}
	case *ast.BinaryExpr:
		e.X = rewriteExpr(e.X, alias)
		e.Y = rewriteExpr(e.Y, alias)
	case *ast.UnaryExpr:
		e.X = rewriteExpr(e.X, alias)
	case *ast.ParenExpr:
		e.X = rewriteExpr(e.X, alias)
	case *ast.IndexExpr:
		e.X = rewriteExpr(e.X, alias)
		e.Index = rewriteExpr(e.Index, alias)
	case *ast.SliceExpr:
		e.X = rewriteExpr(e.X, alias)
		e.Low = rewriteExpr(e.Low, alias)
		e.High = rewriteExpr(e.High, alias)
		e.Max = rewriteExpr(e.Max, alias)
	case *ast.StarExpr:
		e.X = rewriteExpr(e.X, alias)
	case *ast.CompositeLit:
		e.Type = rewriteExpr(e.Type, alias)
		for i, elt := range e.Elts {
			e.Elts[i] = rewriteExpr(elt, alias)
		}
	case *ast.KeyValueExpr:
		e.Key = rewriteExpr(e.Key, alias)
		e.Value = rewriteExpr(e.Value, alias)
	case *ast.TypeAssertExpr:
		e.X = rewriteExpr(e.X, alias)
		e.Type = rewriteExpr(e.Type, alias)
	case *ast.SelectorExpr:
		e.X = rewriteExpr(e.X, alias)
	case *ast.ArrayType:
		e.Len = rewriteExpr(e.Len, alias)
		e.Elt = rewriteExpr(e.Elt, alias)
	case *ast.MapType:
		e.Key = rewriteExpr(e.Key, alias)
		e.Value = rewriteExpr(e.Value, alias)
	case *ast.ChanType:
		e.Value = rewriteExpr(e.Value, alias)
	case *ast.StructType:
		rewriteFieldList(e.Fields, alias)
	case *ast.InterfaceType:
		rewriteFieldList(e.Methods, alias)
	case *ast.FuncLit:
		rewriteFieldList(e.Type.Params, alias)
		rewriteFieldList(e.Type.Results, alias)
		if e.Body != nil {
			rewriteStmtList(e.Body.List, alias)
		}
	}

	return expr
}

func rewriteFieldList(fl *ast.FieldList, alias string) {
	if fl == nil {
		return
	}
	for _, field := range fl.List {
		field.Type = rewriteExpr(field.Type, alias)
	}
}

func rewriteStmt(stmt ast.Stmt, alias string) {
	if stmt == nil {
		return
	}

	switch s := stmt.(type) {
	case *ast.ExprStmt:
		s.X = rewriteExpr(s.X, alias)
	case *ast.AssignStmt:
		for i := range s.Lhs {
			s.Lhs[i] = rewriteExpr(s.Lhs[i], alias)
		}
		for i := range s.Rhs {
			s.Rhs[i] = rewriteExpr(s.Rhs[i], alias)
		}
	case *ast.ReturnStmt:
		for i := range s.Results {
			s.Results[i] = rewriteExpr(s.Results[i], alias)
		}
	case *ast.IfStmt:
		rewriteStmt(s.Init, alias)
		s.Cond = rewriteExpr(s.Cond, alias)
		rewriteStmt(s.Body, alias)
		rewriteStmt(s.Else, alias)
	case *ast.BlockStmt:
		if s != nil {
			rewriteStmtList(s.List, alias)
		}
	case *ast.ForStmt:
		rewriteStmt(s.Init, alias)
		s.Cond = rewriteExpr(s.Cond, alias)
		rewriteStmt(s.Post, alias)
		rewriteStmt(s.Body, alias)
	case *ast.RangeStmt:
		if s.Key != nil {
			s.Key = rewriteExpr(s.Key, alias)
		}
		if s.Value != nil {
			s.Value = rewriteExpr(s.Value, alias)
		}
		s.X = rewriteExpr(s.X, alias)
		rewriteStmt(s.Body, alias)
	case *ast.DeclStmt:
		if gen, ok := s.Decl.(*ast.GenDecl); ok {
			for _, spec := range gen.Specs {
				switch sp := spec.(type) {
				case *ast.ValueSpec:
					sp.Type = rewriteExpr(sp.Type, alias)
					for i := range sp.Values {
						sp.Values[i] = rewriteExpr(sp.Values[i], alias)
					}
				case *ast.TypeSpec:
					sp.Type = rewriteExpr(sp.Type, alias)
				}
			}
		}
	case *ast.SwitchStmt:
		rewriteStmt(s.Init, alias)
		s.Tag = rewriteExpr(s.Tag, alias)
		rewriteStmt(s.Body, alias)
	case *ast.TypeSwitchStmt:
		rewriteStmt(s.Init, alias)
		rewriteStmt(s.Assign, alias)
		rewriteStmt(s.Body, alias)
	case *ast.CaseClause:
		for i := range s.List {
			s.List[i] = rewriteExpr(s.List[i], alias)
		}
		rewriteStmtList(s.Body, alias)
	case *ast.SelectStmt:
		rewriteStmt(s.Body, alias)
	case *ast.CommClause:
		rewriteStmt(s.Comm, alias)
		rewriteStmtList(s.Body, alias)
	case *ast.SendStmt:
		s.Chan = rewriteExpr(s.Chan, alias)
		s.Value = rewriteExpr(s.Value, alias)
	case *ast.IncDecStmt:
		s.X = rewriteExpr(s.X, alias)
	case *ast.GoStmt:
		s.Call.Fun = rewriteExpr(s.Call.Fun, alias)
		for i := range s.Call.Args {
			s.Call.Args[i] = rewriteExpr(s.Call.Args[i], alias)
		}
	case *ast.DeferStmt:
		s.Call.Fun = rewriteExpr(s.Call.Fun, alias)
		for i := range s.Call.Args {
			s.Call.Args[i] = rewriteExpr(s.Call.Args[i], alias)
		}
	}
}

func rewriteStmtList(stmts []ast.Stmt, alias string) {
	for _, stmt := range stmts {
		rewriteStmt(stmt, alias)
	}
}

// rewriteSelectors rewrites all alias.Symbol references in source code.
func rewriteSelectors(m *Merger, alias string) {
	for _, fn := range m.addedFunc {
		if fn.Body != nil {
			rewriteStmtList(fn.Body.List, alias)
		}
		rewriteFieldList(fn.Recv, alias)
		rewriteFieldList(fn.Type.Params, alias)
		rewriteFieldList(fn.Type.Results, alias)
	}

	for _, spec := range m.addedVars {
		if v, ok := spec.(*ast.ValueSpec); ok {
			v.Type = rewriteExpr(v.Type, alias)
			for i := range v.Values {
				v.Values[i] = rewriteExpr(v.Values[i], alias)
			}
		}
	}

	for _, spec := range m.addedTypes {
		if t, ok := spec.(*ast.TypeSpec); ok {
			t.Type = rewriteExpr(t.Type, alias)
		}
	}
}

// hasFuncNamed checks if any function with the given name already exists.
func (m *Merger) hasFuncNamed(name string) bool {
	for _, fn := range m.addedFunc {
		if fn.Name.Name == name {
			return true
		}
	}
	return false
}

// resolveLocalImports processes all local package imports.
func (m *Merger) resolveLocalImports() error {
	if len(m.localImports) == 0 {
		return nil
	}

	for importPath, alias := range m.localImports {
		if alias == "_" {
			continue
		}

		relPath := strings.TrimPrefix(importPath, m.moduleName+"/")
		dir := filepath.Join(m.repoRoot, relPath)

		pkg, err := parsePackageSymbols(dir, alias)
		if err != nil {
			return fmt.Errorf("parsing package %s: %w", importPath, err)
		}

		used := computeUsedSymbols(m, alias)
		used = expandTransitive(pkg, used)

		rewriteSelectors(m, alias)

		m.inlinePackage(pkg, used)
	}

	// Second pass: rewrite any alias.XYZ refs that were introduced by inlined
	// packages (e.g. bot functions using game.XYZ inlined before game was rewritten).
	for _, alias := range m.localImports {
		if alias != "_" {
			rewriteSelectors(m, alias)
		}
	}

	return nil
}

// inlinePackage merges used symbols from a package into the merger.
func (m *Merger) inlinePackage(pkg *PackageSymbols, used map[string]bool) {
	for name, fn := range pkg.funcs {
		if !used[name] {
			continue
		}
		if !m.hasFuncNamed(name) {
			m.addedFunc[name] = fn
		}
	}

	for name, spec := range pkg.types {
		if !used[name] {
			continue
		}
		if _, exists := m.addedTypes[name]; !exists {
			m.addedTypes[name] = spec
		}
	}

	for name, spec := range pkg.vars {
		if !used[name] {
			continue
		}
		if _, exists := m.addedVars[name]; !exists {
			m.addedVars[name] = spec
		}
	}

	for _, block := range pkg.consts {
		blockUsed := false
		for _, spec := range block.Specs {
			if v, ok := spec.(*ast.ValueSpec); ok {
				for _, n := range v.Names {
					if used[n.Name] {
						blockUsed = true
						break
					}
				}
			}
			if blockUsed {
				break
			}
		}
		if blockUsed {
			for _, spec := range block.Specs {
				if v, ok := spec.(*ast.ValueSpec); ok {
					for _, n := range v.Names {
						m.constNames[n.Name] = true
					}
				}
			}
			m.constDecls = append(m.constDecls, block)
		}
	}

	for path, spec := range pkg.imports {
		importPath := strings.Trim(path, `"`)
		if m.moduleName != "" && strings.HasPrefix(importPath, m.moduleName) {
			continue // local package already inlined, don't re-import
		}
		if _, exists := m.addedImports[path]; !exists {
			m.addedImports[path] = spec
		}
	}
}
