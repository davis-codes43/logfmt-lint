// Command logfmt-lint validates logfmt-style log lines and pretty-prints them.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
)

func main() {
	lenient := flag.Bool("lenient", false, "tolerate malformed lines instead of rejecting the whole file")
	checkOnly := flag.Bool("check", false, "validate only; print errors but no formatted output")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [--lenient] [--check] [file ...]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "reads stdin if no files are given\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	paths := flag.Args()
	if len(paths) == 0 {
		paths = []string{"-"}
	}

	exitCode := 0
	for _, path := range paths {
		if err := run(path, *lenient, *checkOnly); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", path, err)
			exitCode = 1
		}
	}
	os.Exit(exitCode)
}

func run(path string, lenient, checkOnly bool) error {
	f := os.Stdin
	if path != "-" {
		opened, err := os.Open(path)
		if err != nil {
			return err
		}
		defer opened.Close()
		f = opened
	}

	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	p := NewParser(lenient)
	records, errs := p.Parse(lines)
	for _, e := range errs {
		fmt.Fprintf(os.Stderr, "%s: %v\n", path, e)
	}
	if len(errs) > 0 && !lenient {
		return fmt.Errorf("invalid input")
	}

	if checkOnly {
		return nil
	}
	return Print(os.Stdout, records)
}
