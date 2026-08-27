# Tape de vhs para grabar el GIF de devclean.
#   vhs docs/demo.tape
# En vhs, Width y Height son PÍXELES (tamaño de la imagen), no columnas.
# 1200x700 con FontSize 22 da una terminal de ~90 columnas, cómoda para el
# contenido de 80 columnas. La demo usa scripts/demo.sh (agente falso).

Output docs/demo.gif

Set Shell bash
Set FontSize 22
Set Width 1200
Set Height 700
Set Padding 2
Set WaitTimeout 60s

Type "go build -o devclean ./cmd/devclean"
Enter
Sleep 3s

Type "sh scripts/demo.sh"
Enter
Sleep 5s
