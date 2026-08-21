package main

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"
)

// Entry is a single key/value pair inside a section.
type Entry struct {
	Key   string
	Value string
	Line  int

	// Comments holds the comment lines (with their ';' or '#' marker
	// intact) that appeared directly above this entry in the source,
	// in file order, so WriteINI can put them back.
	Comments []string
}

// Section is a named block of entries. The implicit section at the top
// of a file, before any [header], has an empty Name.
type Section struct {
	Name    string
	Line    int
	Entries []Entry

	// Comments holds comment lines that appeared directly above the
	// [section] header (or, for the implicit section, at the top of
	// the file before anything else).
	Comments []string

	// TrailingComments holds comment lines that appeared after this
	// section's last entry with nothing following them - i.e. a
	// comment block at the very end of the file.
	TrailingComments []string
}

// INIFile is the parsed form of an .ini document, sections in file order.
type INIFile struct {
	Sections []Section
}

// ParseError reports a line that could not be interpreted as a section
// header, a comment, or a key/value pair.
type ParseError struct {
	Line int
	Text string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("line %d: cannot parse %q", e.Line, e.Text)
}

// ParseINI reads an INI document. Comment lines (starting with ';' or '#')
// are attached to whatever follows them - the next entry, or the next
// [section] header - so WriteINI can reproduce them on round trip. Blank
// lines are still dropped rather than preserved.
func ParseINI(r io.Reader) (*INIFile, error) {
	file := &INIFile{}
	// entries before the first [section] header live in the implicit
	// unnamed section, so we always start with one open.
	current := Section{Name: "", Line: 0}
	haveCurrent := false
	var pendingComments []string

	scanner := bufio.NewScanner(r)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		raw := scanner.Text()
		line := strings.TrimSpace(raw)

		if line == "" {
			continue
		}
		if strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			pendingComments = append(pendingComments, line)
			continue
		}

		if strings.HasPrefix(line, "[") {
			if !strings.HasSuffix(line, "]") {
				return nil, &ParseError{Line: lineNo, Text: raw}
			}
			if haveCurrent {
				file.Sections = append(file.Sections, current)
			}
			name := strings.TrimSpace(line[1 : len(line)-1])
			current = Section{Name: name, Line: lineNo, Comments: pendingComments}
			pendingComments = nil
			haveCurrent = true
			continue
		}

		sep := strings.IndexAny(line, "=:")
		if sep < 0 {
			return nil, &ParseError{Line: lineNo, Text: raw}
		}
		key := strings.TrimSpace(line[:sep])
		if key == "" {
			return nil, &ParseError{Line: lineNo, Text: raw}
		}
		value := strings.TrimSpace(line[sep+1:])

		if !haveCurrent {
			haveCurrent = true
		}
		current.Entries = append(current.Entries, Entry{Key: key, Value: value, Line: lineNo, Comments: pendingComments})
		pendingComments = nil
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(pendingComments) > 0 {
		current.TrailingComments = pendingComments
		haveCurrent = true
	}
	if haveCurrent {
		file.Sections = append(file.Sections, current)
	}
	return file, nil
}

// WriteINI serializes f in a stable, sorted-key form. Section order from
// the source is preserved; keys within a section are sorted so output is
// deterministic regardless of how the entries were built.
func WriteINI(w io.Writer, f *INIFile) error {
	bw := bufio.NewWriter(w)
	for i, sec := range f.Sections {
		if sec.Name != "" {
			if i > 0 {
				if _, err := fmt.Fprintln(bw); err != nil {
					return err
				}
			}
			if err := writeComments(bw, sec.Comments); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(bw, "[%s]\n", sec.Name); err != nil {
				return err
			}
		} else if err := writeComments(bw, sec.Comments); err != nil {
			return err
		}
		entries := append([]Entry(nil), sec.Entries...)
		sort.Slice(entries, func(a, b int) bool { return entries[a].Key < entries[b].Key })
		for _, e := range entries {
			if err := writeComments(bw, e.Comments); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(bw, "%s = %s\n", e.Key, e.Value); err != nil {
				return err
			}
		}
		if err := writeComments(bw, sec.TrailingComments); err != nil {
			return err
		}
	}
	return bw.Flush()
}

func writeComments(w io.Writer, comments []string) error {
	for _, c := range comments {
		if _, err := fmt.Fprintln(w, c); err != nil {
			return err
		}
	}
	return nil
}
