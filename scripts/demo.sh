#!/bin/sh
# Demo reproducible de devclean, sin gastar tokens: un agente falso que
# resuelve las tareas. Para grabar el GIF:
#   vhs docs/demo.tape
# Para una demo con agentes reales, quita FAKE=1 y usa --ejecutor opencode.
set -e

bin="${DEVCLEAN_BIN:-$(dirname "$0")/../devclean}"
[ -x "$bin" ] || { echo "compila primero: go build -o devclean ./cmd/devclean"; exit 1; }

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# agente falso: escribe el archivo que pide cada listo_cuando
mkdir -p "$tmp/bin"
cat > "$tmp/bin/opencode" <<'EOF'
#!/bin/sh
if [ "$1" = "--version" ]; then echo "0.1.0"; exit 0; fi
dir="."; prev=""
for a in "$@"; do [ "$prev" = "--dir" ] && dir="$a"; prev="$a"; done
case "$2" in
  *"planificador de devclean"*)
    printf '%s\n' '{"type":"message","part":{"type":"text","text":"[{\"titulo\":\"exportar clientes a CSV\",\"listo_cuando\":\"test -f src/export.go\",\"tocar_solo\":[\"src/**\"]},{\"titulo\":\"documentar la API\",\"listo_cuando\":\"test -f docs/api.md\",\"tocar_solo\":[\"docs/**\"]}]"}}'
    printf '%s\n' '{"type":"step_finish","tokens":{"input":10,"output":5}}'
    ;;
  *)
    path=$(printf '%s' "$2" | grep -o 'test -f [^ ]*' | awk '{print $3}')
    [ -n "$path" ] && { mkdir -p "$dir/$(dirname "$path")"; echo "demo" > "$dir/$path"; }
    printf '%s\n' '{"type":"step_finish","tokens":{"input":100,"output":20}}'
    ;;
esac
EOF
chmod +x "$tmp/bin/opencode"
export PATH="$tmp/bin:$PATH"

# repo de demo
repo="$tmp/repo"
mkdir "$repo" && cd "$repo"
git init -b main -q
git -c user.email=t@t -c user.name=t commit --allow-empty -m init -q

run() { echo "\$ $*"; "$@"; echo; sleep 1; }

run "$bin" init --pruebas "true"
run "$bin" plan "necesito exportar clientes y documentar la API" --aprobar
run "$bin" board
run "$bin" run --agentes 2 --ejecutor opencode
run "$bin" board
run "$bin" logs T-001
run "$bin" ship T-001 --dry-run
run "$bin" report
