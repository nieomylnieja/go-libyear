Output docs/assets/history.gif

Require go

Set Shell "bash"
Set Theme "nord"
Set FontSize 18
Set Width 1280
Set Height 510
Set Padding 20
Set Framerate 40
Set TypingSpeed 20ms
Set PlaybackSpeed 0.5
Set WaitTimeout 10m

Hide
Type "export PS1='$ '"
Enter
Type "alias go-libyear='go run ./cmd/go-libyear'"
Enter
Type "clear"
Enter
Show

Type "go-libyear history --cache --from 2026-01-01T00:00:00Z --to 2026-07-01T00:00:00Z --interval 336h"
Enter
Wait+Screen /2026-01-01 -> 2026-07-01 \| 14 samples/
Sleep 2s
