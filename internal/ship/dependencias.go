package ship

import (
	"fmt"
	"strings"
)

// verificarDependencias comprueba que el diff no viole las reglas de
// importación declaradas en config (§6.10). Para cada regla
// "A → B → C", un archivo que pertenezca a una capa inferior no puede
// importar una capa superior.
//
// Detección: para cada línea `+import` del diff:
//   - si el path del archivo contiene el nombre de una capa
//   - y el paquete importado contiene el nombre de una capa superior
//   - es una violación.
//
// Es heurístico: funciona cuando los nombres de las capas aparecen en
// las rutas. No reemplaza un analizador de AST completo.
func verificarDependencias(diff string, reglasImport []string) (string, bool) {
	if len(reglasImport) == 0 {
		return "sin reglas declaradas", true
	}

	// parse all chains into ordered layers
	type capaSlice []string // ordered from top (can import all below) to bottom

	var cadenas []capaSlice
	for _, regla := range reglasImport {
		partes := strings.Split(regla, "→")
		if len(partes) < 2 {
			continue
		}
		var c capaSlice
		for _, p := range partes {
			c = append(c, strings.TrimSpace(p))
		}
		cadenas = append(cadenas, c)
	}

	// parse diff: find current file and its imports
	var violaciones []string
	archivoActual := ""
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "diff --git") {
			// extract file path: "diff --git a/foo b/foo" → "foo"
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				archivoActual = strings.TrimPrefix(parts[3], "b/")
			}
			continue
		}
		if !strings.HasPrefix(line, "+import") && !strings.HasPrefix(line, "+\t\"") && !strings.HasPrefix(line, "+\t`") {
			continue
		}
		// extract imported package path
		importado := extraerImport(line)
		if importado == "" {
			continue
		}
		// check against each chain
		for _, c := range cadenas {
			capArchivo := posicionEnCadena(archivoActual, c)
			capImport := posicionEnCadena(importado, c)
			if capArchivo < 0 || capImport < 0 {
				continue // no matching layer name in either
			}
			// valid: archivo_pos < import_pos (file is higher, importing lower is ok)
			// violation: archivo_pos > import_pos (file is lower, importing higher)
			if capArchivo > capImport {
				violaciones = append(violaciones, fmt.Sprintf("%s importa %s (viola %s→%s)",
					archivoActual, importado, c[capImport], c[capArchivo]))
			}
		}
	}

	if len(violaciones) > 0 {
		return strings.Join(violaciones, "; "), false
	}
	return "grafo de imports limpio", true
}

func posicionEnCadena(ruta string, c []string) int {
	for i, capa := range c {
		if strings.Contains(strings.ToLower(ruta), strings.ToLower(capa)) {
			return i
		}
	}
	return -1
}

func extraerImport(line string) string {
	// handles: +import "pkg", +	"pkg", +	`pkg`
	s := strings.TrimLeft(line, "+ \t")
	s = strings.TrimPrefix(s, "import")
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "\"`")
	if strings.Contains(s, " ") || s == "" {
		return ""
	}
	return s
}
