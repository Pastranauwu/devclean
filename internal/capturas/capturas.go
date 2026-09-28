// Package capturas levanta la interfaz web de un proyecto y la
// fotografía en tamaño celular. Las pruebas de UI solo pueden comprobar
// que una clase existe; si el efecto se ve o no lo juzga alguien que
// mira la pantalla: el arquitecto antes de planear, el revisor visual
// después de cada tarea de interfaz y el humano en el PR.
package capturas

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Pantallas dice cómo levantar la app y qué mirar. La declara el
// arquitecto y vive en config.yml.
type Pantallas struct {
	// Levantar sirve la app completa en $PORT (con build si hace falta).
	Levantar string `json:"levantar"`
	// URL es la raíz, con $PORT: "http://localhost:$PORT".
	URL string `json:"url"`
	// Rutas son las pantallas principales: "/", "/#/agregar".
	Rutas []string `json:"rutas"`
}

func (p Pantallas) Vacia() bool {
	return strings.TrimSpace(p.Levantar) == "" || strings.TrimSpace(p.URL) == "" || len(p.Rutas) == 0
}

// Ancho y Alto son la pantalla de un celular (iPhone 14/15).
const Ancho, Alto = 390, 844

// Espera es cuánto se le da a la app para responder: puede incluir un
// build de producción.
var Espera = 3 * time.Minute

// Navegador devuelve un navegador que sabe capturar en headless, o "".
// chrome-headless-shell (el de Playwright) primero: no abre perfil ni
// ventana. Brave queda fuera: en headless se cuelga con --screenshot.
func Navegador() string {
	casa, _ := os.UserHomeDir()
	for _, patron := range []string{
		filepath.Join(casa, ".cache", "ms-playwright", "chromium_headless_shell-*", "chrome-*", "chrome-headless-shell"),
		filepath.Join(casa, "Library", "Caches", "ms-playwright", "chromium_headless_shell-*", "chrome-*", "chrome-headless-shell"),
	} {
		if m, _ := filepath.Glob(patron); len(m) > 0 {
			return m[len(m)-1]
		}
	}
	for _, b := range []string{"chromium", "chromium-browser", "google-chrome", "google-chrome-stable", "microsoft-edge"} {
		if p, err := exec.LookPath(b); err == nil {
			return p
		}
	}
	return ""
}

// Instalar trae chrome-headless-shell con Playwright (unos 100 MB, una
// vez por máquina). Sin node no hay forma.
func Instalar(ctx context.Context) error {
	if _, err := exec.LookPath("npx"); err != nil {
		return errors.New("sin navegador para capturas y sin npx para instalar uno · instala chromium o node")
	}
	out, err := exec.CommandContext(ctx, "npx", "--yes", "playwright", "install", "chromium-headless-shell").CombinedOutput()
	if err != nil {
		return fmt.Errorf("no se pudo instalar chrome-headless-shell · %s", ultimas(string(out)))
	}
	return nil
}

// Tomar levanta la app en dir con PORT=puerto, espera a que responda y
// captura cada ruta en outDir. La app se apaga al terminar, falle o no.
func Tomar(ctx context.Context, dir string, p Pantallas, puerto int, env []string, outDir string) ([]string, error) {
	if p.Vacia() {
		return nil, errors.New("sin pantallas declaradas")
	}
	nav := Navegador()
	if nav == "" {
		return nil, errors.New("sin navegador headless · corre npx playwright install chromium-headless-shell")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}
	base := strings.ReplaceAll(p.URL, "$PORT", strconv.Itoa(puerto))
	app := exec.Command("sh", "-c", p.Levantar)
	if runtime.GOOS == "windows" {
		app = exec.Command("cmd", "/c", p.Levantar)
	}
	app.Dir = dir
	app.Env = append(append(os.Environ(), env...), "PORT="+strconv.Itoa(puerto))
	registro, err := os.Create(filepath.Join(outDir, "app.log"))
	if err != nil {
		return nil, err
	}
	defer registro.Close()
	app.Stdout, app.Stderr = registro, registro
	enGrupo(app)
	if err := app.Start(); err != nil {
		return nil, fmt.Errorf("no se pudo levantar la app · %w", err)
	}
	defer apagar(app)

	if err := esperar(ctx, base, Espera); err != nil {
		return nil, fmt.Errorf("la app no respondió en %s · %s · detalle en %s", base, err, registro.Name())
	}
	var fotos []string
	for i, ruta := range p.Rutas {
		foto := filepath.Join(outDir, fmt.Sprintf("%02d-%s.png", i+1, nombre(ruta)))
		if err := capturar(ctx, nav, unir(base, ruta), foto); err != nil {
			return fotos, fmt.Errorf("captura de %s · %w", ruta, err)
		}
		fotos = append(fotos, foto)
	}
	return fotos, nil
}

func esperar(ctx context.Context, url string, tope time.Duration) error {
	fin := time.Now().Add(tope)
	cliente := http.Client{Timeout: 3 * time.Second}
	for time.Now().Before(fin) {
		if r, err := cliente.Get(url); err == nil {
			r.Body.Close()
			if r.StatusCode < 500 {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return errors.New("se acabó la espera")
}

func capturar(ctx context.Context, nav, url, foto string) error {
	perfil, err := os.MkdirTemp("", "devclean-nav-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(perfil)
	args := []string{
		"--disable-gpu", "--hide-scrollbars", "--no-first-run", "--no-default-browser-check",
		"--user-data-dir=" + perfil,
		fmt.Sprintf("--window-size=%d,%d", Ancho, Alto),
		"--virtual-time-budget=6000",
		"--screenshot=" + foto, url,
	}
	if !strings.Contains(filepath.Base(nav), "headless-shell") {
		args = append([]string{"--headless=new"}, args...)
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, nav, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v · %s", err, ultimas(string(out)))
	}
	if _, err := os.Stat(foto); err != nil {
		return errors.New("el navegador no escribió la captura")
	}
	return nil
}

// unir pega la ruta a la raíz sin duplicar ni perder la barra.
func unir(base, ruta string) string {
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(ruta, "/")
}

var noNombre = regexp.MustCompile(`[^a-z0-9]+`)

func nombre(ruta string) string {
	n := strings.Trim(noNombre.ReplaceAllString(strings.ToLower(ruta), "-"), "-")
	if n == "" {
		return "inicio"
	}
	return n
}

func ultimas(s string) string {
	l := strings.Split(strings.TrimSpace(s), "\n")
	if len(l) > 3 {
		l = l[len(l)-3:]
	}
	return strings.Join(l, " · ")
}
