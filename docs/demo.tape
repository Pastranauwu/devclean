# Tape de vhs para grabar el GIF de devclean CON la TUI.
#   vhs docs/demo.tape
# Graba run (corrida en vivo), board (plasma) y ship (compuerta), sin --plain.
# En vhs, Width y Height son PÍXELES (tamaño de la imagen final).

Output docs/demo.gif

Set Shell bash
Set FontSize 22
Set Width 1200
Set Height 700
Set Padding 2
Set WaitTimeout 30s

Type "go build -o devclean ./cmd/devclean"
Enter
Sleep 3s

Type "sh scripts/demo-env.sh"
Enter
Sleep 3s

Type "cd /tmp/devclean-demo && export PATH=/tmp/fakebin:\$PATH"
Enter
Sleep 500ms

# run: la corrida en vivo (spinner + barra), sale solo
Type "devclean run --agentes 2 --ejecutor opencode"
Enter
Sleep 15s

# board: plasma de fondo + logo, sal con q
Type "devclean board"
Enter
Sleep 4s
Type "q"
Sleep 1s

# ship: la compuerta animada, sale solo
Type "devclean ship T-001 --dry-run"
Enter
Sleep 10s
