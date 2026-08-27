package ship

import (
	"regexp"
)

// escanearSecretos busca credenciales en las líneas añadidas (§6.5.4).
// Patrones de alta señal: claves de proveedores, claves privadas y
// asignaciones literales de contraseñas. Las keys nunca entran al PR.
func escanearSecretos(diff string) []Hallazgo {
	var h []Hallazgo
	for _, ad := range parseDiffAnadido(diff) {
		for _, linea := range ad.lineas {
			if nombre := tipoSecreto(linea); nombre != "" {
				h = append(h, Hallazgo{Tipo: nombre, Archivo: ad.nombre})
			}
		}
	}
	return h
}

var patronesSecretos = []struct {
	nombre string
	re     *regexp.Regexp
}{
	{"clave AWS", regexp.MustCompile(`\b(AKIA|ASIA)[A-Z0-9]{16}\b`)},
	{"clave privada", regexp.MustCompile(`-----BEGIN (RSA |EC |OPENSSH |DSA |PGP )?PRIVATE KEY-----`)},
	{"token GitHub", regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{36,}\b`)},
	{"clave de API de Google", regexp.MustCompile(`\bAIza[0-9A-Za-z_\-]{35}\b`)},
	{"clave Stripe", regexp.MustCompile(`\bsk_(live|test)_[0-9a-zA-Z]{24}\b`)},
	{"token Slack", regexp.MustCompile(`\bxox[baprs]-[0-9a-zA-Z-]{10,}\b`)},
	{"credencial en claro", regexp.MustCompile(`(?i)\b(api[_-]?key|secret|password|passwd|token)\s*[:=]\s*['"][A-Za-z0-9_\-./+]{8,}['"]`)},
}

func tipoSecreto(linea string) string {
	for _, p := range patronesSecretos {
		if p.re.MatchString(linea) {
			return p.nombre
		}
	}
	return ""
}
