package ship

import (
	"context"
	"testing"
	"time"

	"github.com/Pastranauwu/devclean/internal/sealed"
)

// Quemar la suite al fallar dejaba el paso saltado en el siguiente ship
// (sin suite sellada se omite en silencio): la misma tarea que la
// compuerta acababa de frenar salía en un PR con solo repetir el comando.
func TestSuiteOcultaNoSeQuemaAlFallar(t *testing.T) {
	root := repoConCommit(t)
	tarea := taskTitulo("una tarea")
	s := sealed.SuiteOculta{Content: "no importa el contenido\n", Archivo: "oculta_test.txt"}
	if err := sealed.Write(root, tarea.ID, s); err != nil {
		t.Fatal(err)
	}

	_, detalle, ok := verificarSuiteOculta(context.Background(), root, root, tarea, "false", time.Minute)
	if ok {
		t.Fatalf("un examen que falla no puede aprobar la compuerta · detalle=%q", detalle)
	}
	if !sealed.Exists(root, tarea.ID) {
		t.Fatal("la suite se quemó al fallar: el siguiente ship saltaría el paso")
	}

	// y el reintento vuelve a examinar, no salta
	if _, _, ok := verificarSuiteOculta(context.Background(), root, root, tarea, "false", time.Minute); ok {
		t.Error("el reintento aprobó sin haber arreglado nada")
	}
}

func TestSuiteOcultaSeQuemaAlAprobar(t *testing.T) {
	root := repoConCommit(t)
	tarea := taskTitulo("una tarea")
	s := sealed.SuiteOculta{Content: "no importa el contenido\n", Archivo: "oculta_test.txt"}
	if err := sealed.Write(root, tarea.ID, s); err != nil {
		t.Fatal(err)
	}

	if _, _, ok := verificarSuiteOculta(context.Background(), root, root, tarea, "true", time.Minute); !ok {
		t.Fatal("el examen pasó y la compuerta lo rechazó")
	}
	if sealed.Exists(root, tarea.ID) {
		t.Error("el examen aprobado debe consumirse, no volver a correr")
	}
}
