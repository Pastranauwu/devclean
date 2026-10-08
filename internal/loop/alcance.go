package loop

import (
	"os/exec"
	"strings"
)

// topeListaArchivos: un repo más grande que esto no se lista entero; sale
// más caro mandarlo en cada turno que dejar que el agente busque.
const topeListaArchivos = 200

// alcancePara pega al final del prompt la lista de archivos del cuarto.
// Sin ella el agente abría cada intento explorando: en una corrida real, una
// mediana de 6 llamadas (ls, find, ARCHITECTURE.md) antes de la primera
// edición, y cada llamada es un turno que reenvía todo el contexto. El
// contenido de sus archivos no se pega: Edit exige haberlos leído con
// Read, así que se pagaría dos veces. Va al final: cambia entre intentos
// y no puede entrar en el prefijo común del caché.
func alcancePara(dir string) string {
	cmd := exec.Command("git", "ls-files", "-co", "--exclude-standard")
	cmd.Dir = dir
	out, err := cmd.Output()
	archivos := strings.Fields(string(out))
	if err != nil || len(archivos) == 0 || len(archivos) > topeListaArchivos {
		return ""
	}
	return "\nArchivos del repositorio (es todo lo que hay: no lo listes ni lo recorras):\n" + strings.Join(archivos, "\n") +
		"\nLee tus archivos y solo los de otros módulos que tu código llama. La arquitectura ya está resumida arriba.\n"
}
