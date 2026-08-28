package ship

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Pastranauwu/devclean/internal/loop"
	"github.com/Pastranauwu/devclean/internal/sealed"
	"github.com/Pastranauwu/devclean/internal/task"
)

// verificarSuiteOculta runs the hidden test suite against the task's worktree.
// Returns:
//   - (nil, "", true) when there is no sealed suite — step skipped gracefully
//   - (&brecha, detalle, true) when hidden suite passed
//   - (&brecha, detalle, false) when hidden suite failed — block PR
func verificarSuiteOculta(ctx context.Context, root, roomPath string, t task.Task, pruebas string, timeout time.Duration) (brecha *float64, detalle string, ok bool) {
	s, err := sealed.Read(root, t.ID)
	if errors.Is(err, os.ErrNotExist) {
		return nil, "", true // no suite sealed, skip
	}
	if err != nil {
		return nil, "error leyendo suite sellada · " + err.Error(), false
	}

	hiddenPath := filepath.Join(roomPath, filepath.FromSlash(s.Archivo))
	if err := os.MkdirAll(filepath.Dir(hiddenPath), 0o755); err != nil {
		return nil, "no se pudo preparar la suite oculta · " + err.Error(), false
	}
	if err := os.WriteFile(hiddenPath, []byte(s.Content), 0o644); err != nil {
		return nil, "no se pudo escribir la suite oculta · " + err.Error(), false
	}
	defer func() {
		_ = os.Remove(hiddenPath)
		_ = sealed.Burn(root, t.ID)
	}()

	salida, code := runComando(ctx, roomPath, pruebas, timeout)
	pasaron, fallaron := loop.ParseTestCounts(salida)
	brechaVal := calcularBrecha(root, t.ID, pasaron, fallaron)

	if code != nil && *code == 0 {
		detalle = "suite oculta superada"
		if brechaVal != nil {
			detalle = fmt.Sprintf("suite oculta superada · brecha=%.1f%%", *brechaVal)
		}
		return brechaVal, detalle, true
	}

	brechaStr := "sin datos"
	if brechaVal != nil {
		brechaStr = fmt.Sprintf("%.1f%%", *brechaVal)
	}
	detalle = fmt.Sprintf(
		"suite oculta falló · brecha=%s · requiere nuevo examinador",
		// ponytail: --reexaminar flag not yet implemented, referenced for future UX
		brechaStr,
	)
	return brechaVal, detalle, false
}

// calcularBrecha computes visible_pct - hidden_pct.
// visible_pct comes from the last attempts.jsonl entry.
// Returns nil when either side has no data.
func calcularBrecha(root, id string, hiddenPasaron, hiddenFallaron *int) *float64 {
	if hiddenPasaron == nil && hiddenFallaron == nil {
		return nil
	}
	hiddenTotal := 0
	if hiddenPasaron != nil {
		hiddenTotal += *hiddenPasaron
	}
	if hiddenFallaron != nil {
		hiddenTotal += *hiddenFallaron
	}
	if hiddenTotal == 0 {
		return nil
	}
	hiddenPct := 0.0
	if hiddenPasaron != nil {
		hiddenPct = float64(*hiddenPasaron) / float64(hiddenTotal) * 100
	}

	as, err := loop.ReadAttempts(root, id)
	if err != nil || len(as) == 0 {
		return nil
	}
	last := as[len(as)-1]
	if last.TestsPasaron == nil {
		return nil
	}
	visibleTotal := 0
	if last.TestsPasaron != nil {
		visibleTotal += *last.TestsPasaron
	}
	if last.TestsFallaron != nil {
		visibleTotal += *last.TestsFallaron
	}
	if visibleTotal == 0 {
		return nil
	}
	visiblePct := float64(*last.TestsPasaron) / float64(visibleTotal) * 100
	b := visiblePct - hiddenPct
	return &b
}
