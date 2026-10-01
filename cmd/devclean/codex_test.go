package main

import (
	"slices"
	"testing"

	"github.com/Pastranauwu/devclean/internal/config"
	"github.com/Pastranauwu/devclean/internal/executor"
)

// config.Clis valida `cli:` y los providers sin poder importar executor:
// si las dos listas se separan, un CLI nuevo se instala y la config lo
// rechaza.
func TestClisDeConfigSonLosEjecutores(t *testing.T) {
	a, b := slices.Clone(config.Clis), executor.Nombres()
	slices.Sort(a)
	slices.Sort(b)
	if !slices.Equal(a, b) {
		t.Errorf("config.Clis = %v, executor.Todos = %v", a, b)
	}
}

func TestEjecutorParaMandaLosGPTACodex(t *testing.T) {
	if (executor.Codex{}).Available() != nil {
		t.Skip("codex no está instalado")
	}
	for _, m := range []string{"gpt-6-luna", "gpt-5.5"} {
		if e := ejecutorPara(executor.Claude{}, m); e.Name() != "codex" {
			t.Errorf("%s corre en %s, quiero codex", m, e.Name())
		}
	}
	if e := ejecutorPara(executor.Codex{}, "sonnet"); e.Name() == "codex" && (executor.Claude{}).Available() == nil {
		t.Error("sonnet no puede correr en codex")
	}
}
