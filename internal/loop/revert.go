package loop

import (
	"fmt"
	"os"
	"path/filepath"
)

// revertFueraDeAlcance revierte los archivos que el agente tocó fuera de
// tocar_solo y los archivos de prueba (adenda A.3), y devuelve la lista
// de lo revertido para anotarlo en attempts.jsonl. La reversión es el
// enforcement real de los límites: la verificación la hace el código, no
// la confianza en el modelo (§6.4, §11).
func revertFueraDeAlcance(roomPath string, tocarSolo, patrones []string) ([]string, error) {
	files, err := statusFiles(roomPath)
	if err != nil {
		return nil, err
	}
	var revertidos []string
	for _, f := range files {
		if enAlcance(f, tocarSolo) && !esPrueba(f, patrones) {
			continue
		}
		if err := revertir(roomPath, f); err != nil {
			return revertidos, err
		}
		revertidos = append(revertidos, f)
	}
	return revertidos, nil
}

// enAlcance reporta si un archivo cae dentro de tocar_solo. Vacío
// significa "sin restricción": todo el repo salvo las pruebas.
func enAlcance(s string, tocarSolo []string) bool {
	if len(tocarSolo) == 0 {
		return true
	}
	return matchesAny(tocarSolo, s)
}

func esPrueba(s string, patrones []string) bool {
	return matchesAny(patrones, s)
}

// revertir devuelve un archivo al estado de HEAD, o lo borra si no tenía
// seguimiento (nuevo).
func revertir(roomPath, s string) error {
	if _, err := gitRun(roomPath, "ls-files", "--error-unmatch", "--", s); err != nil {
		// sin seguimiento: lo creó el agente, no hay HEAD al que volver
		if err := os.Remove(filepath.Join(roomPath, s)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("no se pudo borrar %s · %s", s, err)
		}
		return nil
	}
	if _, err := gitRun(roomPath, "restore", "--source=HEAD", "--", s); err != nil {
		return fmt.Errorf("no se pudo revertir %s · %s", s, err)
	}
	return nil
}
