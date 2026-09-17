package loop

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRevertFueraDeAlcance(t *testing.T) {
	root := repoConCommit(t)
	escribir(t, root, "src/export/base.go", "package export\n")

	// dentro de alcance: se queda
	escribir(t, root, "src/export/writer.go", "nuevo dentro\n")
	// fuera de alcance: se revierte
	escribir(t, root, "src/auth/login.go", "nuevo fuera\n")
	// archivo de prueba: se revierte aunque caiga dentro del alcance
	escribir(t, root, "src/export/writer_test.go", "prueba\n")

	revertidos, err := revertFueraDeAlcance(root, "HEAD", []string{"src/export/**"}, []string{"*_test.go"})
	if err != nil {
		t.Fatalf("revert: %v", err)
	}
	if len(revertidos) != 2 {
		t.Fatalf("revertidos = %v, quiero 2", revertidos)
	}
	if _, err := os.Stat(filepath.Join(root, "src/auth/login.go")); !os.IsNotExist(err) {
		t.Error("el archivo fuera de alcance sigue en disco")
	}
	if _, err := os.Stat(filepath.Join(root, "src/export/writer_test.go")); !os.IsNotExist(err) {
		t.Error("el archivo de prueba sigue en disco")
	}
	if _, err := os.Stat(filepath.Join(root, "src/export/writer.go")); err != nil {
		t.Error("el archivo dentro de alcance desapareció")
	}
}

