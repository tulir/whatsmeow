package sqlstore

import (
	"database/sql"
	"errors"
	"testing"
)

// fakeRows is a minimal dbutil.Rows that yields rowsLeft rows and always fails to
// scan them, recording whether Close was called.
type fakeRows struct {
	rowsLeft int
	closed   bool
}

func (r *fakeRows) Next() bool {
	if r.rowsLeft > 0 {
		r.rowsLeft--
		return true
	}
	return false
}
func (r *fakeRows) Scan(...any) error                       { return errors.New("scan failed") }
func (r *fakeRows) Close() error                            { r.closed = true; return nil }
func (r *fakeRows) Err() error                              { return nil }
func (r *fakeRows) Columns() ([]string, error)              { return nil, nil }
func (r *fakeRows) ColumnTypes() ([]*sql.ColumnType, error) { return nil, nil }
func (r *fakeRows) NextResultSet() bool                     { return false }

// TestScanManyLidsClosesRowsOnScanError ensures the rows (and the underlying pooled
// DB connection) are closed even when scanning a row fails partway through iteration.
// Without the fix, the early return skips Close and leaks the connection.
func TestScanManyLidsClosesRowsOnScanError(t *testing.T) {
	rows := &fakeRows{rowsLeft: 1}
	s := NewCachedLIDMap(nil)
	if err := s.scanManyLids(rows, nil); err == nil {
		t.Fatal("expected an error from the failing scan")
	}
	if !rows.closed {
		t.Error("scanManyLids leaked the rows: Close() was not called on the scan-error path")
	}
}
