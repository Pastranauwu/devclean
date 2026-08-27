# Tape de vhs para grabar el GIF de devclean.
#   vhs docs/demo.tape
# Genera docs/demo.gif. La demo usa un agente falso (scripts/demo.sh) para
# no gastar tokens; para uso real, edita scripts/demo.sh y quita el fake.

Output docs/demo.gif

Set Shell bash
Set FontSize 18
Set Width 120
Set Height 40
Set Padding 0
Set PlaybackSpeed 1

Hide

Type "go build -o devclean ./cmd/devclean"
Enter
Sleep 1s

Type "sh scripts/demo.sh"
Enter
Sleep 20s

Show
