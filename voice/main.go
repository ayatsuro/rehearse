// rehearse-voice turns a scene into a file Rehearse can import: the script plus
// every line spoken by a fast American voice, made on this Mac with `say`.
//
//	go run . scene.txt                 # writes scene.rehearse.json next to it (1.2x speed)
//	go run . -speed 1.0 scene.txt
//	go run . -voice "Ava (Premium)" scene.txt
//	say -v '?' | grep en_US           # the American voices on this Mac
//
// The scene uses the same format as the app: one speech per line, NAME: text.
// (Stage directions) and [notes] are not spoken.
package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

type Line struct{ Who, Text string }

var speechRe = regexp.MustCompile(`^([A-Z][A-Za-z .'\-]{0,30}):\s*(.*)$`)

// parse reads a scene exactly as Rehearse does.
func parse(t string) []Line {
	var out []Line
	for _, r := range strings.Split(strings.ReplaceAll(t, "\r\n", "\n"), "\n") {
		r = strings.TrimSpace(strings.ReplaceAll(r, "\ufeff", ""))
		if r == "" {
			continue
		}
		if m := speechRe.FindStringSubmatch(r); m != nil {
			out = append(out, Line{strings.ToUpper(strings.TrimSpace(m[1])), m[2]})
		} else if len(out) > 0 {
			out[len(out)-1].Text += " " + r
		}
	}
	return out
}

var unspoken = regexp.MustCompile(`\([^)]*\)|\[[^\]]*\]`)
var spaces = regexp.MustCompile(`\s+`)

func speakable(t string) string {
	return strings.TrimSpace(spaces.ReplaceAllString(unspoken.ReplaceAllString(t, ""), " "))
}

// key names a line's audio the same way the app looks it up, so lines keep
// their audio even when other parts of the scene are edited.
func key(l Line) string { return l.Who + "|" + strings.ToLower(speakable(l.Text)) }

type Scene struct {
	Format string            `json:"format"`
	Name   string            `json:"name"`
	Script string            `json:"script"`
	Voice  string            `json:"voice"`
	Speed  float64           `json:"speed"`
	Audio  map[string]string `json:"audio"`
}

func main() {
	voice := flag.String("voice", "Samantha", "American voice to use (list them with: say -v '?' | grep en_US)")
	speed := flag.Float64("speed", 1.2, "how much faster than normal speech; the app can speed it up further")
	out := flag.String("o", "", "output file (default: next to the scene, .rehearse.json)")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: go run . [-speed 1.4] [-voice Samantha] scene.txt")
		os.Exit(2)
	}
	src := flag.Arg(0)
	if err := run(src, *voice, *speed, *out); err != nil {
		fmt.Fprintln(os.Stderr, "rehearse-voice:", err)
		os.Exit(1)
	}
}

func run(src, voice string, speed float64, out string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	// Files exported from Google Docs or Word start with an invisible mark (BOM)
	// that would hide the first speaker's name.
	text := strings.TrimPrefix(string(b), "\ufeff")
	lines := parse(text)
	if len(lines) == 0 {
		return fmt.Errorf("%s has no speeches; write them as NAME: text", src)
	}
	if out == "" {
		out = strings.TrimSuffix(src, filepath.Ext(src)) + ".rehearse.json"
	}
	tmp, err := os.MkdirTemp("", "rehearse-voice")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	// Every speech gets audio, for both parts, so you can switch roles later.
	todo := map[string]string{}
	for _, l := range lines {
		if t := speakable(l.Text); t != "" {
			todo[key(l)] = t
		}
	}
	wpm := int(175*speed + 0.5) // `say` speaks about 175 words a minute by default
	scene := Scene{Format: "rehearse-scene", Name: strings.TrimSuffix(filepath.Base(src), filepath.Ext(src)),
		Script: text, Voice: voice, Speed: speed, Audio: map[string]string{}}

	type job struct{ k, text string }
	jobs := make(chan job)
	var mu sync.Mutex
	var firstErr error
	var wg sync.WaitGroup
	n := 0
	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for j := range jobs {
				data, err := synth(tmp, fmt.Sprintf("w%d", w), voice, wpm, j.text)
				mu.Lock()
				if err != nil && firstErr == nil {
					firstErr = err
				}
				if err == nil {
					scene.Audio[j.k] = "data:audio/mp4;base64," + base64.StdEncoding.EncodeToString(data)
					n++
					fmt.Printf("\r  %d of %d lines", n, len(todo))
				}
				mu.Unlock()
			}
		}(w)
	}
	for k, t := range todo {
		jobs <- job{k, t}
	}
	close(jobs)
	wg.Wait()
	fmt.Println()
	if firstErr != nil {
		return firstErr
	}
	js, err := json.Marshal(scene)
	if err != nil {
		return err
	}
	if err := os.WriteFile(out, js, 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s (%.1f MB, %d lines, voice %s at %.2gx)\n", out, float64(len(js))/1e6, len(todo), voice, speed)
	fmt.Println("Send it to your phone and use Import scene in Rehearse.")
	return nil
}

// synth speaks one line to a small AAC file: 32 kbps mono, plenty for a voice.
var sayBin, convertBin = envOr("REHEARSE_SAY", "say"), envOr("REHEARSE_AFCONVERT", "afconvert")

func synth(dir, name, voice string, wpm int, text string) ([]byte, error) {
	aiff := filepath.Join(dir, name+".aiff")
	m4a := filepath.Join(dir, name+".m4a")
	if out, err := exec.Command(sayBin, "-v", voice, "-r", fmt.Sprint(wpm), "-o", aiff, text).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("say failed (is the voice %q installed?): %v %s", voice, err, out)
	}
	if out, err := exec.Command(convertBin, "-f", "m4af", "-d", "aac", "-b", "32000", "-c", "1", aiff, m4a).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("afconvert failed: %v %s", err, out)
	}
	return os.ReadFile(m4a)
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
