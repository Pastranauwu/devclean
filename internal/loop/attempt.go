// Package loop implements the work loop of §6.4 and its instrumentation
// (adenda A.2): cada intento escribe una línea en
// .devclean/runs/<id>/attempts.jsonl. Ese archivo es la única fuente de
// las métricas y del parte de datos (§6.7); nada se recalcula leyendo el
// repo después.
package loop

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Attempt is one line of attempts.jsonl. Los campos que el bucle no
// puede derivar con certeza van en nil, que en JSON es null: un número
// que no se pudo medir nunca se inventa (adenda A.2).
type Attempt struct {
	Intento                  int       `json:"intento"`
	Inicio                   time.Time `json:"inicio"`
	Fin                      time.Time `json:"fin"`
	SalidaCodigo             *int      `json:"salida_codigo"`
	TestsPasaron             *int      `json:"tests_pasaron"`
	TestsFallaron            *int      `json:"tests_fallaron"`
	ArchivosTocados          []string  `json:"archivos_tocados"`
	SimbolosExportados       *[]string `json:"simbolos_exportados"`
	LineasMas                int       `json:"lineas_mas"`
	LineasMenos              int       `json:"lineas_menos"`
	RevertidosFueraDeAlcance []string  `json:"revertidos_fuera_de_alcance"`
	Tokens                   Tokens    `json:"tokens"`
	Modelo                   string    `json:"modelo"`
}

// Tokens is the token spend of one attempt.
type Tokens struct {
	Entrada int `json:"entrada"`
	Salida  int `json:"salida"`
}

// RunsDir returns the runs directory of the repository at root.
func RunsDir(root string) string { return filepath.Join(root, ".devclean", "runs") }

// attemptsPath returns the attempts.jsonl path of a task.
func attemptsPath(root, id string) string {
	return filepath.Join(RunsDir(root), id, "attempts.jsonl")
}

// store writes attempts.jsonl lines append-only. Cada Run abre uno.
type store struct {
	path string
}

func openStore(root, id string) (*store, error) {
	p := attemptsPath(root, id)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return nil, err
	}
	return &store{path: p}, nil
}

// Append writes one attempt as a single JSON line.
func (s *store) Append(a Attempt) error {
	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := json.Marshal(a)
	if err != nil {
		return err
	}
	_, err = f.Write(append(data, '\n'))
	return err
}

// ReadAttempts returns every recorded attempt of a task, oldest first.
// A task that never ran has no attempts (not an error).
func ReadAttempts(root, id string) ([]Attempt, error) {
	f, err := os.Open(attemptsPath(root, id))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var out []Attempt
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		var a Attempt
		if err := json.Unmarshal([]byte(line), &a); err != nil {
			return nil, fmt.Errorf("attempts.jsonl corrupto · %s", err)
		}
		out = append(out, a)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
