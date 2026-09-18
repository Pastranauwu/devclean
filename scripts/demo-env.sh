#!/bin/sh
# Prepara el entorno para grabar la TUI de devclean: agente falso con pausa
# (para que se vea la animación) y un repo de demo en /tmp/devclean-demo.
# No corre ningún comando de devclean; eso lo hace el tape con la TUI.
set -e

raiz="$(cd "$(dirname "$0")/.." && pwd)"
bin="${DEVCLEAN_BIN:-$raiz/devclean}"
# siempre recompila: go build es incremental y un binario viejo grababa
# una demo que ya no corresponde al código del repo
if [ -z "$DEVCLEAN_BIN" ]; then
  echo "compilando devclean..."
  (cd "$raiz" && go build -o devclean ./cmd/devclean) || exit 1
fi

# agente falso
mkdir -p /tmp/fakebin
cat > /tmp/fakebin/opencode <<'EOF'
#!/bin/sh
if [ "$1" = "--version" ]; then echo "0.1.0"; exit 0; fi
# catalogo de modelos: sin esto init guardaba la salida JSON del agente
# como id de modelo y la corrida lo imprimia como nombre
if [ "$1" = "models" ]; then
  printf '%s\n' anthropic/claude-haiku-4-5 anthropic/claude-sonnet-4-5 anthropic/claude-opus-4-1
  exit 0
fi
dir="."; prev=""
for a in "$@"; do [ "$prev" = "--dir" ] && dir="$a"; prev="$a"; done
case "$2" in
  *"planificador de devclean"*)
    printf '%s\n' '{"type":"message","part":{"type":"text","text":"[{\"titulo\":\"exportador CSV\",\"listo_cuando\":\"test -f src/csv/export.go\",\"tocar_solo\":[\"src/csv/**\"]},{\"titulo\":\"exportador JSON\",\"listo_cuando\":\"test -f src/json/export.go\",\"tocar_solo\":[\"src/json/**\"]}]"}}'
    printf '%s\n' '{"type":"step_finish","tokens":{"input":10,"output":5}}'
    ;;
  *)
    for path in $(printf '%s' "$2" | grep -o 'test -f [^ ]*' | awk '{print $3}'); do
      mkdir -p "$dir/$(dirname "$path")"
      echo "demo" > "$dir/$path"
      printf '%s\n' "{\"type\":\"message\",\"part\":{\"type\":\"text\",\"text\":\"escribiendo $path\"}}"
    done
    sleep 1
    printf '%s\n' '{"type":"step_finish","tokens":{"input":100,"output":20}}'
    ;;
esac
EOF
chmod +x /tmp/fakebin/opencode
cp "$bin" /tmp/fakebin/devclean

# HOME aislado: el ledger de consumo vive en $HOME/.devclean, y sin esto
# el GIF grabaria el gasto real de quien graba. El tape exporta este HOME.
casa=/tmp/devclean-demo-home
rm -rf "$casa"
mkdir -p "$casa"
cat > "$casa/.gitconfig" <<'GITCFG'
[user]
	name = devclean demo
	email = demo@devclean.local
[init]
	defaultBranch = main
GITCFG

# repo de demo en ruta fija
rm -rf /tmp/devclean-demo
mkdir /tmp/devclean-demo
cd /tmp/devclean-demo
git init -b main -q
git -c user.email=t@t -c user.name=t commit --allow-empty -m init -q
HOME="$casa" PATH="/tmp/fakebin:$PATH" /tmp/fakebin/devclean init --pruebas true --plain >/dev/null
echo "recursion_max: 1" >> .devclean/config.yml

cat > .devclean/tasks/T-001.md <<'EOF'
---
version: 1
id: T-001
titulo: crear el módulo de exportación
listo_cuando: test -f src/export.go
tocar_solo: ["src/**"]
limite_intentos: 3
limite_lineas: 200
---
EOF
cat > .devclean/tasks/T-002.md <<'EOF'
---
version: 1
id: T-002
titulo: documentar la API
listo_cuando: test -f docs/api.md
tocar_solo: ["docs/**"]
limite_intentos: 3
limite_lineas: 200
---
EOF

# T-003 recursiva: llega como .pendiente, el tape la activa (mv) después
# de que T-001/T-002 ya corrieron — si entrara pendiente desde el arranque,
# la esclusa de entrada la rechaza por cruce de tocar_solo con T-001/T-002
# en la misma oleada, que es un chequeo real y correcto, no un bug.
cat > .devclean/tasks/T-003.md.pendiente <<'EOF'
---
version: 1
id: T-003
titulo: exportar en CSV y en JSON
listo_cuando: test -f src/csv/export.go && test -f src/json/export.go
tocar_solo: ["src/**"]
limite_intentos: 3
limite_lineas: 200
recursivo: true
limite_subtareas: 2
---
EOF
echo "demo lista en /tmp/devclean-demo"
