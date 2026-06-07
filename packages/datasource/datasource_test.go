package datasource

import (
	"errors"
	"testing"
)

func sampleColumns() []Column {
	return []Column{
		{Name: "id", Type: "int", IsPrimary: true},
		{Name: "name", Type: "text"},
	}
}

func sampleRows() [][]interface{} {
	return [][]interface{}{
		{1, "alice"},
		{2, "bob"},
		{3, "carol"},
	}
}

// TestIteratorNextScanErrOrdering verifies the standard cursor contract: Next
// advances row by row, Scan reads the current row, Next eventually returns
// false, and Err reports no error for a clean iteration.
func TestIteratorNextScanErrOrdering(t *testing.T) {
	ds := NewMemorySource(sampleColumns(), sampleRows())
	defer func() { _ = ds.Close() }()

	it, err := ds.Query("select *")
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	defer func() { _ = it.Close() }()

	var ids []int
	var names []string
	for it.Next() {
		var id int
		var name string
		if err := it.Scan(&id, &name); err != nil {
			t.Fatalf("Scan returned error: %v", err)
		}
		ids = append(ids, id)
		names = append(names, name)
	}

	if err := it.Err(); err != nil {
		t.Fatalf("Err returned error after clean iteration: %v", err)
	}

	wantIDs := []int{1, 2, 3}
	if len(ids) != len(wantIDs) {
		t.Fatalf("got %d rows, want %d", len(ids), len(wantIDs))
	}
	for i := range wantIDs {
		if ids[i] != wantIDs[i] {
			t.Errorf("row %d id = %d, want %d", i, ids[i], wantIDs[i])
		}
	}
	wantNames := []string{"alice", "bob", "carol"}
	for i := range wantNames {
		if names[i] != wantNames[i] {
			t.Errorf("row %d name = %q, want %q", i, names[i], wantNames[i])
		}
	}

	// Next must keep returning false once exhausted.
	if it.Next() {
		t.Error("Next returned true after iteration was exhausted")
	}
}

// TestIteratorScanBeforeNext verifies Scan fails when there is no current row
// (Next has not been called yet).
func TestIteratorScanBeforeNext(t *testing.T) {
	ds := NewMemorySource(sampleColumns(), sampleRows())
	defer func() { _ = ds.Close() }()

	it, err := ds.Query("select *")
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	defer func() { _ = it.Close() }()
	var id int
	var name string
	if err := it.Scan(&id, &name); err == nil {
		t.Fatal("Scan before Next returned nil error, want error")
	}
}

// TestIteratorEmptyResult verifies an empty result set iterates zero times,
// Next is immediately false, and Err is nil.
func TestIteratorEmptyResult(t *testing.T) {
	ds := NewMemorySource(sampleColumns(), nil)
	defer func() { _ = ds.Close() }()

	it, err := ds.Query("select *")
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	defer func() { _ = it.Close() }()

	count := 0
	for it.Next() {
		count++
	}
	if count != 0 {
		t.Errorf("iterated %d times over empty result, want 0", count)
	}
	if err := it.Err(); err != nil {
		t.Errorf("Err returned error for empty result: %v", err)
	}
}

// TestIteratorEmptyResultViaQueryFunc verifies an empty selection produced by a
// custom QueryFunc also iterates zero times.
func TestIteratorEmptyResultViaQueryFunc(t *testing.T) {
	ds := NewMemorySource(sampleColumns(), sampleRows()).
		WithQueryFunc(func(query string, rows [][]interface{}) ([][]interface{}, error) {
			return nil, nil
		})
	defer func() { _ = ds.Close() }()

	it, err := ds.Query("select * where false")
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	defer func() { _ = it.Close() }()

	if it.Next() {
		t.Error("Next returned true for empty filtered result")
	}
	if err := it.Err(); err != nil {
		t.Errorf("Err returned error: %v", err)
	}
}

