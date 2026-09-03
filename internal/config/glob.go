package config

// El matcher de rutas vive junto a los patrones que consume
// (zonas_prohibidas, patrones_prueba, tocar_solo): el bucle lo usa para
// revertir lo que se salió del alcance y la esclusa de salida para no
// cobrarle al agente las pruebas del examinador.

import (
	"path"
	"regexp"
	"strings"
)

// globRE converts a doublestar glob into a regex. `**` cruza
// directorios; `*` y `?` se quedan dentro de un segmento de ruta.
func globRE(pattern string) string {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				b.WriteString(".*")
				i++
				if i+1 < len(pattern) && pattern[i+1] == '/' {
					b.WriteString("/?")
					i++
				}
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(string(pattern[i])))
		}
	}
	b.WriteString("$")
	return b.String()
}

// matchGlob reports whether path matches a doublestar glob.
func MatchGlob(pattern, s string) bool {
	re, err := regexp.Compile(globRE(pattern))
	if err != nil {
		return false
	}
	return re.MatchString(s)
}

// matchPattern reports whether path matches one contract/zone pattern.
// Sin barra, el patrón es un nombre de archivo y casa contra la base;
// con barra, casa contra la ruta completa.
func MatchPattern(pattern, s string) bool {
	if !strings.Contains(pattern, "/") {
		return MatchGlob(pattern, path.Base(s))
	}
	return MatchGlob(pattern, s)
}

// matchesAny reports whether path matches any of the patterns.
func MatchesAny(patterns []string, s string) bool {
	for _, p := range patterns {
		if MatchPattern(p, s) {
			return true
		}
	}
	return false
}
