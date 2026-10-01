package main

import (
	"bytes"
	"encoding/json"
	"io"
)

// PrintJSON writes one JSON object per record (newline-delimited), so
// the output can be piped straight into jq or another line-oriented
// tool. Keys keep the order they had in the source line; encoding/json
// would sort a map, which loses that. A repeated key (lenient mode
// only) is emitted once, at its first position, with the last value,
// matching what the text printer shows.
func PrintJSON(w io.Writer, records []Record) error {
	var buf bytes.Buffer
	for _, rec := range records {
		buf.Reset()
		if err := encodeRecord(&buf, rec); err != nil {
			return err
		}
		buf.WriteByte('\n')
		if _, err := w.Write(buf.Bytes()); err != nil {
			return err
		}
	}
	return nil
}

func encodeRecord(buf *bytes.Buffer, rec Record) error {
	last := make(map[string]string, len(rec.Fields))
	for _, f := range rec.Fields {
		last[f.Key] = f.Value
	}

	buf.WriteByte('{')
	written := make(map[string]bool, len(rec.Fields))
	for _, f := range rec.Fields {
		if written[f.Key] {
			continue
		}
		written[f.Key] = true
		if len(written) > 1 {
			buf.WriteByte(',')
		}
		if err := encodeString(buf, f.Key); err != nil {
			return err
		}
		buf.WriteByte(':')
		if err := encodeString(buf, last[f.Key]); err != nil {
			return err
		}
	}
	buf.WriteByte('}')
	return nil
}

// encodeString avoids json.Marshal because it escapes <, > and & as
// < and friends, which makes log messages harder to read for no
// benefit here.
func encodeString(buf *bytes.Buffer, s string) error {
	var tmp bytes.Buffer
	enc := json.NewEncoder(&tmp)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return err
	}
	buf.Write(bytes.TrimRight(tmp.Bytes(), "\n"))
	return nil
}
