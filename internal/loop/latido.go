package loop

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Fases de un intento, en el orden en que ocurren.
const (
	FaseExamen      = "examen"      // el examinador escribe la suite ciega
	FaseAgente      = "agente"      // el implementador trabaja
	FaseVerificando = "verificando" // corre listo_cuando
)

// Latido es el estado vivo de una tarea: en qué fase está ahora mismo y
// desde cuándo.
//
// Existe porque attempts.jsonl solo se escribe cuando un intento
// TERMINA. Durante los veinte minutos que puede durar una invocación no
// había un solo byte en disco, así que `standup` informaba "dentro de
// contrato" de una tarea colgada y el tablero no tenía nada que pintar.
// El archivo se borra al terminar la tarea: si está, la tarea corre.
type Latido struct {
	ID        string    `json:"id"`
	Intento   int       `json:"intento"`
	Limite    int       `json:"limite"`
	Fase      string    `json:"fase"`
	Modelo    string    `json:"modelo"`
	DesdeFase time.Time `json:"desde_fase"`
	Tokens    Tokens    `json:"tokens"` // acumulado de la tarea hasta ahora
}

// EnFaseDesde devuelve cuánto lleva la tarea en su fase actual.
func (l Latido) EnFaseDesde() time.Duration { return time.Since(l.DesdeFase) }

// Descripcion resume el latido en una línea para el tablero.
func (l Latido) Descripcion() string {
	s := fmt.Sprintf("intento %d", l.Intento)
	if l.Limite > 0 {
		s += fmt.Sprintf("/%d", l.Limite)
	}
	s += " · " + l.Fase
	if l.Modelo != "" {
		s += " · " + l.Modelo
	}
	return s
}

// latidoPath devuelve la ruta del latido de una tarea.
func latidoPath(root, id string) string {
	return filepath.Join(RunsDir(root), id, "latido.json")
}

// EscribirLatido guarda el estado vivo de una tarea. Falla en silencio:
// perder el latido nunca debe frenar el trabajo real.
func EscribirLatido(root string, l Latido) {
	p := latidoPath(root, l.ID)
	if os.MkdirAll(filepath.Dir(p), 0o755) != nil {
		return
	}
	datos, err := json.Marshal(l)
	if err != nil {
		return
	}
	_ = os.WriteFile(p, datos, 0o644)
}

// BorrarLatido quita el latido de una tarea que dejó de correr.
func BorrarLatido(root, id string) {
	_ = os.Remove(latidoPath(root, id))
}

// LeerLatido devuelve el latido de una tarea, si está corriendo.
func LeerLatido(root, id string) (Latido, bool) {
	datos, err := os.ReadFile(latidoPath(root, id))
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return Latido{}, false
		}
		return Latido{}, false
	}
	var l Latido
	if json.Unmarshal(datos, &l) != nil {
		return Latido{}, false
	}
	return l, true
}

// LeerLatidos devuelve los latidos de todas las tareas que corren ahora.
func LeerLatidos(root string, ids []string) map[string]Latido {
	res := make(map[string]Latido, len(ids))
	for _, id := range ids {
		if l, ok := LeerLatido(root, id); ok {
			res[id] = l
		}
	}
	return res
}
