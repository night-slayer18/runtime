package model

// View identifies which presentation the UI is currently showing.
type View int

const (
	// ViewLog shows the (optionally filtered) stream of log lines.
	ViewLog View = iota
	// ViewGroups shows similar log lines collapsed into templated groups.
	ViewGroups
)

// State holds all application state for runtime-pulse: the streaming log
// reader, the active pattern filter, and the similar-error grouping. It is the
// PulseModel referenced by the design.
type State struct {
	reader *LogReader
	filter *Filter

	// visible is the filtered view of the reader's buffered entries, recomputed
	// whenever the buffer or filter changes.
	visible []LogEntry
	// groups is the similar-error grouping of the filtered view.
	groups []LogGroup

	view   View
	cursor int
	err    error
}

// New returns an initialised State with an empty filter and no log source.
func New() *State {
	return &State{
		filter: NewFilter(),
		view:   ViewLog,
	}
}

// Open attaches a log file to the model and performs an initial poll so the
// already-written portion of the file is available immediately.
func (s *State) Open(path string) error {
	r, err := NewLogReader(path)
	if err != nil {
		s.err = err
		return err
	}
	if s.reader != nil {
		_ = s.reader.Close()
	}
	s.reader = r
	s.err = nil
	return s.Refresh()
}

// SetReader attaches an already-constructed LogReader (useful for tests and
// non-file sources) and refreshes the derived views.
func (s *State) SetReader(r *LogReader) error {
	if s.reader != nil && s.reader != r {
		_ = s.reader.Close()
	}
	s.reader = r
	return s.Refresh()
}

// Refresh polls the reader for newly appended lines and recomputes the filtered
// view and grouping. It is safe to call repeatedly to drive live tailing.
func (s *State) Refresh() error {
	if s.reader == nil {
		s.visible = nil
		s.groups = nil
		return nil
	}
	if _, err := s.reader.Poll(); err != nil {
		s.err = err
		return err
	}
	s.recompute()
	return nil
}

// recompute rebuilds the filtered view and grouping from the reader's buffered
// entries. Only the bounded in-memory window is touched, never the whole file.
func (s *State) recompute() {
	entries := s.reader.Entries()
	s.visible = s.filter.Apply(entries)
	s.groups = GroupSimilar(s.visible)
	s.clampCursor()
}

// SetFilter updates the active pattern filter and recomputes the view.
func (s *State) SetFilter(query string) {
	s.filter.SetQuery(query)
	if s.reader != nil {
		s.recompute()
	}
}

// FilterQuery returns the active filter pattern.
func (s *State) FilterQuery() string { return s.filter.Query() }

// Visible returns the current filtered log entries.
func (s *State) Visible() []LogEntry { return s.visible }

// Groups returns the current similar-error groups.
func (s *State) Groups() []LogGroup { return s.groups }

// View returns the active presentation mode.
func (s *State) View() View { return s.view }

// ToggleView switches between the log and groups presentations.
func (s *State) ToggleView() {
	if s.view == ViewLog {
		s.view = ViewGroups
	} else {
		s.view = ViewLog
	}
	s.cursor = 0
	s.clampCursor()
}

// Cursor returns the current selection index within the active view.
func (s *State) Cursor() int { return s.cursor }

// MoveCursor shifts the selection by delta, clamped to the active view bounds.
func (s *State) MoveCursor(delta int) {
	s.cursor += delta
	s.clampCursor()
}

// Top moves the selection to the first row.
func (s *State) Top() { s.cursor = 0; s.clampCursor() }

// Bottom moves the selection to the last row.
func (s *State) Bottom() { s.cursor = s.rowCount() - 1; s.clampCursor() }

// rowCount returns the number of rows in the active view.
func (s *State) rowCount() int {
	if s.view == ViewGroups {
		return len(s.groups)
	}
	return len(s.visible)
}

// clampCursor keeps the cursor within the active view bounds.
func (s *State) clampCursor() {
	n := s.rowCount()
	if n == 0 {
		s.cursor = 0
		return
	}
	if s.cursor < 0 {
		s.cursor = 0
	}
	if s.cursor >= n {
		s.cursor = n - 1
	}
}

// Err returns the most recent error encountered while opening or polling.
func (s *State) Err() error { return s.err }

// Close releases the underlying log source.
func (s *State) Close() error {
	if s.reader == nil {
		return nil
	}
	return s.reader.Close()
}
