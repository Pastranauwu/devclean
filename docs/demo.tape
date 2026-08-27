# Tape de vhs para grabar el GIF de devclean.
#   vhs docs/demo.tape
# Genera docs/demo.gif con la demo de scripts/demo.sh (agente falso, sin
# gastar tokens). Para uso real, edita scripts/demo.sh y quita el fake.

Output docs/demo.gif

Set Shell bash
Set FontSize 18
Set Width 120
Set Height 40
Set Padding 0

Type "go build -o devclean ./cmd/devclean"
Enter
Sleep 2s

Type "sh scripts/demo.sh"
Enter
Sleep 25s
