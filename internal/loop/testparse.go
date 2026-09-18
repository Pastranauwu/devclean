package loop

import (
	"regexp"
	"strconv"
)

// parseTestCounts extrae tests_pasaron/tests_fallaron de la salida de
// listo_cuando. Si el formato no se reconoce, ambos quedan en nil (null
// en JSON) y solo cuenta el código de salida. No inventar números.
var (
	passedFailedRE = regexp.MustCompile(`(\d+)\s+passed[,;]?\s+(\d+)\s+failed`)
	passingRE      = regexp.MustCompile(`(\d+)\s+passing`)
	failingRE      = regexp.MustCompile(`(\d+)\s+failing`)
)

// ParseTestCounts parses test pass/fail counts from test runner output.
// Returns (nil, nil) when the format is not recognized — callers must
// not invent numbers.
func ParseTestCounts(salida string) (pasaron, fallaron *int) {
	// pytest y jest: "5 passed, 4 failed" / "Tests: 5 passed, 4 failed"
	if m := passedFailedRE.FindStringSubmatch(salida); m != nil {
		p, _ := strconv.Atoi(m[1])
		f, _ := strconv.Atoi(m[2])
		return &p, &f
	}
	// mocha: "5 passing" y "2 failing". Si omite uno, es cero: el formato
	// solo imprime el contador cuando es distinto de cero.
	p, okP := contar(passingRE, salida)
	f, okF := contar(failingRE, salida)
	if okP || okF {
		if !okP {
			p = 0
		}
		if !okF {
			f = 0
		}
		return &p, &f
	}
	return nil, nil
}

func contar(re *regexp.Regexp, salida string) (int, bool) {
	m := re.FindStringSubmatch(salida)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, false
	}
	return n, true
}

// vacuoRE reconoce la salida con que un runner avisa que no había nada
// que correr; corrioRE, la evidencia de que alguna prueba sí corrió.
var (
	vacuoRE  = regexp.MustCompile(`(?i)\[no test files\]|no tests to run|no tests ran|no tests found|collected 0 items`)
	corrioRE = regexp.MustCompile(`(?m)^(ok\s|--- PASS|--- FAIL|PASS|FAIL|OK\b)|\d+\s+(passed|passing|failed|failing)`)
)

// SinPruebas reporta si un comando que salió con 0 lo hizo sin ejecutar
// ninguna prueba. `go test ./pkg/...` sobre un paquete sin archivos de
// prueba devuelve 0, así que un verde así no prueba nada: el agente
// escribió sus pruebas y la reversión de alcance las quitó, o el
// examinador ciego degradó y nunca hubo suite. Quien decide verde por
// código de salida tiene que descartar ese caso.
//
// Solo cuenta como vacío si el runner avisó Y no hay rastro de ninguna
// prueba ejecutada: una corrida multi-paquete donde otros paquetes sí
// corrieron pruebas es verde legítimo.
func SinPruebas(salida string) bool {
	if !vacuoRE.MatchString(salida) {
		return false
	}
	if p, _ := ParseTestCounts(salida); p != nil && *p > 0 {
		return false
	}
	return !corrioRE.MatchString(salida)
}
