package internal

import (
	"go/ast"
)

// eliminateDeadCode removes symbols not reachable from main() and init().
func (m *Merger) eliminateDeadCode() {
	// Build name-based index for functions (addedFunc keys may be position strings).
	funcByName := make(map[string]*ast.FuncDecl)   // funcName → decl
	funcByKey := make(map[string]*ast.FuncDecl)    // funcKey(decl) → decl
	funcKeyToMapKey := make(map[string]string)     // funcKey(decl) → addedFunc map key
	funcNameToMapKeys := make(map[string][]string) // funcName → addedFunc map keys
	for mapKey, fn := range m.addedFunc {
		fk := funcKey(fn)
		funcByKey[fk] = fn
		funcKeyToMapKey[fk] = mapKey
		funcByName[fn.Name.Name] = fn
		funcNameToMapKeys[fn.Name.Name] = append(funcNameToMapKeys[fn.Name.Name], mapKey)
	}

	// Collect all known symbol names (using actual names, not map keys).
	allSymbols := make(map[string]bool)
	for _, fn := range m.addedFunc {
		allSymbols[fn.Name.Name] = true
		fk := funcKey(fn)
		if fk != fn.Name.Name {
			allSymbols[fk] = true
		}
	}
	for name := range m.addedTypes {
		allSymbols[name] = true
	}
	for name := range m.addedVars {
		allSymbols[name] = true
	}
	for name := range m.constNames {
		allSymbols[name] = true
	}

	// Seed with main and init.
	used := make(map[string]bool)
	for name := range m.specialFunc {
		if fn, ok := m.addedFunc[name]; ok {
			used[name] = true
			collectRefs(fn, allSymbols, used)
		}
	}

	// Fixed-point expansion.
	changed := true
	for changed {
		changed = false
		snapshot := make([]string, 0, len(used))
		for name := range used {
			snapshot = append(snapshot, name)
		}
		for _, name := range snapshot {
			before := len(used)
			// Look up by funcKey first, then by name.
			if fn, ok := funcByKey[name]; ok {
				collectRefs(fn, allSymbols, used)
			} else if fn, ok := funcByName[name]; ok {
				collectRefs(fn, allSymbols, used)
			}
			if spec, ok := m.addedTypes[name]; ok {
				collectRefs(spec, allSymbols, used)
			}
			if spec, ok := m.addedVars[name]; ok {
				collectRefs(spec, allSymbols, used)
			}
			if len(used) > before {
				changed = true
			}
		}
		// Walk const blocks: if any const in a block is used, trace type refs.
		for _, block := range m.constDecls {
			blockHasUsed := false
			for _, spec := range block.Specs {
				if v, ok := spec.(*ast.ValueSpec); ok {
					for _, n := range v.Names {
						if used[n.Name] {
							blockHasUsed = true
							break
						}
					}
				}
				if blockHasUsed {
					break
				}
			}
			if blockHasUsed {
				before := len(used)
				collectRefs(block, allSymbols, used)
				if len(used) > before {
					changed = true
				}
			}
		}
		// Expand methods of used types.
		for _, fn := range m.addedFunc {
			recv := recvTypeName(fn)
			if recv != "" && used[recv] {
				fk := funcKey(fn)
				if !used[fk] {
					used[fk] = true
					used[fn.Name.Name] = true
					changed = true
				}
			}
		}
	}

	// Remove unused functions.
	for mapKey, fn := range m.addedFunc {
		if _, special := m.specialFunc[mapKey]; special {
			continue
		}
		fk := funcKey(fn)
		if !used[fk] && !used[fn.Name.Name] {
			delete(m.addedFunc, mapKey)
		}
	}

	// Remove unused types.
	for name := range m.addedTypes {
		if !used[name] {
			delete(m.addedTypes, name)
		}
	}

	// Remove unused vars.
	for name := range m.addedVars {
		if !used[name] {
			delete(m.addedVars, name)
		}
	}

	// Remove unused const blocks.
	var filtered []*ast.GenDecl
	for _, block := range m.constDecls {
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
			filtered = append(filtered, block)
		}
	}
	m.constDecls = filtered
}

// collectRefs walks an AST node and adds any referenced symbols to used.
func collectRefs(node ast.Node, allSymbols, used map[string]bool) {
	ast.Inspect(node, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok {
			if allSymbols[ident.Name] && !used[ident.Name] {
				used[ident.Name] = true
			}
		}
		return true
	})
}
