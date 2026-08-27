package ship

import (
	"fmt"

	"github.com/Pastranauwu/devclean/internal/task"
)

// verificarPresupuesto aplica §6.5.5: las líneas añadidas no pueden
// pasar de limite_lineas; los archivos tocados se reportan.
func verificarPresupuesto(mas, menos, archivos int, t task.Task) (string, bool) {
	limite := t.LimiteLineas
	if limite < 1 {
		limite = task.DefaultLimiteLineas
	}
	detalle := fmt.Sprintf("%d líneas de %d · %d archivos", mas, limite, archivos)
	if mas > limite {
		return fmt.Sprintf("presupuesto excedido · %s · reduce el cambio o sube limite_lineas", detalle), false
	}
	return detalle, true
}
