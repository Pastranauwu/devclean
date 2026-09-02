package ventanas

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestSondaClaudeLeeCabeceras(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") == "" {
			t.Error("la sonda no mandó la API key")
		}
		w.Header().Set("anthropic-ratelimit-unified-5h-utilization", "47")
		w.Header().Set("anthropic-ratelimit-unified-7d-utilization", "12")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"x"}`))
	}))
	defer srv.Close()
	viejoURL := sondaURL
	sondaURL = srv.URL
	defer func() { sondaURL = viejoURL }()

	u, err := SondaClaude(context.Background(), "test-key", 5*time.Second)
	if err != nil {
		t.Fatalf("SondaClaude: %v", err)
	}
	if u.CincoH == nil || *u.CincoH != 47 {
		t.Errorf("5h = %v, quiero 47", u.CincoH)
	}
	if u.Semanal == nil || *u.Semanal != 12 {
		t.Errorf("semanal = %v, quiero 12", u.Semanal)
	}
	if !u.Accesible {
		t.Error("con cabeceras la cuenta debe marcar accesible")
	}
}

func TestSondaClaudeDegradaSinKeyOSinCabeceras(t *testing.T) {
	u, err := SondaClaude(context.Background(), "", 5*time.Second)
	if err != nil {
		t.Fatalf("sin key no debe fallar: %v", err)
	}
	if u.Accesible {
		t.Error("sin key la cuenta no debe marcar accesible")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	viejoURL := sondaURL
	sondaURL = srv.URL
	defer func() { sondaURL = viejoURL }()

	u, err = SondaClaude(context.Background(), "k", 5*time.Second)
	if err != nil {
		t.Fatalf("sin cabeceras no debe fallar: %v", err)
	}
	if u.Accesible {
		t.Error("sin cabeceras la cuenta no debe marcar accesible")
	}
}

func TestSondaCachedUsaCacheFresca(t *testing.T) {
	dir := t.TempDir()
	viejoCache := cachePathFunc
	cachePathFunc = func() string { return filepath.Join(dir, "uso-claude.json") }
	defer func() { cachePathFunc = viejoCache }()

	// si la caché está fresca, la sonda no hace red: sondaURL apuntaría a
	// un servidor que no existe, y aun así debe devolver los datos
	sondaURL = "http://127.0.0.1:1" // puerto muerto: si llama acá, falla
	_ = escribirCache(Uso{Fecha: time.Now().UTC(), CincoH: intPtr(33), Semanal: intPtr(9), Accesible: true})

	u := SondaCached(context.Background(), "k", false)
	if u.CincoH == nil || *u.CincoH != 33 {
		t.Errorf("caché no usada: %+v", u)
	}
}

func intPtr(n int) *int { return &n }