func TestRevertArchivoExistenteModificado(t *testing.T) {
	root := repoConCommit(t)
	escribir(t, root, "src/auth/login.go", "original\n")
	gitCmd(t, root, "add", "-A")
	gitCmd(t, root, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-m", "base")

	escribir(t, root, "src/auth/login.go", "modificado por el agente\n")

	if _, err := revertFueraDeAlcance(root, "HEAD", []string{"src/export/**"}, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "src/auth/login.go"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "original\n" {
		t.Errorf("el archivo modificado no volvió a HEAD: %q", data)
	}
}

func TestRevertSinRestriccionRevierteSoloPruebas(t *testing.T) {
	// tocar_solo vacío = sin restricción: solo las pruebas se revierten
	root := repoConCommit(t)
	escribir(t, root, "src/x.go", "codigo\n")
	escribir(t, root, "src/x_test.go", "prueba\n")

	revertidos, err := revertFueraDeAlcance(root, "HEAD", nil, []string{"*_test.go"})
	if err != nil {
		t.Fatal(err)
	}
	if len(revertidos) != 1 || revertidos[0] != "src/x_test.go" {
		t.Errorf("revertidos = %v, quiero solo la prueba", revertidos)
	}
	if _, err := os.Stat(filepath.Join(root, "src/x.go")); err != nil {
		t.Error("el código debió quedar")
	}
}

func TestStatsConArchivoNuevo(t *testing.T) {
	root := repoConCommit(t)
	antes := cabeza(t, root)
	escribir(t, root, "src/export/writer.go", "linea1\nlinea2\n")

	if _, err := gitRun(root, "add", "-A"); err != nil {
		t.Fatal(err)
	}
	comprobarStats(t, root, antes, "src/export/writer.go", 2, 0)
}

// El agente real commitea por su cuenta dentro del cuarto (la skill
// `implement` lo hace). Medir con `git diff --cached HEAD` daba entonces
// cero archivos y cero líneas pese a haber trabajo, y esa mentira se
// escribía en attempts.jsonl, que es la fuente de las métricas, del
// standup y del solapamiento semántico.
func TestStatsCuandoElAgenteCommiteaSolo(t *testing.T) {
	root := repoConCommit(t)
	antes := cabeza(t, root)

	escribir(t, root, "src/export/writer.go", "linea1\nlinea2\n")
	gitCmd(t, root, "add", "-A")
	gitCmd(t, root, "-c", "user.email=a@a", "-c", "user.name=a", "commit", "-m", "feat: lo hice yo solo")

	// el bucle indexa igual antes de medir; aquí no queda nada que indexar
	if _, err := gitRun(root, "add", "-A"); err != nil {
		t.Fatal(err)
	}
	comprobarStats(t, root, antes, "src/export/writer.go", 2, 0)
}

// cabeza devuelve el commit con que arranca un intento.
func cabeza(t *testing.T, root string) string {
	t.Helper()
	h, err := resolveCommit(root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func comprobarStats(t *testing.T, root, ref, quiero string, mas, menos int) {
	t.Helper()
	archivos, err := filesSince(root, ref)
	if err != nil {
		t.Fatal(err)
	}
	if len(archivos) != 1 || archivos[0] != quiero {
		t.Errorf("archivos = %v, quiero [%s]", archivos, quiero)
	}
	a, m, err := numstatSince(root, ref)
	if err != nil {
		t.Fatal(err)
	}
	if a != mas || m != menos {
		t.Errorf("numstat = +%d/-%d, quiero +%d/-%d", a, m, mas, menos)
	}
}

// El lockfile derivado de un manifiesto en alcance no se revierte:
// hacerlo dejaba "missing go.sum entry" en cada intento, para siempre.
func TestEnAlcanceLockfileDerivado(t *testing.T) {
	scope := []string{"go.mod", "cmd/**"}
	if !enAlcance("go.sum", scope) {
		t.Error("go.sum debe estar en alcance cuando go.mod lo está")
	}
	if !enAlcance("go.mod", scope) {
		t.Error("go.mod declarado debe estar en alcance")
	}
	// sin el manifiesto en alcance, el lockfile sigue vedado
	if enAlcance("go.sum", []string{"cmd/**"}) {
		t.Error("go.sum sin go.mod en alcance no debe pasar")
	}
	// otro stack, mismo trato
	if !enAlcance("package-lock.json", []string{"package.json"}) {
		t.Error("package-lock.json debe seguir a package.json")
	}
	if enAlcance("Cargo.lock", []string{"go.mod"}) {
		t.Error("un lockfile de otro stack no entra por la puerta de atrás")
	}
}

// El agente que commitea por su cuenta dentro del cuarto dejaba el árbol
// limpio, y la reversión —que solo miraba `git status`— no veía nada: sus
// propias pruebas y cualquier archivo fuera de alcance llegaban al PR.
func TestRevertCuandoElAgenteCommiteaSolo(t *testing.T) {
	root := repoConCommit(t)
	escribir(t, root, "src/export/base.go", "package export\n")
	gitCmd(t, root, "add", "-A")
	gitCmd(t, root, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-m", "base")
	antes := cabeza(t, root)

	escribir(t, root, "src/export/writer.go", "nuevo dentro\n")
	escribir(t, root, "src/export/writer_test.go", "prueba propia\n")
	escribir(t, root, "src/auth/login.go", "fuera de alcance\n")
	escribir(t, root, "src/export/base.go", "package export // tocado\n")
	gitCmd(t, root, "add", "-A")
	gitCmd(t, root, "-c", "user.email=a@a", "-c", "user.name=a", "commit", "-m", "feat: lo commiteé yo solo")

	revertidos, err := revertFueraDeAlcance(root, antes, []string{"src/export/**"}, []string{"*_test.go"})
	if err != nil {
		t.Fatalf("revert: %v", err)
	}
	if len(revertidos) != 2 {
		t.Fatalf("revertidos = %v, quiero la prueba y el archivo fuera de alcance", revertidos)
	}
	if _, err := os.Stat(filepath.Join(root, "src/export/writer_test.go")); !os.IsNotExist(err) {
		t.Error("la prueba que se escribió el agente sigue en disco")
	}
	if _, err := os.Stat(filepath.Join(root, "src/auth/login.go")); !os.IsNotExist(err) {
		t.Error("el archivo fuera de alcance sigue en disco")
	}
	if _, err := os.Stat(filepath.Join(root, "src/export/writer.go")); err != nil {
		t.Error("el archivo dentro de alcance no debió tocarse")
	}
	// un archivo en alcance que ya existía sigue con el cambio del agente
	data, err := os.ReadFile(filepath.Join(root, "src/export/base.go"))
	if err != nil || string(data) != "package export // tocado\n" {
		t.Errorf("base.go = %q, quiero el cambio del agente intacto", data)
	}
}
