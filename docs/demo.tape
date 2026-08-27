# Tape de vhs para grabar el GIF de devclean.
#   vhs docs/demo.tape
# 80 columnas (§16.2), letra grande y tema oscuro con la paleta del
# proyecto. La demo usa scripts/demo.sh (agente falso, sin gastar tokens).

Output docs/demo.gif

Set Shell bash
Set FontSize 22
Set Width 80
Set Height 26
Set Padding 2
Set Margin 10
Set CursorBlink false
Set Theme {
  "name": "devclean",
  "black": "#1a1d21",
  "red": "#D96C4A",
  "green": "#4FB3A2",
  "yellow": "#C9A227",
  "blue": "#6B6F72",
  "magenta": "#4FB3A2",
  "cyan": "#4FB3A2",
  "white": "#E6E6E1",
  "brightBlack": "#6B6F72",
  "brightRed": "#D96C4A",
  "brightGreen": "#4FB3A2",
  "brightYellow": "#C9A227",
  "brightBlue": "#6B6F72",
  "brightMagenta": "#4FB3A2",
  "brightCyan": "#4FB3A2",
  "brightWhite": "#E6E6E1",
  "background": "#141619",
  "foreground": "#E6E6E1",
  "cursor": "#4FB3A2",
  "selection": "#2a2d31"
}

Type "go build -o devclean ./cmd/devclean"
Enter
Sleep 3s

Type "sh scripts/demo.sh"
Enter
Sleep 30s
