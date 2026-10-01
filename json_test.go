package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestPrintJSONKeepsSourceOrder(t *testing.T) {
	p := NewParser(false)
	recs, errs := p.Parse([]string{`b=2 a="x y" c=<&>`})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	var out bytes.Buffer
	if err := PrintJSON(&out, recs); err != nil {
		t.Fatal(err)
	}
	want := `{"b":"2","a":"x y","c":"<&>"}` + "\n"
	if out.String() != want {
		t.Errorf("got %q, want %q", out.String(), want)
	}
}

func TestPrintJSONLenientDuplicateKeepsLast(t *testing.T) {
	p := NewParser(true)
	recs, _ := p.Parse([]string{`a=1 b=2 a=3 flag`})
	var out bytes.Buffer
	if err := PrintJSON(&out, recs); err != nil {
		t.Fatal(err)
	}
	want := `{"a":"3","b":"2","flag":""}` + "\n"
	if out.String() != want {
		t.Errorf("got %q, want %q", out.String(), want)
	}
}

func TestPrintJSONOneValidObjectPerLine(t *testing.T) {
	p := NewParser(false)
	recs, errs := p.Parse([]string{
		`level=info msg="say \"hi\"\tnow"`,
		``,
		`level=warn msg=ok`,
	})
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	var out bytes.Buffer
	if err := PrintJSON(&out, recs); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2: %q", len(lines), out.String())
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(lines[0]), &m); err != nil {
		t.Fatalf("line 1 is not valid JSON: %v", err)
	}
	if m["msg"] != "say \"hi\"\tnow" {
		t.Errorf("msg round-trip failed: %q", m["msg"])
	}
}

func TestPrintJSONEmptyInput(t *testing.T) {
	var out bytes.Buffer
	if err := PrintJSON(&out, nil); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no output, got %q", out.String())
	}
}
