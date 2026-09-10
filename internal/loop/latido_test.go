package loop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Un latido recién escrito está vivo; el mismo archivo sin refrescar más
// allá del umbral está muerto. Es la premisa que un SIGKILL rompía: el
// archivo quedaba en disco y todo el mundo lo leía como "corriendo".
func TestLatidoVivoSoloMientrasLoRefrescan(t *testing.T) {
	root := t.TempDir()
	EscribirLatido(root, Latido{ID: "T-001", Intento: 1, Fase: FaseAgente})

	if _, vivo := LeerLatido(root, "T-001"); !vivo {
		t.Fatal("recién escrito debería estar vivo")
	}
	if _, muerta := Interrumpida(root, "T-001"); muerta {
		t.Error("recién escrito no está interrumpido")
	}

	// envejecerlo a mano: es lo que deja un SIGKILL
	l, _ := LeerLatidoCrudo(root, "T-001")
	l.Visto = time.Now().UTC().Add(-LatidoRancioTras - time.Second)
	escribirCrudo(t, root, l)

	if _, vivo := LeerLatido(root, "T-001"); vivo {
		t.Error("un latido que nadie refresca no está vivo")
	}
	lat, muerta := Interrumpida(root, "T-001")
	if !muerta {
		t.Fatal("un latido rancio es una corrida interrumpida")
	}
	if lat.Intento != 1 || lat.Fase != FaseAgente {
		t.Errorf("el latido rancio perdió su contenido: %+v", lat)
	}
}

// Sin archivo no hay ni corrida viva ni corrida muerta: la tarea nunca
// arrancó. Distinguirlo importa porque solo la segunda se retoma.
func TestSinLatidoNoHayCorridaNiInterrupcion(t *testing.T) {
	root := t.TempDir()
	if _, vivo := LeerLatido(root, "T-404"); vivo {
		t.Error("sin archivo no hay corrida viva")
	}
	if _, muerta := Interrumpida(root, "T-404"); muerta {
		t.Error("sin archivo no hay corrida interrumpida")
	}
}

// Un latido de un binario anterior no trae Visto. Se da por muerto: es
// la dirección segura, porque la alternativa deja la tarea atascada para
// siempre, que es justo el problema que el campo vino a resolver.
func TestLatidoSinVistoSeDaPorMuerto(t *testing.T) {
	root := t.TempDir()
	escribirCrudo(t, root, Latido{ID: "T-001", Intento: 1, Fase: FaseAgente}) // Visto en cero
	if _, vivo := LeerLatido(root, "T-001"); vivo {
		t.Error("sin Visto no se puede afirmar que corre")
	}
	if _, muerta := Interrumpida(root, "T-001"); !muerta {
		t.Error("sin Visto se retoma, no se abandona")
	}
}

// escribirCrudo guarda un latido tal cual, sin estampar Visto. Solo sirve
// para fabricar latidos viejos en las pruebas; EscribirLatido siempre lo
// estampa, que es lo correcto en producción.
func escribirCrudo(t *testing.T, root string, l Latido) {
	t.Helper()
	p := latidoPath(root, l.ID)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	datos, err := json.Marshal(l)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, datos, 0o644); err != nil {
		t.Fatal(err)
	}
}
