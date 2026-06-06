// Package datasource implements Runtime Grid's file importers.
//
// Grid must be able to import tabular data from CSV, TSV, XLSX, Parquet, and
// Arrow files (Requirement 7.1). Each supported format is described by a Format
// value and registered in a package-level registry. A Format reports whether it
// is Available — that is, whether this build can actually parse the format — and
// supplies a Reader that turns a file on disk into a datasource.DataSource that
// the rest of the application can navigate and export.
//
// # Fail-closed launch policy (Requirement 7.1)
//
// Requirement 7.1 states that if any required format fails to initialize, the
// application SHALL NOT launch until all formats are available. This package
// makes that policy explicit and testable through CheckAvailability, which
// returns a non-nil error enumerating every required format that is not
// available. The application entry point calls CheckAvailability before
// constructing the UI and refuses to launch when it returns an error. This is a
// fail-closed design: the default posture is "do not launch", and the app only
// proceeds once every required format reports readiness.
//
// # Which formats are actually implemented
//
// All five required formats are backed by real decoders:
//
//   - CSV and TSV are implemented with encoding/csv (stdlib). Always available.
//   - XLSX is implemented with archive/zip + encoding/xml (stdlib): an .xlsx
//     file is a ZIP of XML parts, so it can be read without any third-party
//     dependency. Always available.
//   - Parquet is decoded with github.com/parquet-go/parquet-go, a pure-Go
//     Parquet implementation (see parquet.go).
//   - Arrow IPC (Feather v2 / .arrow / .ipc) is decoded with
//     github.com/apache/arrow-go/v18/arrow/ipc (see arrow.go).
//
// All formats are registered with Available=true, so CheckAvailability passes
// and the fail-closed launch check (see CheckAvailability) lets Grid start with
// every required format genuinely working. ErrFormatUnavailable remains defined
// so a future format that fails to initialize can still be reported honestly,
// and so SetAvailability can simulate that condition in tests.
package datasource

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	ds "github.com/runtime-sh/runtime/packages/datasource"
)

// ErrFormatUnavailable is returned by a Reader for a format that is registered
// but cannot be decoded by this build. All five shipped formats (CSV, TSV,
// XLSX, Parquet, Arrow) have real readers, so this is currently only produced
// via SetAvailability in tests; it is retained so a future format that fails to
// initialize can still be reported honestly.
var ErrFormatUnavailable = errors.New("format support unavailable")

// ErrUnknownFormat is returned by Open when a file's extension does not match
// any registered format.
var ErrUnknownFormat = errors.New("unknown file format")

// Reader parses a file at path into an in-memory datasource.DataSource. The
// first row of the file is treated as the header and becomes the schema's
// column names.
type Reader func(path string) (ds.DataSource, error)

// Format describes a single importable file format.
type Format struct {
	// Name is the human-readable format name (e.g. "CSV").
	Name string
	// Extensions are the lower-case file extensions (without a dot) that map to
	// this format.
	Extensions []string
	// Available reports whether this build can actually decode the format. When
	// false, Read returns ErrFormatUnavailable and the fail-closed launch check
	// (CheckAvailability) treats the format as not ready.
	Available bool
	// Unavailable explains why the format is not available, for diagnostics and
	// the launch-refusal error message. It is empty when Available is true.
	Unavailable string
	// Read decodes a file into a DataSource. For unavailable formats it returns
	// ErrFormatUnavailable.
	Read Reader
}

// registry holds every required format keyed by Name. It is populated by init
// so the set of required formats is fixed and discoverable.
var registry = map[string]Format{}

// register adds or replaces a format in the registry.
func register(f Format) {
	registry[f.Name] = f
}

// SetAvailability overrides the Available flag (and clears the Unavailable
// reason when enabling) for the named format, returning a restore function that
// reinstates the previous Format. It exists so tests can exercise both branches
// of the fail-closed launch check (CheckAvailability) — simulating a fully
// available build as well as one where a required format fails to initialize —
// without shipping decoders that cannot be resolved offline. It returns a no-op
// restore when name is not registered.
func SetAvailability(name string, available bool) (restore func()) {
	prev, ok := registry[name]
	if !ok {
		return func() {}
	}
	next := prev
	next.Available = available
	if available {
		next.Unavailable = ""
	} else if next.Unavailable == "" {
		next.Unavailable = "unavailable (test override)"
	}
	registry[name] = next
	return func() { registry[name] = prev }
}

func init() {
	registerDelimited()
	registerXLSX()
	registerColumnar()
}

// Formats returns every registered format sorted by name. The slice is a copy,
// so callers may not mutate the registry through it.
func Formats() []Format {
	out := make([]Format, 0, len(registry))
	for _, f := range registry {
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// CheckAvailability enforces the fail-closed launch policy of Requirement 7.1.
// It returns nil only when every required format is available; otherwise it
// returns an error enumerating each unavailable format and the reason. The
// application entry point calls this before launching and refuses to start when
// it returns a non-nil error.
func CheckAvailability() error {
	var missing []string
	for _, f := range Formats() {
		if !f.Available {
			reason := f.Unavailable
			if reason == "" {
				reason = "not initialized"
			}
			missing = append(missing, fmt.Sprintf("%s (%s)", f.Name, reason))
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("required import formats unavailable: %s", strings.Join(missing, ", "))
}

// formatFor returns the format registered for the given file extension (without
// the leading dot, case-insensitive), reporting whether one was found.
func formatFor(ext string) (Format, bool) {
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))
	for _, f := range Formats() {
		for _, e := range f.Extensions {
			if e == ext {
				return f, true
			}
		}
	}
	return Format{}, false
}

// Open imports the file at path by dispatching on its extension to the matching
// registered format's Reader. It returns ErrUnknownFormat when the extension is
// not recognised and ErrFormatUnavailable when the format is registered but not
// decodable in this build.
func Open(path string) (ds.DataSource, error) {
	ext := filepath.Ext(path)
	f, ok := formatFor(ext)
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownFormat, ext)
	}
	if !f.Available || f.Read == nil {
		return nil, fmt.Errorf("%s: %w", f.Name, ErrFormatUnavailable)
	}
	return f.Read(path)
}

// buildSource converts a header row and data rows into an in-memory DataSource.
// The header names the columns; each data row's cells are stored as strings.
// Short rows are padded and long rows truncated to the header width so the
// schema and rows stay consistent.
func buildSource(header []string, rows [][]string) ds.DataSource {
	columns := make([]ds.Column, len(header))
	for i, name := range header {
		columns[i] = ds.Column{Name: name, Type: "text", Nullable: true}
	}
	out := make([][]interface{}, len(rows))
	for i, r := range rows {
		cells := make([]interface{}, len(header))
		for c := range header {
			if c < len(r) {
				cells[c] = r[c]
			} else {
				cells[c] = ""
			}
		}
		out[i] = cells
	}
	return ds.NewMemorySource(columns, out)
}
