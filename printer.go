package main

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// priorityKeys are printed first, in this order, when present. They
// cover the fields a human scans for first when reading logs.
var priorityKeys = []string{"time", "level", "msg"}

// Print writes one human-readable line per record: priority fields
// first, then everything else sorted by key so output is stable
// across runs regardless of the order fields appeared in the source.
func Print(w io.Writer, records []Record) error {
	for _, rec := range records {
		if err := printRecord(w, rec); err != nil {
			return err
		}
	}
	return nil
}

func printRecord(w io.Writer, rec Record) error {
	byKey := make(map[string]string, len(rec.Fields))
	for _, f := range rec.Fields {
		byKey[f.Key] = f.Value
	}

	var parts []string
	printed := make(map[string]bool, len(rec.Fields))
	for _, key := range priorityKeys {
		if v, ok := byKey[key]; ok {
			parts = append(parts, formatPair(key, v))
			printed[key] = true
		}
	}

	var rest []string
	for _, f := range rec.Fields {
		if !printed[f.Key] {
			rest = append(rest, f.Key)
			printed[f.Key] = true
		}
	}
	sort.Strings(rest)
	for _, key := range rest {
		parts = append(parts, formatPair(key, byKey[key]))
	}

	_, err := fmt.Fprintln(w, strings.Join(parts, "  "))
	return err
}

func formatPair(key, value string) string {
	if value == "" {
		return key
	}
	if needsQuoting(value) {
		return fmt.Sprintf("%s=%q", key, value)
	}
	return fmt.Sprintf("%s=%s", key, value)
}

func needsQuoting(v string) bool {
	return strings.ContainsAny(v, " \t\"=")
}
