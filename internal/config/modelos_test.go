package config

import "testing"

func TestElegirModelos(t *testing.T) {
	// catálogo real de opencode: ids provider/model
	catalogo := []string{
		"opencode/big-pickle",
		"opencode/nemotron-3.5-lightning-free",
		"opencode-go/glm-5.2",
		"opencode-go/qwen3.7-max",
	}
	m := ElegirModelos(catalogo)
	if m["liviana"] != "opencode/nemotron-3.5-lightning-free" {
		t.Errorf("liviana = %q", m["liviana"])
	}
	if m["pesada"] != "opencode-go/qwen3.7-max" {
		t.Errorf("pesada = %q", m["pesada"])
	}
	if m["media"] == m["liviana"] || m["media"] == m["pesada"] {
		t.Errorf("media = %q, debe diferir de liviana y pesada", m["media"])
	}

	// alias de claude
	c := ElegirModelos([]string{"opus", "sonnet", "haiku"})
	if c["liviana"] != "haiku" || c["pesada"] != "opus" || c["media"] != "sonnet" {
		t.Errorf("claude = %+v", c)
	}

	// catálogo sin pistas: todo cae en el mismo, nunca vacío
	u := ElegirModelos([]string{"unico"})
	for _, peso := range Pesos {
		if u[peso] != "unico" {
			t.Errorf("%s = %q, quiero unico", peso, u[peso])
		}
	}

	if ElegirModelos(nil) != nil {
		t.Error("sin catálogo no se inventa nada")
	}
}

func TestModelosValidos(t *testing.T) {
	catalogo := []string{"opencode-go/glm-5.2", "sonnet"}

	// el id inventado que rompía cada corrida
	malos := ModelosValidos([]string{"glm-5.2", "sonnet", "", "glm-5.2"}, catalogo)
	if len(malos) != 1 || malos[0] != "glm-5.2" {
		t.Errorf("malos = %v, quiero [glm-5.2] sin repetir ni contar el vacío", malos)
	}

	if malos := ModelosValidos([]string{"sonnet"}, catalogo); len(malos) != 0 {
		t.Errorf("malos = %v, quiero ninguno", malos)
	}

	// sin catálogo no se puede afirmar nada: nunca se acusa en falso
	if malos := ModelosValidos([]string{"loquesea"}, nil); len(malos) != 0 {
		t.Errorf("malos = %v, sin catálogo no se juzga", malos)
	}
}
