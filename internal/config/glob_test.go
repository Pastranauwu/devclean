package config

import "testing"

func TestMatchGlob(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"src/export/**", "src/export/writer.go", true},
		{"src/export/**", "src/export/sub/writer.go", true},
		{"src/export/**", "src/auth/writer.go", false},
		{"src/**", "src/a/b/c.go", true},
		{"src/**", "docs/a.go", false},
		{"src/*", "src/x.go", true},
		{"src/*", "src/sub/x.go", false},
		{"*.go", "x.go", true},
	}
	for _, tc := range cases {
		if got := MatchGlob(tc.pattern, tc.path); got != tc.want {
			t.Errorf("MatchGlob(%q, %q) = %v, quiero %v", tc.pattern, tc.path, got, tc.want)
		}
	}
}

func TestMatchPatternNombreDeArchivo(t *testing.T) {
	// sin barra, casa contra la base del archivo
	if !MatchPattern("*_test.go", "src/export/writer_test.go") {
		t.Error("*_test.go debió casar con la base de una ruta profunda")
	}
	if MatchPattern("*_test.go", "src/export/writer.go") {
		t.Error("*_test.go no debió casar con writer.go")
	}
	if !MatchPattern("*.spec.ts", "src/export/writer.spec.ts") {
		t.Error("*.spec.ts debió casar")
	}
}

func TestMatchesAny(t *testing.T) {
	patrones := []string{"*_test.go", "test/**", "spec/**"}
	if !MatchesAny(patrones, "test/foo/bar.go") {
		t.Error("test/** debió casar")
	}
	if !MatchesAny(patrones, "spec/x.spec.ts") {
		t.Error("spec/** debió casar")
	}
	if MatchesAny(patrones, "src/export/writer.go") {
		t.Error("src/export/writer.go no debió casar con patrones de prueba")
	}
}
