package main

import (
	"fmt"
	"io"
	"sort"
)

// DiffField is a single key/value pair as it appears on one side of a
// diff. It's deliberately narrower than Entry - Line and Comments have
// no meaning when comparing two files.
type DiffField struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// FieldChange is a key whose value differs between the two files.
type FieldChange struct {
	Key string `json:"key"`
	Old string `json:"old"`
	New string `json:"new"`
}

// SectionDiff holds the differences found within one section, keyed by
// section name. Added/Removed/Changed are all sorted by key so output is
// deterministic regardless of source order.
type SectionDiff struct {
	Name    string        `json:"section"`
	Added   []DiffField   `json:"added,omitempty"`
	Removed []DiffField   `json:"removed,omitempty"`
	Changed []FieldChange `json:"changed,omitempty"`
}

func (d SectionDiff) empty() bool {
	return len(d.Added) == 0 && len(d.Removed) == 0 && len(d.Changed) == 0
}

// DiffINI compares two INI files section by section and reports keys that
// were added, removed, or changed going from a to b. Sections present in
// only one file are reported as entirely added or removed. Section order
// in the result follows a's order first, then any sections in b that a
// doesn't have.
func DiffINI(a, b *INIFile) []SectionDiff {
	var order []string
	seen := map[string]bool{}
	for _, sec := range a.Sections {
		if !seen[sec.Name] {
			seen[sec.Name] = true
			order = append(order, sec.Name)
		}
	}
	for _, sec := range b.Sections {
		if !seen[sec.Name] {
			seen[sec.Name] = true
			order = append(order, sec.Name)
		}
	}

	var result []SectionDiff
	for _, name := range order {
		aEntries := entryMap(a, name)
		bEntries := entryMap(b, name)

		sd := SectionDiff{Name: name}
		for key, av := range aEntries {
			bv, ok := bEntries[key]
			if !ok {
				sd.Removed = append(sd.Removed, DiffField{Key: key, Value: av})
			} else if av != bv {
				sd.Changed = append(sd.Changed, FieldChange{Key: key, Old: av, New: bv})
			}
		}
		for key, bv := range bEntries {
			if _, ok := aEntries[key]; !ok {
				sd.Added = append(sd.Added, DiffField{Key: key, Value: bv})
			}
		}
		if sd.empty() {
			continue
		}
		sort.Slice(sd.Added, func(i, j int) bool { return sd.Added[i].Key < sd.Added[j].Key })
		sort.Slice(sd.Removed, func(i, j int) bool { return sd.Removed[i].Key < sd.Removed[j].Key })
		sort.Slice(sd.Changed, func(i, j int) bool { return sd.Changed[i].Key < sd.Changed[j].Key })
		result = append(result, sd)
	}
	return result
}

func entryMap(f *INIFile, section string) map[string]string {
	m := map[string]string{}
	for _, sec := range f.Sections {
		if sec.Name != section {
			continue
		}
		for _, e := range sec.Entries {
			m[e.Key] = e.Value
		}
	}
	return m
}

// WriteDiff renders diffs in a unified-diff-ish text form: '+' for keys
// only in b, '-' for keys only in a, '~' for keys whose value changed.
func WriteDiff(w io.Writer, diffs []SectionDiff) error {
	for i, sd := range diffs {
		if i > 0 {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
		name := sd.Name
		if name == "" {
			name = "(top level)"
		}
		if _, err := fmt.Fprintf(w, "[%s]\n", name); err != nil {
			return err
		}
		for _, e := range sd.Removed {
			if _, err := fmt.Fprintf(w, "  - %s: %s\n", e.Key, e.Value); err != nil {
				return err
			}
		}
		for _, c := range sd.Changed {
			if _, err := fmt.Fprintf(w, "  ~ %s: %s -> %s\n", c.Key, c.Old, c.New); err != nil {
				return err
			}
		}
		for _, e := range sd.Added {
			if _, err := fmt.Fprintf(w, "  + %s: %s\n", e.Key, e.Value); err != nil {
				return err
			}
		}
	}
	return nil
}
