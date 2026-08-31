package loop

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Pastranauwu/devclean/internal/sealed"
)

// examinadorFalso cuenta invocaciones: lo que se verifica es si el bucle
// lo llamó o no, no lo que produce.
type examinadorFalso struct{ veces int }

func (e *examinadorFalso) Run(context.Context, string) (bool, error) {
	e.veces++
	return true, nil
}

func agenteQueTermina(t *testing.T) *agenteFalso {
	t.Helper()
	ag := &agenteFalso{nombre: "falso"}
	ag.hacer = func(_ int, req Request) (string, int, error) {
		escribir(t, req.RoomPath, "src/done.txt", "ok")
		return "", 0, nil
	}
	return ag
}

// una suite sellada a mano manda: el examinador automático no corre y no
// la pisa.
func TestRunNoExaminaConSuiteSelladaAMano(t *testing.T) {
	root := repoConCommit(t)
	tk := tareaDePrueba()
	if err := sealed.Write(root, tk.ID, sealed.SuiteOculta{
		Content:        "def test_oculta():\n    assert True\n",
		Archivo:        "src/test_devclean_hidden.py",
		Visible:        "def test_visible():\n    assert True\n",
		ArchivoVisible: "src/test_devclean_visible.py",
	}); err != nil {
		t.Fatal(err)
	}

	o := optsDePrueba(t, root, agenteQueTermina(t), tk)
	exam := &examinadorFalso{}
	o.Examinador = exam

	res, err := Run(context.Background(), o)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Verde {
		t.Fatalf("la tarea debió quedar verde: %+v", res)
	}
	if exam.veces != 0 {
		t.Errorf("el examinador corrió %d veces pese a la suite sellada a mano", exam.veces)
	}

	// la suite visible manual aterriza en el cuarto, en la ruta sellada
	visible := filepath.Join(o.Room.Path, "src", "test_devclean_visible.py")
	data, err := os.ReadFile(visible)
	if err != nil {
		t.Fatalf("la suite visible manual no llegó al cuarto: %v", err)
	}
	if string(data) != "def test_visible():\n    assert True\n" {
		t.Errorf("contenido de la visible = %q", data)
	}

	// y queda commiteada, o revertFueraDeAlcance (A.3) la borraría en el
	// primer intento
	if salida, err := gitRun(o.Room.Path, "log", "--oneline", "--", "src/test_devclean_visible.py"); err != nil || salida == "" {
		t.Errorf("la suite visible manual no quedó commiteada: %q %v", salida, err)
	}

	// sellar a mano no toca la oculta: ship la lee igual que la automática
	s, err := sealed.Read(root, tk.ID)
	if err != nil {
		t.Fatalf("sealed.Read: %v", err)
	}
	if s.Archivo != "src/test_devclean_hidden.py" {
		t.Errorf("la oculta cambió de ruta: %q", s.Archivo)
	}
}

// sin suite sellada el examinador automático corre como siempre.
func TestRunExaminaSinSuiteSellada(t *testing.T) {
	root := repoConCommit(t)
	o := optsDePrueba(t, root, agenteQueTermina(t), tareaDePrueba())
	exam := &examinadorFalso{}
	o.Examinador = exam

	if _, err := Run(context.Background(), o); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if exam.veces != 1 {
		t.Errorf("el examinador debió correr una vez, corrió %d", exam.veces)
	}
}
