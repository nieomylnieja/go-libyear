Output docs/assets/history.gif

Require go

Set Shell "bash"
Set Theme "nord"
Set FontSize 18
Set Width 1280
Set Height 510
Set Padding 20
Set Framerate 10
Set TypingSpeed 20ms
Set PlaybackSpeed 2

Hide
Type "alias go-libyear='go run ./cmd/go-libyear'"
Enter
Show

Type "go-libyear history --cache --from 2026-01-01T00:00:00Z --to 2026-07-01T00:00:00Z --interval 336h"
Enter
Sleep 8s
