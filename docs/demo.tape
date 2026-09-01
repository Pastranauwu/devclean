# Tape de vhs para grabar el GIF de devclean CON la TUI.
#   vhs docs/demo.tape
# Graba run (corrida en vivo), board (plasma, con árbol de una tarea
# recursiva) y ship (compuerta), sin --plain.
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
Sleep 3s

Type "cd /tmp/devclean-demo"
Enter
Sleep 500ms

Type "export PATH=/tmp/fakebin:$PATH"
Enter
Sleep 500ms

# run: la corrida en vivo (spinner + barra), sale solo
Type "devclean run --agentes 2 --ejecutor opencode"
Enter
Sleep 8s

# board: plasma de fondo + logo, déjalo verse un rato
Type "devclean board"
Enter
Sleep 10s
Type "q"
Sleep 3s

# activa la tarea recursiva (ver scripts/demo-env.sh) y corre: devclean
# la reparte sola en subtareas, cada una en su propio cuarto anidado
Type "mv .devclean/tasks/T-003.md.pendiente .devclean/tasks/T-003.md"
Enter
Sleep 500ms
Type "devclean run --agentes 1 --ejecutor opencode"
Enter
Sleep 10s

# board --plain: ahora T-003 muestra su árbol de subtareas indentado.
# --plain a propósito acá (no TUI): entrar y salir de un segundo alt-screen
# en la misma sesión de vhs es frágil (la tecla "q" puede llegarle a la
# shell en vez de al programa si el timing no da), y el texto plano
# muestra el árbol con más claridad para un GIF de todos modos.
Type "devclean board --plain"
Enter
Sleep 6s

# ship: la compuerta animada, sale solo
Type "devclean ship T-001 --dry-run"
Enter
Sleep 5s
