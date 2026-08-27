package ship

import (
	"path"
	"regexp"
	"strings"
)

// escanearRuido busca la basura de §6.5.3: prints de debug, código
// comentado y archivos temporales. Conservador a propósito: un falso
// positivo frena un PR válido.
func escanearRuido(diff string, archivos []string) []Hallazgo {
	var h []Hallazgo
	for _, ad := range parseDiffAnadido(diff) {
		for _, linea := range ad.lineas {
			if t := tipoDebug(linea); t != "" {
				h = append(h, Hallazgo{Tipo: t, Archivo: ad.nombre, Detalle: recortar(linea)})
			} else if esCodigoComentado(linea) {
				h = append(h, Hallazgo{Tipo: "código comentado", Archivo: ad.nombre, Detalle: recortar(linea)})
			}
		}
	}
	for _, a := range archivos {
		if esTemporal(a) {
			h = append(h, Hallazgo{Tipo: "archivo temporal", Archivo: a})
		}
	}
	return h
}

var patronesDebug = []struct {
	tipo string
	re   *regexp.Regexp
}{
	{"print de debug", regexp.MustCompile(`fmt\.Print(?:ln|f)?\(`)},
	{"print de debug", regexp.MustCompile(`log\.Print(?:ln|f)?\(`)},
	{"print de debug", regexp.MustCompile(`\bprintln\(`)},
	{"print de debug", regexp.MustCompile(`\bprint\(`)},
	{"print de debug", regexp.MustCompile(`console\.(?:log|debug|warn|info|error)\(`)},
	{"print de debug", regexp.MustCompile(`\bdebugger\b`)},
	{"print de debug", regexp.MustCompile(`pdb\.set_trace\(`)},
	{"print de debug", regexp.MustCompile(`\bbreakpoint\(`)},
	{"print de debug", regexp.MustCompile(`System\.out\.print(?:ln)?\(`)},
	{"print de debug", regexp.MustCompile(`var_dump\(`)},
	{"print de debug", regexp.MustCompile(`\bprint_r\(`)},
	{"print de debug", regexp.MustCompile(`\bdd\(`)},
}

func tipoDebug(linea string) string {
	for _, p := range patronesDebug {
		if p.re.MatchString(linea) {
			return p.tipo
		}
	}
	return ""
}

// esCodigoComentado marca una línea de comentario que en realidad es
// código: termina en ; o empieza con una palabra clave tras el marcador.
// TODO/FIXME y narrativa normal pasan.
func esCodigoComentado(linea string) bool {
	t := strings.TrimSpace(linea)
	if !esComentario(t) {
		return false
	}
	if strings.Contains(t, "TODO") || strings.Contains(t, "FIXME") {
		return false
	}
	sin := strings.TrimLeft(t, "/#*")
	sin = strings.TrimSpace(sin)
	if sin == "" {
		return false
	}
	if strings.Contains(sin, ";") {
		return true
	}
	for _, kw := range []string{"return ", "if ", "for ", "while ", "func ", "def ", "var ", "let ", "const ", "import ", "class ", "type "} {
		if strings.HasPrefix(sin, kw) {
			return true
		}
	}
	return false
}

func esComentario(s string) bool {
	for _, m := range []string{"//", "#", "/*", "*", "--", "<!--"} {
		if strings.HasPrefix(s, m) {
			return true
		}
	}
	return false
}

func esTemporal(nombre string) bool {
	base := path.Base(nombre)
	for _, suf := range []string{".tmp", ".bak", ".swp", ".orig", ".rej", "~"} {
		if strings.HasSuffix(base, suf) {
			return true
		}
	}
	if strings.HasPrefix(base, "~$") {
		return true
	}
	return base == ".DS_Store" || base == "Thumbs.db"
}

func recortar(s string) string {
	s = strings.TrimSpace(s)
	const max = 80
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}
