# Rehearse

Scene-partner cue player: it speaks your partner's lines, you answer.
Live at https://ayatsuro.github.io/rehearse/ (install it from Chrome's menu on Android).

## Making the partner's voice on the Mac

    cd voice
    go run . ~/path/to/scene.txt              # American voice, 1.2x speed
    go run . -speed 1.0 ~/path/to/scene.txt   # the app's speed slider goes faster without losing quality
    go run . -voice "Ava (Premium)" ~/path/to/scene.txt
    say -v '?' | grep en_US                   # American voices on this Mac

This writes `scene.rehearse.json` next to the scene: the script plus every line spoken.
Send it to the phone (Drive, email, cable) and tap **Import scene** in Rehearse.
Importing a file with the same scene name replaces that scene and keeps your role and start mark.

The scene format: one speech per line, `NAME: text`. (Stage directions) and [notes] are not spoken.
