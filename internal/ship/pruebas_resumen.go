package ship

import (
	"regexp"
	"strings"
)

var ansiPruebas = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

// resumenPruebas conserva el fallo y la aserción que lo explica. Los runners
// suelen cerrar con estadísticas o enlaces, que no sirven como diagnóstico.
func resumenPruebas(s string) string {
	s = ansiPruebas.ReplaceAllString(s, "")
	var lineas []string
	for _, raw := range strings.Split(s, "\n") {
		l := strings.TrimSpace(raw)
		if l == "" {
			continue
		}
		if strings.HasPrefix(l, "FAIL ") || strings.HasPrefix(l, "Error:") ||
			strings.HasPrefix(l, "AssertionError:") || strings.HasPrefix(l, "Expected:") ||
			strings.HasPrefix(l, "Received:") {
			if len(l) > 240 {
				l = l[:240] + "…"
			}
			lineas = append(lineas, l)
		}
		if len(lineas) == 5 {
			break
		}
	}
	if len(lineas) > 0 {
		return strings.Join(lineas, " · ")
	}
	// Los comandos que no son runners también necesitan un motivo.
	return tail(s)
}
