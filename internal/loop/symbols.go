package loop

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// simbolosExportados recoge los símbolos exportados de nivel superior en
// los archivos Go cambiados desde base, vía go/ast. Devuelve nil (null en
// JSON) cuando el diff no trae archivos Go: la adenda A.2 manda null si
// el lenguaje no se soporta.
//
// Aproximación honesta: reporta los exportados de los archivos del diff,
// no solo los añadidos línea a línea. Es señal suficiente para el cruce
// semántico de §6.9 y no inventa nada.
func simbolosExportados(roomPath, base string) (*[]string, error) {
	files, err := changedVsBase(roomPath, base)
	if err != nil {
		return nil, err
	}
	var goFiles []string
	for _, f := range files {
		if strings.HasSuffix(f, ".go") {
			goFiles = append(goFiles, f)
		}
	}
	if len(goFiles) == 0 {
		return nil, nil
	}

	seen := map[string]bool{}
	var syms []string
	for _, f := range goFiles {
		abs := filepath.Join(roomPath, f)
		if _, err := os.Stat(abs); err != nil {
			continue // borrado en el diff
		}
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, abs, nil, 0)
		if err != nil {
			continue // no compila todavía: no hay símbolos que leer
		}
		for _, d := range node.Decls {
			recogerSimbolo(d, seen, &syms)
		}
	}
	sort.Strings(syms)
	return &syms, nil
}

func recogerSimbolo(decl ast.Decl, seen map[string]bool, syms *[]string) {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		if d.Name.IsExported() {
			agregar(d.Name.Name, seen, syms)
		}
	case *ast.GenDecl:
		for _, spec := range d.Specs {
			switch s := spec.(type) {
			case *ast.TypeSpec:
				if s.Name.IsExported() {
					agregar(s.Name.Name, seen, syms)
				}
			case *ast.ValueSpec:
				for _, n := range s.Names {
					if n.IsExported() {
						agregar(n.Name, seen, syms)
					}
				}
			}
		}
	}
}

func agregar(name string, seen map[string]bool, syms *[]string) {
	if seen[name] {
		return
	}
	seen[name] = true
	*syms = append(*syms, name)
}
