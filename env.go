package main

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// EnvVar is one NAME=value line of a .env file.
type EnvVar struct {
	Name  string
	Value string
}

var envNameSanitizer = regexp.MustCompile(`[^A-Za-z0-9]+`)

// ToEnv flattens an INIFile into env-style vars. Keys in the implicit
// unnamed section keep their own name; keys in a [section] get the
// section name upper-cased and prefixed, e.g. [database] host becomes
// DATABASE_HOST. Collisions after sanitizing are left as-is: whoever
// wrote the source file is responsible for picking non-colliding names.
func ToEnv(f *INIFile) []EnvVar {
	var vars []EnvVar
	for _, sec := range f.Sections {
		prefix := ""
		if sec.Name != "" {
			prefix = envSafeName(sec.Name) + "_"
		}
		for _, e := range sec.Entries {
			vars = append(vars, EnvVar{
				Name:  prefix + envSafeName(e.Key),
				Value: e.Value,
			})
		}
	}
	return vars
}

func envSafeName(s string) string {
	s = envNameSanitizer.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	return strings.ToUpper(s)
}

// FromEnv builds an INIFile from env vars, splitting each name on its
// first underscore: the part before becomes the section, the part after
// becomes the key. A name with no underscore lands in the implicit
// top-level section. This is the exact reverse of ToEnv only for
// single-word section names; a section named "my section" flattens to
// the MY_SECTION_ prefix, and splitting on the first underscore alone
// can't tell that apart from a section named "my" with a key starting
// "section_...". See the README's known limitations.
func FromEnv(vars []EnvVar) *INIFile {
	order := []string{""}
	sections := map[string]*Section{"": {Name: ""}}

	for _, v := range vars {
		secName, key := splitEnvName(v.Name)
		sec, ok := sections[secName]
		if !ok {
			sec = &Section{Name: secName}
			sections[secName] = sec
			order = append(order, secName)
		}
		sec.Entries = append(sec.Entries, Entry{Key: key, Value: v.Value})
	}

	f := &INIFile{}
	for _, name := range order {
		sec := sections[name]
		if name == "" && len(sec.Entries) == 0 && len(order) > 1 {
			continue
		}
		f.Sections = append(f.Sections, *sec)
	}
	return f
}

// splitEnvName divides an env var name into a section and key on its
// first underscore, e.g. "DATABASE_HOST" -> ("database", "host"). A
// name with no underscore has no section.
func splitEnvName(name string) (section, key string) {
	i := strings.IndexByte(name, '_')
	if i < 0 {
		return "", strings.ToLower(name)
	}
	return strings.ToLower(name[:i]), strings.ToLower(name[i+1:])
}

// ParseEnv reads NAME=value lines, ignoring blank lines and '#' comments.
// A leading "export " on a line is stripped, matching common .env usage.
func ParseEnv(r io.Reader) ([]EnvVar, error) {
	var vars []EnvVar
	scanner := bufio.NewScanner(r)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		raw := scanner.Text()
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			return nil, &ParseError{Line: lineNo, Text: raw}
		}
		name := strings.TrimSpace(line[:eq])
		if name == "" {
			return nil, &ParseError{Line: lineNo, Text: raw}
		}
		value := strings.TrimSpace(line[eq+1:])
		value = unquoteEnvValue(value)
		vars = append(vars, EnvVar{Name: name, Value: value})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return vars, nil
}

func unquoteEnvValue(v string) string {
	if len(v) >= 2 && (v[0] == '"' && v[len(v)-1] == '"' || v[0] == '\'' && v[len(v)-1] == '\'') {
		return v[1 : len(v)-1]
	}
	return v
}

// WriteEnv serializes vars as NAME=value lines in the order given.
func WriteEnv(w io.Writer, vars []EnvVar) error {
	bw := bufio.NewWriter(w)
	for _, v := range vars {
		if _, err := fmt.Fprintf(bw, "%s=%s\n", v.Name, v.Value); err != nil {
			return err
		}
	}
	return bw.Flush()
}
