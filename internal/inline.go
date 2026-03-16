package internal

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
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
func funcKey(fn *ast.FuncDecl) string {
	recv := recvTypeName(fn)
	if recv == "" {
		return fn.Name.Name
	}
	return recv + "." + fn.Name.Name
}

// PackageSymbols holds all declarations from a parsed package.
type PackageSymbols struct {
	alias        string
	funcs        map[string]*ast.FuncDecl
	types        map[string]ast.Spec
	vars         map[string]ast.Spec
	consts       []*ast.GenDecl
	constNames   map[string]bool
	imports      map[string]ast.Spec
	typeMethods  map[string][]string
	localImports map[string]string // import path → alias (for sub-package deps)
}

func parsePackageSymbols(dir, alias, moduleName string) (*PackageSymbols, error) {
	pkg := &PackageSymbols{
		alias:        alias,
		funcs:        make(map[string]*ast.FuncDecl),
		types:        make(map[string]ast.Spec),
		vars:         make(map[string]ast.Spec),
		constNames:   make(map[string]bool),
		imports:      make(map[string]ast.Spec),
		typeMethods:  make(map[string][]string),
		localImports: make(map[string]string),
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
							impPath := strings.Trim(v.Path.Value, `"`)
							if moduleName != "" && strings.HasPrefix(impPath, moduleName) {
								localAlias := filepath.Base(impPath)
								if v.Name != nil {
									localAlias = v.Name.Name
								}
								if localAlias != "_" {
									pkg.localImports[impPath] = localAlias
								}
							} else {
								pkg.imports[v.Path.Value] = spec
							}
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

// expandTransitive expands the used set to include transitive dependencies.
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
			if spec, ok := pkg.vars[name]; ok {
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

// rewriteExprPrefix replaces alias.Symbol selector expressions with prefixSymbol.
// When prefix is "", it behaves like the old rewriteExpr (drops alias, keeps symbol name).
func rewriteExprPrefix(expr ast.Expr, alias, prefix string) ast.Expr {
	if expr == nil {
		return nil
	}

	if sel, ok := expr.(*ast.SelectorExpr); ok {
		if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == alias {
			return ast.NewIdent(prefix + sel.Sel.Name)
		}
	}

	switch e := expr.(type) {
	case *ast.CallExpr:
		e.Fun = rewriteExprPrefix(e.Fun, alias, prefix)
		for i, arg := range e.Args {
			e.Args[i] = rewriteExprPrefix(arg, alias, prefix)
		}
	case *ast.BinaryExpr:
		e.X = rewriteExprPrefix(e.X, alias, prefix)
		e.Y = rewriteExprPrefix(e.Y, alias, prefix)
	case *ast.UnaryExpr:
		e.X = rewriteExprPrefix(e.X, alias, prefix)
	case *ast.ParenExpr:
		e.X = rewriteExprPrefix(e.X, alias, prefix)
	case *ast.IndexExpr:
		e.X = rewriteExprPrefix(e.X, alias, prefix)
		e.Index = rewriteExprPrefix(e.Index, alias, prefix)
	case *ast.SliceExpr:
		e.X = rewriteExprPrefix(e.X, alias, prefix)
		e.Low = rewriteExprPrefix(e.Low, alias, prefix)
		e.High = rewriteExprPrefix(e.High, alias, prefix)
		e.Max = rewriteExprPrefix(e.Max, alias, prefix)
	case *ast.StarExpr:
		e.X = rewriteExprPrefix(e.X, alias, prefix)
	case *ast.CompositeLit:
		e.Type = rewriteExprPrefix(e.Type, alias, prefix)
		for i, elt := range e.Elts {
			e.Elts[i] = rewriteExprPrefix(elt, alias, prefix)
		}
	case *ast.KeyValueExpr:
		e.Key = rewriteExprPrefix(e.Key, alias, prefix)
		e.Value = rewriteExprPrefix(e.Value, alias, prefix)
	case *ast.TypeAssertExpr:
		e.X = rewriteExprPrefix(e.X, alias, prefix)
		e.Type = rewriteExprPrefix(e.Type, alias, prefix)
	case *ast.SelectorExpr:
		e.X = rewriteExprPrefix(e.X, alias, prefix)
	case *ast.ArrayType:
		e.Len = rewriteExprPrefix(e.Len, alias, prefix)
		e.Elt = rewriteExprPrefix(e.Elt, alias, prefix)
	case *ast.MapType:
		e.Key = rewriteExprPrefix(e.Key, alias, prefix)
		e.Value = rewriteExprPrefix(e.Value, alias, prefix)
	case *ast.ChanType:
		e.Value = rewriteExprPrefix(e.Value, alias, prefix)
	case *ast.StructType:
		rewriteFieldListPrefix(e.Fields, alias, prefix)
	case *ast.InterfaceType:
		rewriteFieldListPrefix(e.Methods, alias, prefix)
	case *ast.FuncLit:
		rewriteFieldListPrefix(e.Type.Params, alias, prefix)
		rewriteFieldListPrefix(e.Type.Results, alias, prefix)
		if e.Body != nil {
			rewriteStmtListPrefix(e.Body.List, alias, prefix)
		}
	}

	return expr
}

func rewriteFieldListPrefix(fl *ast.FieldList, alias, prefix string) {
	if fl == nil {
		return
	}
	for _, field := range fl.List {
		field.Type = rewriteExprPrefix(field.Type, alias, prefix)
	}
}

func rewriteStmtPrefix(stmt ast.Stmt, alias, prefix string) {
	if stmt == nil {
		return
	}

	switch s := stmt.(type) {
	case *ast.ExprStmt:
		s.X = rewriteExprPrefix(s.X, alias, prefix)
	case *ast.AssignStmt:
		for i := range s.Lhs {
			s.Lhs[i] = rewriteExprPrefix(s.Lhs[i], alias, prefix)
		}
		for i := range s.Rhs {
			s.Rhs[i] = rewriteExprPrefix(s.Rhs[i], alias, prefix)
		}
	case *ast.ReturnStmt:
		for i := range s.Results {
			s.Results[i] = rewriteExprPrefix(s.Results[i], alias, prefix)
		}
	case *ast.IfStmt:
		rewriteStmtPrefix(s.Init, alias, prefix)
		s.Cond = rewriteExprPrefix(s.Cond, alias, prefix)
		rewriteStmtPrefix(s.Body, alias, prefix)
		rewriteStmtPrefix(s.Else, alias, prefix)
	case *ast.BlockStmt:
		if s != nil {
			rewriteStmtListPrefix(s.List, alias, prefix)
		}
	case *ast.ForStmt:
		rewriteStmtPrefix(s.Init, alias, prefix)
		s.Cond = rewriteExprPrefix(s.Cond, alias, prefix)
		rewriteStmtPrefix(s.Post, alias, prefix)
		rewriteStmtPrefix(s.Body, alias, prefix)
	case *ast.RangeStmt:
		if s.Key != nil {
			s.Key = rewriteExprPrefix(s.Key, alias, prefix)
		}
		if s.Value != nil {
			s.Value = rewriteExprPrefix(s.Value, alias, prefix)
		}
		s.X = rewriteExprPrefix(s.X, alias, prefix)
		rewriteStmtPrefix(s.Body, alias, prefix)
	case *ast.DeclStmt:
		if gen, ok := s.Decl.(*ast.GenDecl); ok {
			for _, spec := range gen.Specs {
				switch sp := spec.(type) {
				case *ast.ValueSpec:
					sp.Type = rewriteExprPrefix(sp.Type, alias, prefix)
					for i := range sp.Values {
						sp.Values[i] = rewriteExprPrefix(sp.Values[i], alias, prefix)
					}
				case *ast.TypeSpec:
					sp.Type = rewriteExprPrefix(sp.Type, alias, prefix)
				}
			}
		}
	case *ast.SwitchStmt:
		rewriteStmtPrefix(s.Init, alias, prefix)
		s.Tag = rewriteExprPrefix(s.Tag, alias, prefix)
		rewriteStmtPrefix(s.Body, alias, prefix)
	case *ast.TypeSwitchStmt:
		rewriteStmtPrefix(s.Init, alias, prefix)
		rewriteStmtPrefix(s.Assign, alias, prefix)
		rewriteStmtPrefix(s.Body, alias, prefix)
	case *ast.CaseClause:
		for i := range s.List {
			s.List[i] = rewriteExprPrefix(s.List[i], alias, prefix)
		}
		rewriteStmtListPrefix(s.Body, alias, prefix)
	case *ast.SelectStmt:
		rewriteStmtPrefix(s.Body, alias, prefix)
	case *ast.CommClause:
		rewriteStmtPrefix(s.Comm, alias, prefix)
		rewriteStmtListPrefix(s.Body, alias, prefix)
	case *ast.SendStmt:
		s.Chan = rewriteExprPrefix(s.Chan, alias, prefix)
		s.Value = rewriteExprPrefix(s.Value, alias, prefix)
	case *ast.IncDecStmt:
		s.X = rewriteExprPrefix(s.X, alias, prefix)
	case *ast.GoStmt:
		s.Call.Fun = rewriteExprPrefix(s.Call.Fun, alias, prefix)
		for i := range s.Call.Args {
			s.Call.Args[i] = rewriteExprPrefix(s.Call.Args[i], alias, prefix)
		}
	case *ast.DeferStmt:
		s.Call.Fun = rewriteExprPrefix(s.Call.Fun, alias, prefix)
		for i := range s.Call.Args {
			s.Call.Args[i] = rewriteExprPrefix(s.Call.Args[i], alias, prefix)
		}
	}
}

func rewriteStmtListPrefix(stmts []ast.Stmt, alias, prefix string) {
	for _, stmt := range stmts {
		rewriteStmtPrefix(stmt, alias, prefix)
	}
}

// rewriteSelectorsPrefix rewrites all alias.Symbol references in the merger's code
// to prefixSymbol.
func rewriteSelectorsPrefix(m *Merger, alias, prefix string) {
	for _, fn := range m.addedFunc {
		if fn.Body != nil {
			rewriteStmtListPrefix(fn.Body.List, alias, prefix)
		}
		rewriteFieldListPrefix(fn.Recv, alias, prefix)
		rewriteFieldListPrefix(fn.Type.Params, alias, prefix)
		rewriteFieldListPrefix(fn.Type.Results, alias, prefix)
	}

	for _, spec := range m.addedVars {
		if v, ok := spec.(*ast.ValueSpec); ok {
			v.Type = rewriteExprPrefix(v.Type, alias, prefix)
			for i := range v.Values {
				v.Values[i] = rewriteExprPrefix(v.Values[i], alias, prefix)
			}
		}
	}

	for _, spec := range m.addedTypes {
		if t, ok := spec.(*ast.TypeSpec); ok {
			t.Type = rewriteExprPrefix(t.Type, alias, prefix)
		}
	}
}

// renameIdent renames a single identifier if it appears in the map.
func renameIdent(ident *ast.Ident, renames map[string]string) {
	if ident == nil {
		return
	}
	if newName, found := renames[ident.Name]; found {
		ident.Name = newName
	}
}

// renameExpr renames identifiers in an expression (type references, values, etc.).
func renameExpr(expr ast.Expr, renames map[string]string) {
	if expr == nil {
		return
	}
	switch e := expr.(type) {
	case *ast.Ident:
		renameIdent(e, renames)
	case *ast.StarExpr:
		renameExpr(e.X, renames)
	case *ast.ArrayType:
		renameExpr(e.Elt, renames)
		renameExpr(e.Len, renames)
	case *ast.MapType:
		renameExpr(e.Key, renames)
		renameExpr(e.Value, renames)
	case *ast.ChanType:
		renameExpr(e.Value, renames)
	case *ast.FuncType:
		renameFieldListTypes(e.Params, renames)
		renameFieldListTypes(e.Results, renames)
	case *ast.SelectorExpr:
		renameExpr(e.X, renames)
		// Do NOT rename e.Sel — it's a field/method selector
	case *ast.IndexExpr:
		renameExpr(e.X, renames)
		renameExpr(e.Index, renames)
	case *ast.IndexListExpr:
		renameExpr(e.X, renames)
		for _, idx := range e.Indices {
			renameExpr(idx, renames)
		}
	case *ast.CallExpr:
		renameExpr(e.Fun, renames)
		for _, arg := range e.Args {
			renameExpr(arg, renames)
		}
	case *ast.UnaryExpr:
		renameExpr(e.X, renames)
	case *ast.BinaryExpr:
		renameExpr(e.X, renames)
		renameExpr(e.Y, renames)
	case *ast.ParenExpr:
		renameExpr(e.X, renames)
	case *ast.TypeAssertExpr:
		renameExpr(e.X, renames)
		renameExpr(e.Type, renames)
	case *ast.CompositeLit:
		renameExpr(e.Type, renames)
		for _, elt := range e.Elts {
			renameExpr(elt, renames)
		}
	case *ast.KeyValueExpr:
		// Key can be a struct field name, map key, or array index constant.
		// We rename keys because array/map index constants must be renamed (e.g.,
		// DirUp in [4][2]int{DirUp: ...}). Struct field names are generally not
		// package-level symbols, so they won't match the renames map. Edge case:
		// if a field name coincidentally matches a package symbol, it would be
		// incorrectly renamed — fixing this requires type info we don't have.
		renameExpr(e.Key, renames)
		renameExpr(e.Value, renames)
	case *ast.SliceExpr:
		renameExpr(e.X, renames)
		renameExpr(e.Low, renames)
		renameExpr(e.High, renames)
		renameExpr(e.Max, renames)
	case *ast.FuncLit:
		renameExpr(e.Type, renames)
		renameStmtList(e.Body.List, renames)
	case *ast.Ellipsis:
		renameExpr(e.Elt, renames)
	case *ast.InterfaceType:
		renameFieldListTypes(e.Methods, renames)
	case *ast.StructType:
		renameFieldListTypes(e.Fields, renames)
	}
}

// renameFieldListTypes renames type references in a field list, leaving field names untouched.
func renameFieldListTypes(fl *ast.FieldList, renames map[string]string) {
	if fl == nil {
		return
	}
	for _, field := range fl.List {
		renameExpr(field.Type, renames)
	}
}

// renameStmt renames identifiers in a statement.
func renameStmt(stmt ast.Stmt, renames map[string]string) {
	if stmt == nil {
		return
	}
	switch s := stmt.(type) {
	case *ast.ExprStmt:
		renameExpr(s.X, renames)
	case *ast.AssignStmt:
		for _, expr := range s.Lhs {
			renameExpr(expr, renames)
		}
		for _, expr := range s.Rhs {
			renameExpr(expr, renames)
		}
	case *ast.ReturnStmt:
		for _, expr := range s.Results {
			renameExpr(expr, renames)
		}
	case *ast.IfStmt:
		renameStmt(s.Init, renames)
		renameExpr(s.Cond, renames)
		renameStmt(s.Body, renames)
		renameStmt(s.Else, renames)
	case *ast.ForStmt:
		renameStmt(s.Init, renames)
		renameExpr(s.Cond, renames)
		renameStmt(s.Post, renames)
		renameStmt(s.Body, renames)
	case *ast.RangeStmt:
		renameExpr(s.Key, renames)
		renameExpr(s.Value, renames)
		renameExpr(s.X, renames)
		renameStmt(s.Body, renames)
	case *ast.BlockStmt:
		renameStmtList(s.List, renames)
	case *ast.SwitchStmt:
		renameStmt(s.Init, renames)
		renameExpr(s.Tag, renames)
		renameStmtList(s.Body.List, renames)
	case *ast.TypeSwitchStmt:
		renameStmt(s.Init, renames)
		renameStmt(s.Assign, renames)
		renameStmtList(s.Body.List, renames)
	case *ast.CaseClause:
		for _, expr := range s.List {
			renameExpr(expr, renames)
		}
		renameStmtList(s.Body, renames)
	case *ast.SelectStmt:
		renameStmtList(s.Body.List, renames)
	case *ast.CommClause:
		renameStmt(s.Comm, renames)
		renameStmtList(s.Body, renames)
	case *ast.SendStmt:
		renameExpr(s.Chan, renames)
		renameExpr(s.Value, renames)
	case *ast.IncDecStmt:
		renameExpr(s.X, renames)
	case *ast.DeclStmt:
		renameDecl(s.Decl, renames)
	case *ast.GoStmt:
		renameExpr(s.Call, renames)
	case *ast.DeferStmt:
		renameExpr(s.Call, renames)
	case *ast.BranchStmt:
		// label — don't rename
	case *ast.LabeledStmt:
		// don't rename the label
		renameStmt(s.Stmt, renames)
	}
}

// renameStmtList renames identifiers in a list of statements.
func renameStmtList(stmts []ast.Stmt, renames map[string]string) {
	for _, stmt := range stmts {
		renameStmt(stmt, renames)
	}
}

// renameDecl renames type references in a local declaration (DeclStmt inside functions).
// Does NOT rename declared names — local types/vars are not package symbols.
func renameDecl(decl ast.Decl, renames map[string]string) {
	switch d := decl.(type) {
	case *ast.GenDecl:
		for _, spec := range d.Specs {
			renameSpecTypes(spec, renames)
		}
	}
}

// renameSpec renames both the declared name and type references in a spec.
// Used for top-level package symbols.
func renameSpec(spec ast.Spec, renames map[string]string) {
	switch s := spec.(type) {
	case *ast.TypeSpec:
		renameIdent(s.Name, renames)
		renameExpr(s.Type, renames)
	case *ast.ValueSpec:
		for _, name := range s.Names {
			renameIdent(name, renames)
		}
		renameExpr(s.Type, renames)
		for _, val := range s.Values {
			renameExpr(val, renames)
		}
	}
}

// renameSpecTypes renames only type references in a spec, leaving declared names intact.
// Used for local declarations inside function bodies.
func renameSpecTypes(spec ast.Spec, renames map[string]string) {
	switch s := spec.(type) {
	case *ast.TypeSpec:
		renameExpr(s.Type, renames)
	case *ast.ValueSpec:
		renameExpr(s.Type, renames)
		for _, val := range s.Values {
			renameExpr(val, renames)
		}
	}
}

// renameIdents walks an AST node and renames identifiers according to the map,
// carefully distinguishing between type references and field/label names.
func renameIdents(node ast.Node, renames map[string]string) {
	switch n := node.(type) {
	case *ast.FuncDecl:
		// Rename the function name
		renameIdent(n.Name, renames)
		// Rename receiver types (not names)
		renameFieldListTypes(n.Recv, renames)
		// Rename parameter and result types (not names)
		renameFieldListTypes(n.Type.Params, renames)
		renameFieldListTypes(n.Type.Results, renames)
		// Rename body
		if n.Body != nil {
			renameStmtList(n.Body.List, renames)
		}
	case *ast.TypeSpec:
		renameSpec(n, renames)
	case ast.Spec:
		renameSpec(n, renames)
	case *ast.GenDecl:
		// Top-level GenDecl: rename both names and types (package symbols).
		for _, spec := range n.Specs {
			renameSpec(spec, renames)
		}
	}
}

// renamePackageSymbols renames all symbols in a package with the given prefix
// and returns the rename map (old name → new name).
func renamePackageSymbols(pkg *PackageSymbols, prefix string) map[string]string {
	if prefix == "" {
		return nil
	}

	renames := make(map[string]string)

	// Collect all symbol names that need renaming.
	for name := range pkg.funcs {
		parts := strings.SplitN(name, ".", 2)
		if len(parts) == 2 {
			// Method: only rename RecvType — method names are scoped by receiver
			// and call sites (obj.Method()) can't be rewritten without type info.
			renames[parts[0]] = prefix + parts[0]
		} else {
			renames[name] = prefix + name
		}
	}
	for name := range pkg.types {
		renames[name] = prefix + name
	}
	for name := range pkg.vars {
		renames[name] = prefix + name
	}
	for name := range pkg.constNames {
		renames[name] = prefix + name
	}

	// Rename all AST nodes within the package.
	for _, fn := range pkg.funcs {
		renameIdents(fn, renames)
	}
	for _, spec := range pkg.types {
		renameIdents(spec, renames)
	}
	for _, spec := range pkg.vars {
		renameIdents(spec, renames)
	}
	for _, block := range pkg.consts {
		renameIdents(block, renames)
	}

	// Rebuild maps with new keys.
	newFuncs := make(map[string]*ast.FuncDecl, len(pkg.funcs))
	for _, fn := range pkg.funcs {
		newFuncs[funcKey(fn)] = fn
	}
	pkg.funcs = newFuncs

	newTypes := make(map[string]ast.Spec, len(pkg.types))
	for _, spec := range pkg.types {
		if t, ok := spec.(*ast.TypeSpec); ok {
			newTypes[t.Name.Name] = spec
		}
	}
	pkg.types = newTypes

	newVars := make(map[string]ast.Spec, len(pkg.vars))
	for _, spec := range pkg.vars {
		if v, ok := spec.(*ast.ValueSpec); ok {
			newVars[v.Names[0].Name] = spec
		}
	}
	pkg.vars = newVars

	newConstNames := make(map[string]bool, len(pkg.constNames))
	for oldName := range pkg.constNames {
		if newName, ok := renames[oldName]; ok {
			newConstNames[newName] = true
		} else {
			newConstNames[oldName] = true
		}
	}
	pkg.constNames = newConstNames

	newTypeMethods := make(map[string][]string)
	for typeName, methods := range pkg.typeMethods {
		newTypeName := typeName
		if r, ok := renames[typeName]; ok {
			newTypeName = r
		}
		newMethods := make([]string, len(methods))
		for i, key := range methods {
			parts := strings.SplitN(key, ".", 2)
			if len(parts) == 2 {
				t := parts[0]
				m := parts[1]
				if r, ok := renames[t]; ok {
					t = r
				}
				if r, ok := renames[m]; ok {
					m = r
				}
				newMethods[i] = t + "." + m
			} else {
				if r, ok := renames[key]; ok {
					newMethods[i] = r
				} else {
					newMethods[i] = key
				}
			}
		}
		newTypeMethods[newTypeName] = newMethods
	}
	pkg.typeMethods = newTypeMethods

	return renames
}

// buildPrefixMap assigns a prefix letter (a, b, c, ...) to each local import.
func buildPrefixMap(localImports map[string]string) map[string]string {
	prefixMap := make(map[string]string, len(localImports))

	// Sort import paths for deterministic prefix assignment.
	paths := make([]string, 0, len(localImports))
	for p := range localImports {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for i, p := range paths {
		prefixMap[p] = string(rune('a' + i))
	}
	return prefixMap
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

// resolveLocalImports processes all local package imports with prefix renaming.
func (m *Merger) resolveLocalImports() error {
	if len(m.localImports) == 0 {
		return nil
	}

	prefixMap := buildPrefixMap(m.localImports)

	// Sort import paths for deterministic processing order.
	importPaths := make([]string, 0, len(m.localImports))
	for p := range m.localImports {
		importPaths = append(importPaths, p)
	}
	sort.Strings(importPaths)

	// First pass: parse all packages, compute used symbols, rename with prefix,
	// rewrite selectors in main code, and inline.
	parsedPkgs := make(map[string]*PackageSymbols)
	for _, importPath := range importPaths {
		alias := m.localImports[importPath]
		if alias == "_" {
			continue
		}

		prefix := prefixMap[importPath]

		relPath := strings.TrimPrefix(importPath, m.moduleName+"/")
		dir := filepath.Join(m.repoRoot, relPath)

		pkg, err := parsePackageSymbols(dir, alias, m.moduleName)
		if err != nil {
			return fmt.Errorf("parsing package %s: %w", importPath, err)
		}

		// Resolve sub-package local imports (e.g., gamma imports epsilon).
		if err := m.resolveSubPkgImports(pkg); err != nil {
			return err
		}

		used := computeUsedSymbols(m, alias)
		used = expandTransitive(pkg, used)

		// Rewrite cross-package selectors within the package itself.
		// e.g., bot's code referencing game.Point → aPoint
		for _, otherPath := range importPaths {
			otherAlias := m.localImports[otherPath]
			if otherAlias == "_" || otherPath == importPath {
				continue
			}
			otherPrefix := prefixMap[otherPath]
			rewritePkgSelectors(pkg, otherAlias, otherPrefix)
		}

		// Rename symbols within the package with its own prefix.
		renamePackageSymbols(pkg, prefix)

		// Rewrite selectors in main code: alias.Symbol → prefixSymbol
		rewriteSelectorsPrefix(m, alias, prefix)

		// Inline the renamed package symbols.
		m.inlinePackage(pkg, used, prefix)

		parsedPkgs[importPath] = pkg
	}

	// Second pass: rewrite any remaining alias.XYZ refs introduced by inlined packages.
	for _, importPath := range importPaths {
		alias := m.localImports[importPath]
		if alias != "_" {
			prefix := prefixMap[importPath]
			rewriteSelectorsPrefix(m, alias, prefix)
		}
	}

	// Third pass: resolve cross-package transitive dependencies.
	// e.g., gamma uses alpha.ExampleVar → rewritten to aExampleVar,
	// but alpha didn't inline ExampleVar because it wasn't referenced from main.
	m.resolveCrossPkgDeps(parsedPkgs)

	return nil
}

// rewritePkgSelectors rewrites selector expressions within a package's own AST.
// e.g., inside bot package: game.Point → aPoint
func rewritePkgSelectors(pkg *PackageSymbols, alias, prefix string) {
	for _, fn := range pkg.funcs {
		if fn.Body != nil {
			rewriteStmtListPrefix(fn.Body.List, alias, prefix)
		}
		rewriteFieldListPrefix(fn.Recv, alias, prefix)
		rewriteFieldListPrefix(fn.Type.Params, alias, prefix)
		rewriteFieldListPrefix(fn.Type.Results, alias, prefix)
	}
	for _, spec := range pkg.vars {
		if v, ok := spec.(*ast.ValueSpec); ok {
			v.Type = rewriteExprPrefix(v.Type, alias, prefix)
			for i := range v.Values {
				v.Values[i] = rewriteExprPrefix(v.Values[i], alias, prefix)
			}
		}
	}
	for _, spec := range pkg.types {
		if t, ok := spec.(*ast.TypeSpec); ok {
			t.Type = rewriteExprPrefix(t.Type, alias, prefix)
		}
	}
}

// inlinePackage merges used symbols from a package into the merger.
// The prefix parameter is used to match prefixed symbol names against the used set.
func (m *Merger) inlinePackage(pkg *PackageSymbols, used map[string]bool, prefix string) {
	for name, fn := range pkg.funcs {
		// Check if the original (unprefixed) name is in the used set.
		origName := name
		if prefix != "" {
			origName = strings.TrimPrefix(name, prefix)
			// For methods: aRecvType.MethodName → check RecvType.MethodName
			parts := strings.SplitN(name, ".", 2)
			if len(parts) == 2 {
				origName = strings.TrimPrefix(parts[0], prefix) + "." + parts[1]
			}
		}
		if !used[origName] {
			continue
		}
		if !m.hasFuncNamed(fn.Name.Name) {
			m.addedFunc[name] = fn
		}
	}

	for name, spec := range pkg.types {
		origName := name
		if prefix != "" {
			origName = strings.TrimPrefix(name, prefix)
		}
		if !used[origName] {
			continue
		}
		if _, exists := m.addedTypes[name]; !exists {
			m.addedTypes[name] = spec
		}
	}

	for name, spec := range pkg.vars {
		origName := name
		if prefix != "" {
			origName = strings.TrimPrefix(name, prefix)
		}
		if !used[origName] {
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
					origName := n.Name
					if prefix != "" {
						origName = strings.TrimPrefix(n.Name, prefix)
					}
					if used[origName] {
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
			continue
		}
		if _, exists := m.addedImports[path]; !exists {
			m.addedImports[path] = spec
		}
	}
}

// resolveSubPkgImports inlines a package's own local imports that are NOT
// already handled by the main resolveLocalImports (sibling packages).
// e.g., gamma imports epsilon → epsilon.ExampleScale is inlined into gamma.
func (m *Merger) resolveSubPkgImports(pkg *PackageSymbols) error {
	if len(pkg.localImports) == 0 {
		return nil
	}

	for subPath, subAlias := range pkg.localImports {
		// Skip imports already handled as sibling packages by the main resolver.
		if _, isSibling := m.localImports[subPath]; isSibling {
			continue
		}
		relPath := strings.TrimPrefix(subPath, m.moduleName+"/")
		dir := filepath.Join(m.repoRoot, relPath)

		subPkg, err := parsePackageSymbols(dir, subAlias, m.moduleName)
		if err != nil {
			return fmt.Errorf("parsing sub-package %s: %w", subPath, err)
		}

		// Recursively resolve sub-sub-packages.
		if err := m.resolveSubPkgImports(subPkg); err != nil {
			return err
		}

		// Rewrite subAlias.Symbol → Symbol (inline directly, no prefix).
		for _, fn := range pkg.funcs {
			if fn.Body != nil {
				rewriteStmtListPrefix(fn.Body.List, subAlias, "")
			}
			rewriteFieldListPrefix(fn.Recv, subAlias, "")
			rewriteFieldListPrefix(fn.Type.Params, subAlias, "")
			rewriteFieldListPrefix(fn.Type.Results, subAlias, "")
		}
		for _, spec := range pkg.vars {
			if v, ok := spec.(*ast.ValueSpec); ok {
				v.Type = rewriteExprPrefix(v.Type, subAlias, "")
				for i := range v.Values {
					v.Values[i] = rewriteExprPrefix(v.Values[i], subAlias, "")
				}
			}
		}
		for _, spec := range pkg.types {
			if t, ok := spec.(*ast.TypeSpec); ok {
				t.Type = rewriteExprPrefix(t.Type, subAlias, "")
			}
		}

		// Merge sub-package symbols into parent package.
		for key, fn := range subPkg.funcs {
			if _, exists := pkg.funcs[key]; !exists {
				pkg.funcs[key] = fn
				if recv := recvTypeName(fn); recv != "" {
					pkg.typeMethods[recv] = append(pkg.typeMethods[recv], key)
				}
			}
		}
		for name, spec := range subPkg.types {
			if _, exists := pkg.types[name]; !exists {
				pkg.types[name] = spec
			}
		}
		for name, spec := range subPkg.vars {
			if _, exists := pkg.vars[name]; !exists {
				pkg.vars[name] = spec
			}
		}
		for _, block := range subPkg.consts {
			for _, spec := range block.Specs {
				if v, ok := spec.(*ast.ValueSpec); ok {
					for _, n := range v.Names {
						pkg.constNames[n.Name] = true
					}
				}
			}
			pkg.consts = append(pkg.consts, block)
		}
		for path, spec := range subPkg.imports {
			if _, exists := pkg.imports[path]; !exists {
				pkg.imports[path] = spec
			}
		}
	}

	return nil
}

// resolveCrossPkgDeps inlines symbols from already-processed packages that
// are transitively referenced by newly-inlined code (e.g., gamma uses
// alpha.ExampleVar which became aExampleVar after rewriting).
func (m *Merger) resolveCrossPkgDeps(parsedPkgs map[string]*PackageSymbols) {
	// Build index: prefixed symbol name → package symbol (vars, types, funcs, consts).
	type pkgSymRef struct {
		pkg  *PackageSymbols
		kind string // "var", "type", "func", "const"
	}
	symIndex := make(map[string]pkgSymRef)
	for _, pkg := range parsedPkgs {
		for name := range pkg.vars {
			symIndex[name] = pkgSymRef{pkg, "var"}
		}
		for name := range pkg.types {
			symIndex[name] = pkgSymRef{pkg, "type"}
		}
		for name := range pkg.funcs {
			symIndex[name] = pkgSymRef{pkg, "func"}
		}
		for name := range pkg.constNames {
			symIndex[name] = pkgSymRef{pkg, "const"}
		}
	}

	for {
		// Collect all currently declared symbols.
		declared := make(map[string]bool)
		for name := range m.addedVars {
			declared[name] = true
		}
		for name := range m.addedTypes {
			declared[name] = true
		}
		for _, fn := range m.addedFunc {
			declared[fn.Name.Name] = true
		}
		for name := range m.constNames {
			declared[name] = true
		}

		// Walk merged code for undeclared identifiers matching package symbols.
		missing := make(map[string]bool)
		visitor := func(n ast.Node) bool {
			if ident, ok := n.(*ast.Ident); ok {
				if !declared[ident.Name] {
					if _, ok := symIndex[ident.Name]; ok {
						missing[ident.Name] = true
					}
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
		for _, spec := range m.addedTypes {
			ast.Inspect(spec, visitor)
		}
		for _, block := range m.constDecls {
			ast.Inspect(block, visitor)
		}

		if len(missing) == 0 {
			break
		}

		// Inline the missing symbols.
		for name := range missing {
			ref := symIndex[name]
			switch ref.kind {
			case "var":
				if spec, ok := ref.pkg.vars[name]; ok {
					m.addedVars[name] = spec
				}
			case "type":
				if spec, ok := ref.pkg.types[name]; ok {
					m.addedTypes[name] = spec
				}
			case "func":
				if fn, ok := ref.pkg.funcs[name]; ok {
					m.addedFunc[name] = fn
				}
			case "const":
				// Skip if already added as part of a block by a previous iteration.
				if m.constNames[name] {
					break
				}
				// Find and inline the entire const block containing this name.
				for _, block := range ref.pkg.consts {
					for _, spec := range block.Specs {
						if v, ok := spec.(*ast.ValueSpec); ok {
							for _, n := range v.Names {
								if n.Name == name {
									for _, s := range block.Specs {
										if vs, ok := s.(*ast.ValueSpec); ok {
											for _, cn := range vs.Names {
												m.constNames[cn.Name] = true
											}
										}
									}
									m.constDecls = append(m.constDecls, block)
									goto nextMissing
								}
							}
						}
					}
				}
			nextMissing:
			}
		}
	}
}
