package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSkill(t *testing.T, root, nombre, contenido string) {
	t.Helper()
	dir := filepath.Join(Dir(root), nombre)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(contenido), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReadParseaFrontmatterYCuerpo(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "caveman", "---\nname: caveman\ndescription: >\n  habla breve\n---\n\nRespond terse.\n")

	sk, err := Read(root, "caveman")
	if err != nil {
		t.Fatal(err)
	}
	if sk.Descripcion != "habla breve" {
		t.Errorf("Descripcion = %q, quiero %q", sk.Descripcion, "habla breve")
	}
	if sk.Cuerpo != "Respond terse." {
		t.Errorf("Cuerpo = %q, quiero %q", sk.Cuerpo, "Respond terse.")
	}
}

func TestInstalado(t *testing.T) {
	root := t.TempDir()
	if Instalado(root, "caveman") {
		t.Error("Instalado debió ser false sin fetch")
	}
	writeSkill(t, root, "caveman", "cuerpo")
	if !Instalado(root, "caveman") {
		t.Error("Instalado debió ser true tras escribir el SKILL.md")
	}
}

func TestContentSaltaSkillsFaltantesSinBloquear(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "clean-code", "---\nname: clean-code\n---\n\nfunciones chicas.")

	got := Content(root, []string{"clean-code", "no-existe"})
	if got == "" {
		t.Fatal("Content vacío, esperaba el cuerpo de clean-code")
	}
	if !strings.Contains(got, "funciones chicas.") {
		t.Errorf("Content sin el cuerpo instalado: %q", got)
	}
}

func TestContentVacioSinSkillsInstaladas(t *testing.T) {
	root := t.TempDir()
	if got := Content(root, []string{"no-existe"}); got != "" {
		t.Errorf("Content = %q, quiero vacío", got)
	}
}
