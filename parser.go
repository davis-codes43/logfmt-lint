package main

import (
	"fmt"
	"strings"
	"time"
)

// Field is one key=value pair parsed from a logfmt line.
type Field struct {
	Key   string
	Value string
}

// Record is everything parsed from a single line.
type Record struct {
	Line   int
	Fields []Field
}

// ParseError describes why a line was rejected. The line number is
// 1-indexed to match what a text editor would show.
type ParseError struct {
	Line int
	Msg  string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("line %d: %s", e.Line, e.Msg)
}

func keyStartOK(r byte) bool {
	return r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func keyPartOK(r byte) bool {
	return keyStartOK(r) || (r >= '0' && r <= '9') || r == '.' || r == '-'
}

// Parser turns logfmt text into Records. Strict mode, the default,
// rejects a line the moment anything about it doesn't round-trip
// cleanly: bare words, duplicate keys, unterminated quotes, and (if
// a "time" field is present) timestamps that aren't RFC3339. Lenient
// mode does its best with the same input instead of throwing it all
// away, because plenty of real log sources are a little sloppy and
// sometimes you just need to see what's in the file.
type Parser struct {
	Lenient bool
}

func NewParser(lenient bool) *Parser {
	return &Parser{Lenient: lenient}
}

// ParseLine parses a single line. lineNo is only used to annotate errors.
func (p *Parser) ParseLine(line string, lineNo int) (Record, error) {
	rec := Record{Line: lineNo}
	seen := make(map[string]bool)

	i, n := 0, len(line)
	for i < n {
		for i < n && line[i] == ' ' {
			i++
		}
		if i >= n {
			break
		}

		keyStart := i
		for i < n && keyPartOK(line[i]) {
			i++
		}
		key := line[keyStart:i]

		if key == "" {
			if !p.Lenient {
				return Record{}, &ParseError{lineNo, fmt.Sprintf("unexpected character %q at position %d", line[i], i+1)}
			}
			i++ // skip the offending byte and try to resync
			continue
		}
		if !keyStartOK(key[0]) && !p.Lenient {
			return Record{}, &ParseError{lineNo, fmt.Sprintf("key %q must start with a letter or underscore", key)}
		}

		var value string
		switch {
		case i < n && line[i] == '=':
			i++
			v, newPos, err := p.readValue(line, i, key, lineNo)
			if err != nil {
				return Record{}, err
			}
			value, i = v, newPos
		default:
			if !p.Lenient {
				return Record{}, &ParseError{lineNo, fmt.Sprintf("key %q has no value (bare words need --lenient)", key)}
			}
		}

		if seen[key] && !p.Lenient {
			return Record{}, &ParseError{lineNo, fmt.Sprintf("duplicate key %q", key)}
		}
		seen[key] = true
		rec.Fields = append(rec.Fields, Field{Key: key, Value: value})
	}

	if !p.Lenient {
		if v, ok := findField(rec.Fields, "time"); ok {
			if _, err := time.Parse(time.RFC3339, v); err != nil {
				return Record{}, &ParseError{lineNo, fmt.Sprintf("field \"time\" is not RFC3339: %v", err)}
			}
		}
	}

	return rec, nil
}

// readValue parses the value that follows a key's '='. It returns the
// decoded value and the index just past it.
func (p *Parser) readValue(line string, i int, key string, lineNo int) (string, int, error) {
	n := len(line)
	if i >= n || line[i] != '"' {
		start := i
		for i < n && line[i] != ' ' {
			i++
		}
		return line[start:i], i, nil
	}

	start := i
	i++
	var b strings.Builder
	closed := false
	for i < n {
		c := line[i]
		if c == '\\' && i+1 < n {
			switch line[i+1] {
			case '"', '\\':
				b.WriteByte(line[i+1])
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			default:
				if !p.Lenient {
					return "", 0, &ParseError{lineNo, fmt.Sprintf("invalid escape \"\\%c\" in value for key %q", line[i+1], key)}
				}
				b.WriteByte(line[i+1])
			}
			i += 2
			continue
		}
		if c == '"' {
			closed = true
			i++
			break
		}
		b.WriteByte(c)
		i++
	}
	if !closed && !p.Lenient {
		return "", 0, &ParseError{lineNo, fmt.Sprintf("unterminated quoted value for key %q starting at position %d", key, start+1)}
	}
	return b.String(), i, nil
}

func findField(fields []Field, key string) (string, bool) {
	for _, f := range fields {
		if f.Key == key {
			return f.Value, true
		}
	}
	return "", false
}

// Parse parses a whole file's worth of lines, one Record per non-blank
// line. In strict mode the first bad line aborts the parse entirely
// and is returned as the sole error: a log file that's partly
// malformed usually means something upstream is broken, and silently
// returning the good lines would hide that. In lenient mode bad lines
// are skipped and collected in the returned error slice instead.
func (p *Parser) Parse(lines []string) ([]Record, []error) {
	var records []Record
	var errs []error
	for idx, line := range lines {
		lineNo := idx + 1
		if strings.TrimSpace(line) == "" {
			continue
		}
		rec, err := p.ParseLine(line, lineNo)
		if err != nil {
			if !p.Lenient {
				return nil, []error{err}
			}
			errs = append(errs, err)
			continue
		}
		records = append(records, rec)
	}
	return records, errs
}
