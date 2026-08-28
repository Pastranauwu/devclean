package loop

import "testing"

func TestParseTestCountsPytest(t *testing.T) {
	p, f := ParseTestCounts("5 passed, 4 failed in 2.3s")
	if p == nil || f == nil || *p != 5 || *f != 4 {
		t.Fatalf("pytest = %v/%v, quiero 5/4", p, f)
	}
}

func TestParseTestCountsJest(t *testing.T) {
	p, f := ParseTestCounts("Tests:       5 passed, 4 failed, 9 total")
	if p == nil || f == nil || *p != 5 || *f != 4 {
		t.Fatalf("jest = %v/%v, quiero 5/4", p, f)
	}
}

func TestParseTestCountsMocha(t *testing.T) {
	p, f := ParseTestCounts("  5 passing (12ms)\n  2 failing\n")
	if p == nil || f == nil || *p != 5 || *f != 2 {
		t.Fatalf("mocha = %v/%v, quiero 5/2", p, f)
	}
	// solo passing: el fallido se omite cuando es cero
	p, f = ParseTestCounts("  3 passing (1ms)\n")
	if p == nil || f == nil || *p != 3 || *f != 0 {
		t.Fatalf("mocha passing solo = %v/%v, quiero 3/0", p, f)
	}
}

func TestParseTestCountsSinFormato(t *testing.T) {
	// go test no imprime contadores: null, no un número inventado
	if p, f := ParseTestCounts("ok  pkg/foo  0.123s\n--- FAIL: TestBar\nFAIL"); p != nil || f != nil {
		t.Fatalf("salida sin contadores = %v/%v, quiero nil/nil", p, f)
	}
	if p, f := ParseTestCounts(""); p != nil || f != nil {
		t.Fatalf("salida vacía = %v/%v, quiero nil/nil", p, f)
	}
}
