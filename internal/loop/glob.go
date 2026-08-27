package loop

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
func matchGlob(pattern, s string) bool {
	re, err := regexp.Compile(globRE(pattern))
	if err != nil {
		return false
	}
	return re.MatchString(s)
}

// matchPattern reports whether path matches one contract/zone pattern.
// Sin barra, el patrón es un nombre de archivo y casa contra la base;
// con barra, casa contra la ruta completa.
func matchPattern(pattern, s string) bool {
	if !strings.Contains(pattern, "/") {
		return matchGlob(pattern, path.Base(s))
	}
	return matchGlob(pattern, s)
}

// matchesAny reports whether path matches any of the patterns.
func matchesAny(patterns []string, s string) bool {
	for _, p := range patterns {
		if matchPattern(p, s) {
			return true
		}
	}
	return false
}
