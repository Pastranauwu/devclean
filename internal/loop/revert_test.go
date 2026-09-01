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

	revertidos, err := revertFueraDeAlcance(root, []string{"src/export/**"}, []string{"*_test.go"})
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

	if _, err := revertFueraDeAlcance(root, []string{"src/export/**"}, nil); err != nil {
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

	revertidos, err := revertFueraDeAlcance(root, nil, []string{"*_test.go"})
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

func TestStagedStatsConArchivoNuevo(t *testing.T) {
	root := repoConCommit(t)
	escribir(t, root, "src/export/writer.go", "linea1\nlinea2\n")

	if _, err := gitRun(root, "add", "-A"); err != nil {
		t.Fatal(err)
	}
	archivos, err := stagedFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(archivos) != 1 || archivos[0] != "src/export/writer.go" {
		t.Errorf("archivos = %v", archivos)
	}
	mas, menos, err := stagedNumstat(root)
	if err != nil {
		t.Fatal(err)
	}
	if mas != 2 || menos != 0 {
		t.Errorf("numstat = +%d/-%d, quiero +2/-0", mas, menos)
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
