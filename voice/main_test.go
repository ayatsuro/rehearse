package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAndKey(t *testing.T) {
	ls := parse("CALLIE: Hi, are you  Mrs. Winsley?\nMRS. WINSLEY: (warmly) I am.\nCome in. [beat]\n\nnot a speech\nCallie: Thank you.")
	if len(ls) != 3 || ls[1].Who != "MRS. WINSLEY" || ls[1].Text != "(warmly) I am. Come in. [beat] not a speech" || ls[2].Who != "CALLIE" {
		t.Fatalf("%+v", ls)
	}
	if k := key(ls[0]); k != "CALLIE|hi, are you mrs. winsley?" {
		t.Fatalf("key %q", k)
	}
	if k := key(ls[1]); k != "MRS. WINSLEY|i am. come in. not a speech" {
		t.Fatalf("key %q", k)
	}
}

func TestRunWithFakeMac(t *testing.T) {
	dir := t.TempDir()
	// stand-ins for macOS say and afconvert
	say := filepath.Join(dir, "say")
	os.WriteFile(say, []byte("#!/bin/sh\nwhile [ $# -gt 1 ]; do [ \"$1\" = -o ] && out=$2; shift; done\necho \"$1\" > \"$out\"\n"), 0o755)
	conv := filepath.Join(dir, "afconvert")
	os.WriteFile(conv, []byte("#!/bin/sh\nfor a; do last2=$prev; prev=$a; done\ncp \"$last2\" \"$prev\"\n"), 0o755)
	sayBin, convertBin = say, conv
	src := filepath.Join(dir, "winsley.txt")
	os.WriteFile(src, []byte("CALLIE: Hi.\nMRS. WINSLEY: (sitting) Come in.\nCALLIE: Hi.\n"), 0o644)
	if err := run(src, "Samantha", 1.4, ""); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "winsley.rehearse.json"))
	if err != nil {
		t.Fatal(err)
	}
	var s Scene
	json.Unmarshal(b, &s)
	if s.Format != "rehearse-scene" || s.Name != "winsley" || len(s.Audio) != 2 || !strings.HasPrefix(s.Audio["MRS. WINSLEY|come in."], "data:audio/mp4;base64,") {
		t.Fatalf("%+v", s)
	}
}
