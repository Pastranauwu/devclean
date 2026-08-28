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
// not invent numbers (adenda A.2).
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
