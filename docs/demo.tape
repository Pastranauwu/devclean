# Tape de vhs para grabar el GIF de devclean CON la TUI.
#   vhs docs/demo.tape
# Graba run (corrida en vivo), board (plasma) y ship (compuerta), sin --plain.
# OJO: en vhs, Type manda los caracteres literal (no escapa $). Escribir
# \$PATH rompería el PATH. Aquí va $PATH tal cual.
# En vhs, Width y Height son PÍXELES (tamaño de la imagen final).

Output docs/demo.gif

Set Shell bash
Set FontSize 22
Set Width 1200
Set Height 700
Set Padding 2

# compila si hace falta + prepara el agente falso y el repo de demo
Type "sh scripts/demo-env.sh"
Enter
Sleep 20s

Type "cd /tmp/devclean-demo"
Enter
Sleep 1s

Type "export PATH=/tmp/fakebin:$PATH"
Enter
Sleep 1s

# run: la corrida en vivo (spinner + barra), sale solo
Type "devclean run --agentes 2 --ejecutor opencode"
Enter
Sleep 12s

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
