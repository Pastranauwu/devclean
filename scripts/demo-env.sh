#!/bin/sh
# Prepara el entorno para grabar la TUI de devclean: agente falso con pausa
# (para que se vea la animación) y un repo de demo en /tmp/devclean-demo.
# No corre ningún comando de devclean; eso lo hace el tape con la TUI.
set -e

raiz="$(cd "$(dirname "$0")/.." && pwd)"
bin="${DEVCLEAN_BIN:-$raiz/devclean}"
if [ ! -x "$bin" ]; then
  echo "compilando devclean..."
  (cd "$raiz" && go build -o devclean ./cmd/devclean) || exit 1
fi

# agente falso
mkdir -p /tmp/fakebin
cat > /tmp/fakebin/opencode <<'EOF'
#!/bin/sh
if [ "$1" = "--version" ]; then echo "0.1.0"; exit 0; fi
dir="."; prev=""
for a in "$@"; do [ "$prev" = "--dir" ] && dir="$a"; prev="$a"; done
case "$2" in
  *"planificador de devclean"*)
    printf '%s\n' '{"type":"message","part":{"type":"text","text":"[{\"titulo\":\"crear el módulo de exportación\",\"listo_cuando\":\"test -f src/export.go\",\"tocar_solo\":[\"src/**\"]},{\"titulo\":\"documentar la API\",\"listo_cuando\":\"test -f docs/api.md\",\"tocar_solo\":[\"docs/**\"]}]"}}'
    printf '%s\n' '{"type":"step_finish","tokens":{"input":10,"output":5}}'
    ;;
  *)
    path=$(printf '%s' "$2" | grep -o 'test -f [^ ]*' | awk '{print $3}')
    [ -n "$path" ] && { mkdir -p "$dir/$(dirname "$path")"; echo "demo" > "$dir/$path"; }
    sleep 1
    printf '%s\n' '{"type":"step_finish","tokens":{"input":100,"output":20}}'
    ;;
esac
EOF
chmod +x /tmp/fakebin/opencode
cp "$bin" /tmp/fakebin/devclean

# repo de demo en ruta fija
rm -rf /tmp/devclean-demo
mkdir /tmp/devclean-demo
cd /tmp/devclean-demo
git init -b main -q
git -c user.email=t@t -c user.name=t commit --allow-empty -m init -q
PATH="/tmp/fakebin:$PATH" /tmp/fakebin/devclean init --pruebas true --plain >/dev/null
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
# en la misma oleada (§6.9), que es un chequeo real y correcto, no un bug.
cat > .devclean/tasks/T-003.md.pendiente <<'EOF'
---
version: 1
id: T-003
titulo: preparar exportación completa (módulo + docs)
listo_cuando: test -f src/export.go && test -f docs/api.md
tocar_solo: ["src/**", "docs/**"]
limite_intentos: 3
limite_lineas: 200
recursivo: true
limite_subtareas: 2
---
EOF
echo "demo lista en /tmp/devclean-demo"
