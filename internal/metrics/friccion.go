package metrics

import (
	"context"
	"encoding/json"
	"os/exec"
	"time"
)

// Friccion son los minutos entre abrir el PR y aprobarlo. Es la
// única de las cinco métricas cuya fuente no vive en el repo: el ciclo de
// revisión pasa en GitHub, así que hay que preguntárselo a gh.
//
// Degrada en abierto, como el revisor y al revés que el examinador: sin
// gh, sin red, o con PRs que nadie aprobó todavía, devuelve nil y
// `report` lo imprime como "— sin datos". Una métrica de observación no
// puede frenar trabajo ni volver rojo un comando de lectura.
//
// Solo cuentan los PR aprobados: uno abierto todavía no tiene fricción
// medida, tiene fricción en curso, y meterlo con el reloj de hoy haría
// que el número creciera solo por dejar un PR abierto.
func Friccion(ctx context.Context, root string, entregas []Entrega) *float64 {
	gh, err := exec.LookPath("gh")
	if err != nil {
		return nil
	}
	var minutos []float64
	for _, e := range entregas {
		if e.PR == "" {
			continue
		}
		cmd := exec.CommandContext(ctx, gh, "pr", "view", e.PR, "--json", "createdAt,reviews")
		cmd.Dir = root
		salida, err := cmd.Output()
		if err != nil {
			continue // ese PR no se pudo leer; los demás sí valen
		}
		if m, ok := minutosHastaAprobar(salida); ok {
			minutos = append(minutos, m)
		}
	}
	if len(minutos) == 0 {
		return nil
	}
	var suma float64
	for _, m := range minutos {
		suma += m
	}
	prom := redondear(suma/float64(len(minutos)), 1)
	return &prom
}

// vistaPR es el recorte de `gh pr view --json createdAt,reviews` que
// interesa. gh entrega las revisiones en orden de envío.
type vistaPR struct {
	CreatedAt time.Time `json:"createdAt"`
	Reviews   []struct {
		State       string    `json:"state"`
		SubmittedAt time.Time `json:"submittedAt"`
	} `json:"reviews"`
}

// minutosHastaAprobar mide de la apertura del PR a la PRIMERA aprobación.
// La primera y no la última: lo que se mide es cuánto tarda el trabajo en
// quedar desbloqueado, y una segunda aprobación ya no desbloquea nada.
//
// Devuelve ok=false cuando no hay nada que medir todavía: json ilegible,
// PR sin aprobar, o una marca de tiempo en cero. Un negativo también se
// descarta — relojes de servidor, no fricción.
func minutosHastaAprobar(data []byte) (float64, bool) {
	var v vistaPR
	if err := json.Unmarshal(data, &v); err != nil {
		return 0, false
	}
	if v.CreatedAt.IsZero() {
		return 0, false
	}
	for _, r := range v.Reviews {
		if r.State != "APPROVED" || r.SubmittedAt.IsZero() {
			continue
		}
		min := r.SubmittedAt.Sub(v.CreatedAt).Minutes()
		if min < 0 {
			return 0, false
		}
		return min, true
	}
	return 0, false
}
