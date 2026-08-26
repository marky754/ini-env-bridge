# ini-env-bridge

I keep ending up with config in two shapes on the same project: an `.ini`
file for a tool that only speaks INI, and a `.env` file for whatever
reads environment variables at deploy time. Keeping both in sync by hand
is how you get a staging box with the old database password. This is a
small command-line converter between the two, plus a way to inspect an
INI file's structure before you trust it.

## Install

Requires Go 1.22+. No third-party dependencies.

```
go build -o iniconv .
```

## Usage

Convert an INI file to `.env` format:

```
$ cat config.ini
[database]
host = localhost
port = 5432

[server]
debug = true

$ ./iniconv convert --from ini --to env config.ini
DATABASE_HOST=localhost
DATABASE_PORT=5432
SERVER_DEBUG=true
```

Section names become the variable prefix, upper-cased. Keys outside any
section keep their own name.

Convert the other direction:

```
$ ./iniconv convert --from env --to ini secrets.env > secrets.ini
```

`env` to `ini` splits each name on its first underscore: the part
before becomes the section, the part after becomes the key. So
`DATABASE_HOST` becomes `[database] host`. A name with no underscore
lands in the top-level, unnamed section.

Both directions read from stdin if you don't pass a file:

```
$ curl -s https://example.com/config.ini | ./iniconv convert --from ini --to env
```

Inspect a file's structure — section count, key count, duplicate keys —
without converting it:

```
$ ./iniconv inspect config.ini
2 section(s), 3 key(s) total
  [database] line 1: 2 key(s)
  [server] line 4: 1 key(s)
```

Add `--json` for output a script can parse instead of a person reading
a terminal:

```
$ ./iniconv inspect --json config.ini
{
  "sections": 2,
  "total_entries": 3,
  "per_section": [
    { "name": "database", "line": 1, "key_count": 2 },
    { "name": "server", "line": 4, "key_count": 1 }
  ]
}
```

Add `--strict` to fail the command (non-zero exit) when duplicate keys
turn up, instead of only listing them:

```
$ ./iniconv inspect --strict config.ini
2 section(s), 3 key(s) total
  [database] line 1: 2 key(s)
  [server] line 4: 1 key(s)
duplicate keys:
  [database] host appears 2 times (lines [2 3])
iniconv: 1 duplicate key(s) found
```

Values can be wrapped in quotes when they need to carry an `=`, a `:`,
or leading/trailing whitespace through unchanged - `key = "  a=b  "`
keeps the spaces and the `=` instead of getting trimmed. Inside double
quotes a backslash escapes the next character, so a value can contain
a literal `"` too. Plain values don't need quoting just because they
contain `=` or `:` elsewhere, e.g. `url = http://host:8080/x?a=b`.

Compare two INI files section by section:

```
$ ./iniconv diff old.ini new.ini
[database]
  ~ host: localhost -> db.internal
  + timeout: 30
  - legacy_flag: true
```

`-` is a key only in the first file, `+` is a key only in the second,
`~` is a key present in both with a different value. Add `--json` for a
machine-readable list of the same changes, grouped by section.

## Known limitations

- Comments survive an INI-to-INI round trip (parsing and re-writing the
  same file), attached to whatever they were sitting above - a key, a
  `[section]` header, or the end of the file. They do not survive a
  detour through `.env`, since env files have no comment syntax the
  keys are converted back through.
- A value that's unquoted in the source but happens to start and end
  with matching quote characters is read as quoted, and loses those
  quote characters. Wrap it in an outer pair of quotes if you need the
  inner ones kept literally.
- `env` to `ini` only recovers section names that are a single word:
  splitting on the first underscore can't tell a section named
  `my section` (flattened to a `MY_SECTION_` prefix) apart from a
  section named `my` with a key called `section_...`.
- No support yet for INI value types beyond plain strings (no arrays,
  no interpolation).

## Roadmap

- unit tests for ini.go and env.go

## License

MIT, see LICENSE.
