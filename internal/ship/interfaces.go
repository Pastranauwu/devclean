package ship

import (
	"strings"

	"github.com/Pastranauwu/devclean/internal/task"
)

// verificarExpone comprueba que la tarea entregó las firmas que su
// contrato prometía (§6.10). Una tarea puede pasar sus propias pruebas y
// aun así no exponer lo que su hermana consume: la hermana corre en otro
// cuarto y no se entera hasta que las dos ramas se juntan, cuando ya es
// tarde.
//
// Devuelve las firmas que faltan; vacío si están todas o si la tarea no
// declaró ninguna.
func verificarExpone(firmas []string, diff string) []string {
	if len(firmas) == 0 {
		return nil
	}

	var añadido strings.Builder
	for _, ad := range parseDiffAnadido(diff) {
		for _, linea := range ad.lineas {
			añadido.WriteString(linea)
			añadido.WriteByte('\n')
		}
	}
	cuerpo := añadido.String()

	var faltan []string
	for _, f := range firmas {
		nombre := task.NombreDeFirma(f)
		if nombre == "" || strings.Contains(cuerpo, nombre) {
			continue
		}
		faltan = append(faltan, f)
	}
	return faltan
}
