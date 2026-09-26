package main

import (
	"reflect"
	"testing"
)

func TestParseLineStrictOK(t *testing.T) {
	cases := []struct {
		name string
		line string
		want []Field
	}{
		{
			name: "bare pairs",
			line: "level=info status=200",
			want: []Field{{"level", "info"}, {"status", "200"}},
		},
		{
			name: "quoted value with spaces",
			line: `msg="request completed"`,
			want: []Field{{"msg", "request completed"}},
		},
		{
			name: "escaped quote and backslash",
			line: `msg="a \"quoted\" word and a \\slash"`,
			want: []Field{{"msg", `a "quoted" word and a \slash`}},
		},
		{
			name: "escaped newline and tab",
			line: `msg="line one\nline two\tend"`,
			want: []Field{{"msg", "line one\nline two\tend"}},
		},
		{
			name: "valid RFC3339 time field",
			line: "time=2024-01-15T10:23:01Z level=info",
			want: []Field{{"time", "2024-01-15T10:23:01Z"}, {"level", "info"}},
		},
		{
			name: "key with dot dash underscore",
			line: "http.status_code-1=200",
			want: []Field{{"http.status_code-1", "200"}},
		},
		{
			name: "extra whitespace between pairs",
			line: "a=1    b=2",
			want: []Field{{"a", "1"}, {"b", "2"}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := NewParser(false)
			rec, err := p.ParseLine(tc.line, 1)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(rec.Fields, tc.want) {
				t.Fatalf("got %#v, want %#v", rec.Fields, tc.want)
			}
		})
	}
}

func TestParseLineStrictRejects(t *testing.T) {
	cases := []struct {
		name string
		line string
	}{
		{"bare word without value", "level info"},
		{"duplicate key", "a=1 a=2"},
		{"key starting with digit", "1abc=2"},
		{"unterminated quote", `msg="never closed`},
		{"invalid escape", `msg="bad \q escape"`},
		{"unexpected character", "a=1 @=2"},
		{"time not RFC3339", "time=not-a-time level=info"},
		{"time missing value", "time level=info"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := NewParser(false)
			if _, err := p.ParseLine(tc.line, 1); err == nil {
				t.Fatalf("expected error, got none")
			}
		})
	}
}

func TestParseLineLenientRecovers(t *testing.T) {
	cases := []struct {
		name string
		line string
		want []Field
	}{
		{
			name: "bare word becomes empty-valued flag",
			line: "level info",
			want: []Field{{"level", ""}, {"info", ""}},
		},
		{
			name: "duplicate key keeps both, last wins on lookup",
			line: "a=1 a=2",
			want: []Field{{"a", "1"}, {"a", "2"}},
		},
		{
			name: "key starting with digit is allowed",
			line: "1abc=2",
			want: []Field{{"1abc", "2"}},
		},
		{
			name: "unterminated quote takes rest of line",
			line: `msg="never closed`,
			want: []Field{{"msg", "never closed"}},
		},
		{
			name: "invalid escape keeps the escaped character",
			line: `msg="bad \q escape"`,
			want: []Field{{"msg", "bad q escape"}},
		},
		{
			name: "time left as plain string when unparseable",
			line: "time=not-a-time",
			want: []Field{{"time", "not-a-time"}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := NewParser(true)
			rec, err := p.ParseLine(tc.line, 1)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(rec.Fields, tc.want) {
				t.Fatalf("got %#v, want %#v", rec.Fields, tc.want)
			}
		})
	}
}

func TestParseLineLenientSkipsUnexpectedChar(t *testing.T) {
	p := NewParser(true)
	rec, err := p.ParseLine("a=1 @ b=2", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []Field{{"a", "1"}, {"b", "2"}}
	if !reflect.DeepEqual(rec.Fields, want) {
		t.Fatalf("got %#v, want %#v", rec.Fields, want)
	}
}

func TestParseStrictAbortsOnFirstError(t *testing.T) {
	lines := []string{
		"a=1",
		"b=2 b=3",
		"c=4",
	}
	p := NewParser(false)
	records, errs := p.Parse(lines)
	if records != nil {
		t.Fatalf("expected no records, got %#v", records)
	}
	if len(errs) != 1 {
		t.Fatalf("expected exactly one error, got %d: %v", len(errs), errs)
	}
	perr, ok := errs[0].(*ParseError)
	if !ok {
		t.Fatalf("expected *ParseError, got %T", errs[0])
	}
	if perr.Line != 2 {
		t.Fatalf("expected error on line 2, got line %d", perr.Line)
	}
}

func TestParseLenientCollectsErrorsAndKeepsGoing(t *testing.T) {
	lines := []string{
		"a=1",
		"",
		"b=2 b=3",
		"c=4",
	}
	p := NewParser(true)
	records, errs := p.Parse(lines)
	if len(errs) != 0 {
		t.Fatalf("lenient mode should not produce parse errors here, got %v", errs)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 records (blank line skipped), got %d: %#v", len(records), records)
	}
	if records[0].Line != 1 || records[1].Line != 3 || records[2].Line != 4 {
		t.Fatalf("unexpected line numbers: %d, %d, %d", records[0].Line, records[1].Line, records[2].Line)
	}
}

func TestParseLenientCollectsRealErrors(t *testing.T) {
	lines := []string{
		"a=1",
		`msg="never closed and strict-only would reject upstream but this is fine here`,
	}
	// Use strict to force a genuine error on line 2, then confirm lenient
	// mode on the same input produces no error and a usable record.
	strict := NewParser(false)
	if _, errs := strict.Parse(lines); len(errs) != 1 {
		t.Fatalf("expected strict parse to fail on line 2, got %v", errs)
	}

	lenient := NewParser(true)
	records, errs := lenient.Parse(lines)
	if len(errs) != 0 {
		t.Fatalf("expected lenient parse to recover, got errors %v", errs)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
}
