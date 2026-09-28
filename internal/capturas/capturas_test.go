package capturas

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

func TestUnirYNombre(t *testing.T) {
	if got := unir("http://localhost:8000/", "/#/agregar"); got != "http://localhost:8000/#/agregar" {
		t.Errorf("unir = %q", got)
	}
	if nombre("/") != "inicio" || nombre("/#/agregar") != "agregar" {
		t.Errorf("nombres: %q %q", nombre("/"), nombre("/#/agregar"))
	}
}

func puertoLibre(t *testing.T) int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// levanta una app de verdad, la fotografía y la apaga
func TestTomarLevantaCapturaYApaga(t *testing.T) {
	if Navegador() == "" {
		t.Skip("sin navegador headless")
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("sin python3")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<h1 style='background:#4FB3A2'>hola</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}
	puerto := puertoLibre(t)
	p := Pantallas{Levantar: "exec python3 -m http.server $PORT --bind 127.0.0.1", URL: "http://127.0.0.1:$PORT", Rutas: []string{"/"}}
	fotos, err := Tomar(context.Background(), dir, p, puerto, nil, filepath.Join(dir, "fotos"))
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(fotos[0]); err != nil || info.Size() == 0 {
		t.Fatalf("captura vacía: %v", err)
	}
	// la app quedó apagada: el puerto se puede volver a tomar
	l, err := net.Listen("tcp", "127.0.0.1:"+itoa(puerto))
	if err != nil {
		t.Fatalf("la app sigue viva en %d: %v", puerto, err)
	}
	l.Close()
}

func itoa(n int) string { return strconv.Itoa(n) }
