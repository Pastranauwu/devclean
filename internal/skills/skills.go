// Package skills trae e inyecta skills reales (SKILL.md, formato de
// skills.sh / Claude Code) en el prompt de cada agente, en vez del simple
// nombre-etiqueta que usaba config.Agente.Skills. El fetch corre una vez
// contra la raíz del repo (nunca dentro de un cuarto: los cuartos son
// worktrees separados y no ven archivos sin commitear de otro worktree),
// y el contenido se inyecta como texto — así cualquier ejecutor (claude,
// opencode) lo recibe igual, sin depender de que la CLI del agente
// descubra skills por su cuenta en modo headless.
package skills

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Source es un paquete de skill instalable.
type Source struct {
	// Nombre es el slug con el que devclean identifica la skill en todos
	// lados: config.yml, DefaultAgentes, y la carpeta instalada
	// (.agents/skills/<Nombre>/).
	Nombre string
	Repo   string
	// SkillArg es el valor exacto que exige `--skill` en el repo de
	// origen. Casi siempre es igual a Nombre; algunos repos (p. ej.
	// anthropics/claude-code) publican el título tal cual ("Agent
	// Development") y no su slug, aunque el CLI igual instala en la
	// carpeta con el slug. Vacío = usar Nombre.
	SkillArg string
}

// arg devuelve el valor a pasar a `--skill`.
func (s Source) arg() string {
	if s.SkillArg != "" {
		return s.SkillArg
	}
	return s.Nombre
}

// DefaultSources son los paquetes que devclean trae por defecto: la base
// que se inyecta en todo agente, más los específicos por rol.
func DefaultSources() []Source {
	return []Source{
		{Nombre: "caveman", Repo: "https://github.com/juliusbrussee/caveman"},
		{Nombre: "grill-me", Repo: "https://github.com/mattpocock/skills"},
		{Nombre: "improve-codebase-architecture", Repo: "https://github.com/mattpocock/skills"},
		{Nombre: "implement", Repo: "https://github.com/mattpocock/skills"},
		{Nombre: "code-review", Repo: "https://github.com/mattpocock/skills"},
		{Nombre: "clean-code", Repo: "https://github.com/sickn33/agentic-awesome-skills"},
		{Nombre: "clean-architecture", Repo: "https://github.com/pproenca/dot-skills"},
		{Nombre: "agent-development", Repo: "https://github.com/anthropics/claude-code", SkillArg: "Agent Development"},
		{Nombre: "frontend-design", Repo: "https://github.com/anthropics/skills"},
		{Nombre: "create-a-backend", Repo: "https://github.com/vercel/vercel-plugin"},
		{Nombre: "test-driven-development", Repo: "https://github.com/obra/superpowers"},
	}
}

// BaseSkillNames son los paquetes que devclean inyecta en cualquier
// agente, sea cual sea su rol.
func BaseSkillNames() []string {
	return []string{
		"caveman", "grill-me", "improve-codebase-architecture", "implement",
		"code-review", "clean-code", "clean-architecture", "agent-development",
	}
}

// FrontendSkillName, BackendSkillName y PMSkillName son el paquete extra
// que cada rol agrega sobre la base.
func FrontendSkillName() string { return "frontend-design" }
func BackendSkillName() string  { return "create-a-backend" }
func PMSkillName() string       { return "test-driven-development" }

// Dir es el directorio canónico donde el CLI `skills` instala el
// contenido (`.agents/skills/<nombre>/SKILL.md`), siempre a raíz del
// repo — no del cuarto.
func Dir(root string) string { return filepath.Join(root, ".agents", "skills") }

// Instalado reporta si un paquete ya está descargado.
func Instalado(root, nombre string) bool {
	_, err := os.Stat(filepath.Join(Dir(root), nombre, "SKILL.md"))
	return err == nil
}

// Fetch trae un paquete con `npx skills add`. Idempotente: si ya está
// instalado, no hace nada. Degrada con error legible si falta npx o si
// el fetch falla (sin red, repo sin ese skill); el llamador decide si
// bloquea o sigue sin esa skill.
func Fetch(ctx context.Context, root string, src Source) error {
	if Instalado(root, src.Nombre) {
		return nil
	}
	if _, err := exec.LookPath("npx"); err != nil {
		return fmt.Errorf("npx no está instalado · instala Node.js para traer skills")
	}
	cmd := exec.CommandContext(ctx, "npx", "--yes", "skills", "add", src.Repo,
		"--skill", src.arg(), "-y", "--agent", "universal")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("no se pudo traer skill %s · %s", src.Nombre, tail(string(out)))
	}
	if !Instalado(root, src.Nombre) {
		return fmt.Errorf("skill %s no apareció en el repo tras el fetch · revisa que el paquete la incluya", src.Nombre)
	}
	return nil
}

// Skill es el contenido parseado de un SKILL.md.
type Skill struct {
	Nombre      string
	Descripcion string
	Cuerpo      string
}

// Read lee y parsea el SKILL.md de un paquete ya instalado.
func Read(root, nombre string) (Skill, error) {
	path := filepath.Join(Dir(root), nombre, "SKILL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return Skill{}, err
	}
	return parseSkillMD(nombre, string(data)), nil
}

// parseSkillMD separa el frontmatter YAML mínimo (name/description) del
// cuerpo. No usa un parser YAML completo: el frontmatter de SKILL.md es
// siempre `clave: valor` plano entre dos líneas `---`.
func parseSkillMD(nombre, data string) Skill {
	s := Skill{Nombre: nombre}
	lines := strings.Split(data, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		s.Cuerpo = strings.TrimSpace(data)
		return s
	}
	i := 1
	for ; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			break
		}
		key, val, ok := strings.Cut(lines[i], ":")
		if !ok || strings.TrimSpace(key) != "description" {
			continue
		}
		val = strings.TrimSpace(val)
		if val != ">" && val != "|" {
			s.Descripcion = strings.Trim(val, "> ")
			continue
		}
		// folded scalar (`description: >`): las líneas indentadas que
		// siguen son el valor, hasta la primera sin indentar.
		var partes []string
		for i++; i < len(lines) && (lines[i] == "" || strings.HasPrefix(lines[i], " ") || strings.HasPrefix(lines[i], "\t")); i++ {
			if t := strings.TrimSpace(lines[i]); t != "" {
				partes = append(partes, t)
			}
		}
		i-- // el for externo vuelve a avanzar
		s.Descripcion = strings.Join(partes, " ")
	}
	if i+1 < len(lines) {
		s.Cuerpo = strings.TrimSpace(strings.Join(lines[i+1:], "\n"))
	}
	return s
}

// Content junta el cuerpo de cada skill nombrada que ya está instalada,
// lista para inyectar en el prompt. Una skill que falta se salta en
// silencio — nunca bloquea al agente por una skill no instalada, mismo
// criterio de degradación que el examinador y la constitución.
func Content(root string, nombres []string) string {
	var b strings.Builder
	for _, n := range nombres {
		sk, err := Read(root, n)
		if err != nil {
			continue
		}
		fmt.Fprintf(&b, "### Skill: %s\n%s\n\n", n, sk.Cuerpo)
	}
	return strings.TrimSpace(b.String())
}

func tail(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > 3 {
		lines = lines[len(lines)-3:]
	}
	return strings.Join(lines, " · ")
}
