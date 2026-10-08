package plan

import (
	"strings"
	"testing"
)

func TestCuerpoJSONSaltaLaProsa(t *testing.T) {
	texto := "Uso **{f'{campo}__isnull': False}** y la lista [1].\n```json\n{\"arquitectura\": \"a\", \"tareas\": [{\"titulo\": \"t\"}]}\n```\nlisto"
	if got := CuerpoJSON(texto); !strings.HasPrefix(got, `{"arquitectura"`) {
		t.Errorf("CuerpoJSON = %q", got)
	}
	if got := CuerpoJSON("sin json"); got != "" {
		t.Errorf("sin JSON debe ser vacío, es %q", got)
	}
	// roto: se devuelve desde la primera llave, para que el error sea el del plan
	if got := CuerpoJSON(`pre {"tareas": [`); got != `{"tareas": [` {
		t.Errorf("JSON roto = %q", got)
	}
}
