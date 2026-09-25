# logfmt-lint

A validating parser and pretty-printer for logfmt-style log lines
(`key=value key2="quoted value" key3=123`), the format used by Heroku's
router logs and a lot of Go services that log with `log/slog` or
similar libraries.

## Why

logfmt is simple enough that every service ends up with a slightly
different idea of what's legal: some emit bare words instead of
`flag=true`, some forget to quote values with spaces in them, some
reuse a key twice on the same line. Most of the time that's harmless
until it isn't - a malformed line silently drops a field a dashboard
was relying on, or a timestamp that isn't actually RFC3339 breaks
whatever's parsing it downstream.

`logfmt-lint` parses each line against a strict grammar by default and
fails loudly on the first line that doesn't fit. If you're dealing
with older or messier log sources and just want to see what's in them,
pass `--lenient` and it'll extract what it can instead of giving up.

## What "strict" means

By default a line is rejected if:

- any token isn't a well-formed `key=value` or `key="quoted value"` pair
- a key contains anything other than letters, digits, `.`, `-`, or `_`,
  or doesn't start with a letter or underscore
- the same key appears twice
- a quoted value is missing its closing quote, or uses an escape
  sequence other than `\"`, `\\`, `\n`, `\t`
- a `time` field is present but isn't a valid RFC3339 timestamp

`--lenient` turns every one of those into "do the best you can" instead
of a hard failure: bare words become empty-valued flags, duplicate
keys keep the last value seen, unterminated quotes take the rest of
the line, and unparseable timestamps are left as plain strings.

## Usage

Build it:

```
go build -o logfmt-lint .
```

Pretty-print a file:

```
$ cat app.log
time=2024-01-15T10:23:01Z level=info msg="request completed" status=200 duration=45ms
$ ./logfmt-lint app.log
time=2024-01-15T10:23:01Z  level=info  msg="request completed"  duration=45ms  status=200
```

Validate without printing anything but errors:

```
./logfmt-lint --check access.log
```

Read from stdin, and tolerate a messier legacy source:

```
tail -f app.log | ./logfmt-lint --lenient
```

By default a single malformed line aborts the whole file with a
line-numbered error on stderr and a non-zero exit code:

```
$ ./logfmt-lint bad.log
bad.log: line 3: duplicate key "status"
```

## Status

Early skeleton: the parser, pretty-printer, and CLI flags described
above are implemented. No JSON output, no streaming mode, and no test
suite yet - see the roadmap in commit history for what's next.

## License

MIT, see LICENSE.