// TestIteratorScanColumnCountMismatch verifies Scan returns ErrColumnCount when
// the number of destinations does not match the column count, and that the
// error is surfaced through Err afterwards.
func TestIteratorScanColumnCountMismatch(t *testing.T) {
	ds := NewMemorySource(sampleColumns(), sampleRows())
	defer func() { _ = ds.Close() }()

	it, err := ds.Query("select *")
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	defer func() { _ = it.Close() }()

	if !it.Next() {
		t.Fatal("Next returned false on first row")
	}

	var only int
	err = it.Scan(&only) // row has 2 columns, only 1 destination
	if err == nil {
		t.Fatal("Scan with wrong destination count returned nil error")
	}
	if !errors.Is(err, ErrColumnCount) {
		t.Errorf("Scan error = %v, want ErrColumnCount", err)
	}

	// Once a Scan error occurs, Err reports it and Next stops.
	if !errors.Is(it.Err(), ErrColumnCount) {
		t.Errorf("Err = %v, want ErrColumnCount", it.Err())
	}
	if it.Next() {
		t.Error("Next returned true after a Scan error")
	}
}

// TestIteratorCloseIdempotent verifies Close can be called repeatedly without
// error and that Scan after Close returns ErrClosed.
func TestIteratorCloseIdempotent(t *testing.T) {
	ds := NewMemorySource(sampleColumns(), sampleRows())
	defer func() { _ = ds.Close() }()

	it, err := ds.Query("select *")
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}

	if err := it.Close(); err != nil {
		t.Fatalf("first Close returned error: %v", err)
	}
	if err := it.Close(); err != nil {
		t.Fatalf("second Close returned error: %v", err)
	}

	// After Close, Next returns false and Scan returns ErrClosed.
	if it.Next() {
		t.Error("Next returned true after Close")
	}
	var id int
	var name string
	if err := it.Scan(&id, &name); !errors.Is(err, ErrClosed) {
		t.Errorf("Scan after Close = %v, want ErrClosed", err)
	}
}

// TestSourceCloseIdempotent verifies the MemorySource Close is idempotent.
func TestSourceCloseIdempotent(t *testing.T) {
	ds := NewMemorySource(sampleColumns(), sampleRows())
	if err := ds.Close(); err != nil {
		t.Fatalf("first Close returned error: %v", err)
	}
	if err := ds.Close(); err != nil {
		t.Fatalf("second Close returned error: %v", err)
	}
}

// TestSourceErrClosedAfterClose verifies all source operations return ErrClosed
// once the source is closed.
func TestSourceErrClosedAfterClose(t *testing.T) {
	ds := NewMemorySource(sampleColumns(), sampleRows())
	if err := ds.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	if _, err := ds.Schema(); !errors.Is(err, ErrClosed) {
		t.Errorf("Schema after Close = %v, want ErrClosed", err)
	}
	if _, err := ds.Query("select *"); !errors.Is(err, ErrClosed) {
		t.Errorf("Query after Close = %v, want ErrClosed", err)
	}
	if _, err := ds.Execute("delete"); !errors.Is(err, ErrClosed) {
		t.Errorf("Execute after Close = %v, want ErrClosed", err)
	}
}

// TestScanWithInterfaceDest verifies scanning into *interface{} stores values
// as-is, supporting generic consumers.
func TestScanWithInterfaceDest(t *testing.T) {
	ds := NewMemorySource(sampleColumns(), [][]interface{}{{42, "answer"}})
	defer func() { _ = ds.Close() }()

	it, err := ds.Query("select *")
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	defer func() { _ = it.Close() }()

	if !it.Next() {
		t.Fatal("Next returned false on first row")
	}
	var a, b interface{}
	if err := it.Scan(&a, &b); err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}
	if a != 42 || b != "answer" {
		t.Errorf("scanned (%v, %v), want (42, answer)", a, b)
	}
}
