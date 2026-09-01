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
		entrada := esPuntoDeEntrada(ad.nombre)
		for _, linea := range ad.lineas {
			if t := tipoDebug(linea, entrada); t != "" {
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
	// esSalida marca los patrones que en el punto de entrada de un
	// programa NO son ruido, sino lo que el programa hace. Un CLI cuyo
	// trabajo es imprimir un resultado no puede entregarse nunca si
	// `fmt.Println` en su main.go cuenta como print de debug.
	esSalida bool
}{
	// log.Print* queda afuera a propósito: es el logger estándar de Go
	// para producción (servicios de red, CLIs de larga vida), no un
	// rastro de debug como fmt.Println. Marcarlo como ruido frena
	// cualquier código que loguee de verdad.
	{"print de debug", regexp.MustCompile(`fmt\.Print(?:ln|f)?\(`), true},
	{"print de debug", regexp.MustCompile(`\bprintln\(`), true},
	{"print de debug", regexp.MustCompile(`\bprint\(`), true},
	{"print de debug", regexp.MustCompile(`console\.(?:log|warn|info|error)\(`), true},
	{"print de debug", regexp.MustCompile(`System\.out\.print(?:ln)?\(`), true},
	// estos no son salida de nadie: son depuradores y volcados, ruido
	// en cualquier archivo
	{"print de debug", regexp.MustCompile(`console\.debug\(`), false},
	{"print de debug", regexp.MustCompile(`\bdebugger\b`), false},
	{"print de debug", regexp.MustCompile(`pdb\.set_trace\(`), false},
	{"print de debug", regexp.MustCompile(`\bbreakpoint\(`), false},
	{"print de debug", regexp.MustCompile(`var_dump\(`), false},
	{"print de debug", regexp.MustCompile(`\bprint_r\(`), false},
	{"print de debug", regexp.MustCompile(`\bdd\(`), false},
}

func tipoDebug(linea string, puntoDeEntrada bool) string {
	for _, p := range patronesDebug {
		if p.esSalida && puntoDeEntrada {
			continue
		}
		if p.re.MatchString(linea) {
			return p.tipo
		}
	}
	return ""
}

// esPuntoDeEntrada reporta si el archivo es por dónde arranca un
// programa, donde escribir a stdout es la función y no un descuido.
// Solo por convención de ruta y nombre: leer el archivo para confirmar
// que declara `package main` obligaría a tener el cuarto a mano, y el
// escáner trabaja sobre el diff.
func esPuntoDeEntrada(archivo string) bool {
	limpio := strings.TrimPrefix(path.Clean(archivo), "./")
	for _, dir := range []string{"cmd/", "bin/", "cli/"} {
		if strings.HasPrefix(limpio, dir) || strings.Contains(limpio, "/"+dir) {
			return true
		}
	}
	switch path.Base(limpio) {
	case "main.go", "main.py", "__main__.py", "main.rs", "cli.py", "cli.js", "cli.ts", "main.js", "main.ts":
		return true
	}
	return false
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
	// ";" en medio de una oración es narrativa normal ("Defaults to X;
	// overridable in tests."); código comentado real termina la línea
	// en el punto y coma, sin nada más después.
	if strings.HasSuffix(sin, ";") {
		return true
	}
	// palabra clave al inicio no alcanza: "for", "if", "type" también
	// arrancan oraciones normales ("for Echo discovery, ..."). Solo
	// cuenta si además tiene pinta de código (paréntesis, ":=" o llave).
	tieneFormaDeCodigo := strings.ContainsAny(sin, "({") || strings.Contains(sin, ":=")
	if tieneFormaDeCodigo {
		for _, kw := range []string{"return ", "if ", "for ", "while ", "func ", "def ", "var ", "let ", "const ", "import ", "class ", "type "} {
			if strings.HasPrefix(sin, kw) {
				return true
			}
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
