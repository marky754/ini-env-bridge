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

## Known limitations

- Comments in the source INI file are dropped, not round-tripped.
- `env` to `ini` only recovers section names that are a single word:
  splitting on the first underscore can't tell a section named
  `my section` (flattened to a `MY_SECTION_` prefix) apart from a
  section named `my` with a key called `section_...`.
- No support yet for INI value types beyond plain strings (no arrays,
  no interpolation).

## Roadmap

- preserve comments through parse and write
- `--strict` mode that fails on duplicate keys instead of just
  reporting them
- support quoted values with embedded `=` in INI
- a `diff` subcommand to compare two INI files section by section

## License

MIT, see LICENSE.
