// Command iniconv converts between .ini files and flat .env-style files,
// and can inspect an .ini file's structure before you convert it.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "convert":
		err = runConvert(os.Args[2:])
	case "inspect":
		err = runInspect(os.Args[2:])
	case "-h", "--help", "help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "iniconv: unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "iniconv: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `iniconv converts between .ini and .env files.

Usage:
  iniconv convert --from ini --to env [file]   convert ini to env, stdin if file omitted
  iniconv convert --from env --to ini [file]   convert env to ini, stdin if file omitted
  iniconv inspect [--json] [--strict] file      summarize an ini file's sections and keys

Convert writes to stdout. Inspect defaults to a human-readable report;
pass --json for a machine-readable one. Pass --strict to exit non-zero
when duplicate keys are found instead of just reporting them.
`)
}

func runConvert(args []string) error {
	fs := flag.NewFlagSet("convert", flag.ExitOnError)
	from := fs.String("from", "", "source format: ini or env")
	to := fs.String("to", "", "target format: ini or env")
	if err := fs.Parse(args); err != nil {
		return err
	}

	in, err := openInput(fs.Args())
	if err != nil {
		return err
	}
	defer in.Close()

	switch {
	case *from == "ini" && *to == "env":
		f, err := ParseINI(in)
		if err != nil {
			return err
		}
		return WriteEnv(os.Stdout, ToEnv(f))
	case *from == "env" && *to == "ini":
		vars, err := ParseEnv(in)
		if err != nil {
			return err
		}
		return WriteINI(os.Stdout, FromEnv(vars))
	case *from == "" || *to == "":
		return fmt.Errorf("--from and --to are required (ini or env)")
	default:
		return fmt.Errorf("unsupported conversion: %s to %s", *from, *to)
	}
}

func openInput(args []string) (*os.File, error) {
	if len(args) == 0 {
		return os.Stdin, nil
	}
	return os.Open(args[0])
}

// Report is the inspect output shape, shared by the human-readable and
// --json renderers so the two modes never drift apart in content.
type Report struct {
	Sections     int               `json:"sections"`
	TotalEntries int               `json:"total_entries"`
	PerSection   []SectionSummary  `json:"per_section"`
	Duplicates   []DuplicateEntry  `json:"duplicates,omitempty"`
}

type SectionSummary struct {
	Name  string `json:"name"`
	Line  int    `json:"line"`
	Count int    `json:"key_count"`
}

type DuplicateEntry struct {
	Section string `json:"section"`
	Key     string `json:"key"`
	Lines   []int  `json:"lines"`
}

func runInspect(args []string) error {
	fs := flag.NewFlagSet("inspect", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "emit machine-readable JSON instead of a text report")
	strict := fs.Bool("strict", false, "exit with a non-zero status if any duplicate keys are found")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("inspect takes exactly one file argument")
	}

	in, err := os.Open(fs.Arg(0))
	if err != nil {
		return err
	}
	defer in.Close()

	f, err := ParseINI(in)
	if err != nil {
		return err
	}

	report := buildReport(f)
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			return err
		}
	} else {
		printReport(report)
	}

	// The report itself already lists which keys collided, so the
	// strict failure just needs to flip the exit status - repeating
	// the detail here would only duplicate what's already on stdout.
	if *strict && len(report.Duplicates) > 0 {
		return fmt.Errorf("%d duplicate key(s) found", len(report.Duplicates))
	}
	return nil
}

func buildReport(f *INIFile) Report {
	report := Report{Sections: len(f.Sections)}

	for _, sec := range f.Sections {
		report.PerSection = append(report.PerSection, SectionSummary{
			Name:  sec.Name,
			Line:  sec.Line,
			Count: len(sec.Entries),
		})
		report.TotalEntries += len(sec.Entries)

		seen := map[string][]int{}
		for _, e := range sec.Entries {
			seen[e.Key] = append(seen[e.Key], e.Line)
		}
		for key, lines := range seen {
			if len(lines) > 1 {
				report.Duplicates = append(report.Duplicates, DuplicateEntry{
					Section: sec.Name,
					Key:     key,
					Lines:   lines,
				})
			}
		}
	}
	return report
}

func printReport(r Report) {
	fmt.Printf("%d section(s), %d key(s) total\n", r.Sections, r.TotalEntries)
	for _, s := range r.PerSection {
		name := s.Name
		if name == "" {
			name = "(top level)"
		}
		fmt.Printf("  [%s] line %d: %d key(s)\n", name, s.Line, s.Count)
	}
	if len(r.Duplicates) == 0 {
		return
	}
	fmt.Println("duplicate keys:")
	for _, d := range r.Duplicates {
		name := d.Section
		if name == "" {
			name = "(top level)"
		}
		fmt.Printf("  [%s] %s appears %d times (lines %v)\n", name, d.Key, len(d.Lines), d.Lines)
	}
}
