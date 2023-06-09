package tsm1_test

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/golang/snappy"
	"github.com/influxdata/influxdb/pkg/slices"
	"github.com/influxdata/influxdb/tsdb/engine/tsm1"
)

func TestWALWriter_WriteMulti_Single(t *testing.T) {
	dir := MustTempDir()
	defer os.RemoveAll(dir)
	f := MustTempFile(dir)
	w := tsm1.NewWALSegmentWriter(f)

	p1 := tsm1.NewValue(1, 1.1)
	p2 := tsm1.NewValue(1, int64(1))
	p3 := tsm1.NewValue(1, true)
	p4 := tsm1.NewValue(1, "string")
	p5 := tsm1.NewValue(1, ^uint64(0))

	values := map[string][]tsm1.Value{
		"cpu,host=A#!~#float":    []tsm1.Value{p1},
		"cpu,host=A#!~#int":      []tsm1.Value{p2},
		"cpu,host=A#!~#bool":     []tsm1.Value{p3},
		"cpu,host=A#!~#string":   []tsm1.Value{p4},
		"cpu,host=A#!~#unsigned": []tsm1.Value{p5},
	}

	entry := &tsm1.WriteWALEntry{
		Values: values,
	}

	if err := w.Write(mustMarshalEntry(entry)); err != nil {
		fatal(t, "write points", err)
	}

	if err := w.Flush(); err != nil {
		fatal(t, "flush", err)
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		fatal(t, "seek", err)
	}

	r := tsm1.NewWALSegmentReader(f)

	if !r.Next() {
		t.Fatalf("expected next, got false")
	}

	we, err := r.Read()
	if err != nil {
		fatal(t, "read entry", err)
	}

	e, ok := we.(*tsm1.WriteWALEntry)
	if !ok {
		t.Fatalf("expected WriteWALEntry: got %#v", e)
	}

	for k, v := range e.Values {
		for i, vv := range v {
			if got, exp := vv.String(), values[k][i].String(); got != exp {
				t.Fatalf("points mismatch: got %v, exp %v", got, exp)
			}
		}
	}

	if n := r.Count(); n != MustReadFileSize(f) {
		t.Fatalf("wrong count of bytes read, got %d, exp %d", n, MustReadFileSize(f))
	}
}

func TestWALWriter_WriteMulti_LargeBatch(t *testing.T) {
	dir := MustTempDir()
	defer os.RemoveAll(dir)
	f := MustTempFile(dir)
	w := tsm1.NewWALSegmentWriter(f)

	var points []tsm1.Value
	for i := 0; i < 100000; i++ {
		points = append(points, tsm1.NewValue(int64(i), int64(1)))
	}

	values := map[string][]tsm1.Value{
		"cpu,host=A,server=01,foo=bar,tag=really-long#!~#float": points,
		"mem,host=A,server=01,foo=bar,tag=really-long#!~#float": points,
	}

	entry := &tsm1.WriteWALEntry{
		Values: values,
	}

	if err := w.Write(mustMarshalEntry(entry)); err != nil {
		fatal(t, "write points", err)
	}

	if err := w.Flush(); err != nil {
		fatal(t, "flush", err)
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		fatal(t, "seek", err)
	}

	r := tsm1.NewWALSegmentReader(f)

	if !r.Next() {
		t.Fatalf("expected next, got false")
	}

	we, err := r.Read()
	if err != nil {
		fatal(t, "read entry", err)
	}

	e, ok := we.(*tsm1.WriteWALEntry)
	if !ok {
		t.Fatalf("expected WriteWALEntry: got %#v", e)
	}

	for k, v := range e.Values {
		for i, vv := range v {
			if got, exp := vv.String(), values[k][i].String(); got != exp {
				t.Fatalf("points mismatch: got %v, exp %v", got, exp)
			}
		}
	}

	if n := r.Count(); n != MustReadFileSize(f) {
		t.Fatalf("wrong count of bytes read, got %d, exp %d", n, MustReadFileSize(f))
	}
}
func TestWALWriter_WriteMulti_Multiple(t *testing.T) {
	dir := MustTempDir()
	defer os.RemoveAll(dir)
	f := MustTempFile(dir)
	w := tsm1.NewWALSegmentWriter(f)

	p1 := tsm1.NewValue(1, int64(1))
	p2 := tsm1.NewValue(1, int64(2))

	exp := []struct {
		key    string
		values []tsm1.Value
	}{
		{"cpu,host=A#!~#value", []tsm1.Value{p1}},
		{"cpu,host=B#!~#value", []tsm1.Value{p2}},
	}

	for _, v := range exp {
		entry := &tsm1.WriteWALEntry{
			Values: map[string][]tsm1.Value{v.key: v.values},
		}

		if err := w.Write(mustMarshalEntry(entry)); err != nil {
			fatal(t, "write points", err)
		}
		if err := w.Flush(); err != nil {
			fatal(t, "flush", err)
		}
	}

	// Seek back to the beinning of the file for reading
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		fatal(t, "seek", err)
	}

	r := tsm1.NewWALSegmentReader(f)

	for _, ep := range exp {
		if !r.Next() {
			t.Fatalf("expected next, got false")
		}

		we, err := r.Read()
		if err != nil {
			fatal(t, "read entry", err)
		}

		e, ok := we.(*tsm1.WriteWALEntry)
		if !ok {
			t.Fatalf("expected WriteWALEntry: got %#v", e)
		}

		for k, v := range e.Values {
			if got, exp := k, ep.key; got != exp {
				t.Fatalf("key mismatch. got %v, exp %v", got, exp)
			}

			if got, exp := len(v), len(ep.values); got != exp {
				t.Fatalf("values length mismatch: got %v, exp %v", got, exp)
			}

			for i, vv := range v {
				if got, exp := vv.String(), ep.values[i].String(); got != exp {
					t.Fatalf("points mismatch: got %v, exp %v", got, exp)
				}
			}
		}
	}

	if n := r.Count(); n != MustReadFileSize(f) {
		t.Fatalf("wrong count of bytes read, got %d, exp %d", n, MustReadFileSize(f))
	}
}

func TestWALWriter_WriteDelete_Single(t *testing.T) {
	dir := MustTempDir()
	defer os.RemoveAll(dir)
	f := MustTempFile(dir)
	w := tsm1.NewWALSegmentWriter(f)

	entry := &tsm1.DeleteWALEntry{
		Keys: [][]byte{[]byte("cpu")},
	}

	if err := w.Write(mustMarshalEntry(entry)); err != nil {
		fatal(t, "write points", err)
	}

	if err := w.Flush(); err != nil {
		fatal(t, "flush", err)
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		fatal(t, "seek", err)
	}

	r := tsm1.NewWALSegmentReader(f)

	if !r.Next() {
		t.Fatalf("expected next, got false")
	}

	we, err := r.Read()
	if err != nil {
		fatal(t, "read entry", err)
	}

	e, ok := we.(*tsm1.DeleteWALEntry)
	if !ok {
		t.Fatalf("expected WriteWALEntry: got %#v", e)
	}

	if got, exp := len(e.Keys), len(entry.Keys); got != exp {
		t.Fatalf("key length mismatch: got %v, exp %v", got, exp)
	}

	if got, exp := string(e.Keys[0]), string(entry.Keys[0]); got != exp {
		t.Fatalf("key mismatch: got %v, exp %v", got, exp)
	}
}

func TestWALWriter_WriteMultiDelete_Multiple(t *testing.T) {
	dir := MustTempDir()
	defer os.RemoveAll(dir)
	f := MustTempFile(dir)
	w := tsm1.NewWALSegmentWriter(f)

	p1 := tsm1.NewValue(1, true)
	values := map[string][]tsm1.Value{
		"cpu,host=A#!~#value": []tsm1.Value{p1},
	}

	writeEntry := &tsm1.WriteWALEntry{
		Values: values,
	}

	if err := w.Write(mustMarshalEntry(writeEntry)); err != nil {
		fatal(t, "write points", err)
	}

	if err := w.Flush(); err != nil {
		fatal(t, "flush", err)
	}

	// Write the delete entry
	deleteEntry := &tsm1.DeleteWALEntry{
		Keys: [][]byte{[]byte("cpu,host=A#!~value")},
	}

	if err := w.Write(mustMarshalEntry(deleteEntry)); err != nil {
		fatal(t, "write points", err)
	}

	if err := w.Flush(); err != nil {
		fatal(t, "flush", err)
	}

	// Seek back to the beinning of the file for reading
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		fatal(t, "seek", err)
	}

	r := tsm1.NewWALSegmentReader(f)

	// Read the write points first
	if !r.Next() {
		t.Fatalf("expected next, got false")
	}

	we, err := r.Read()
	if err != nil {
		fatal(t, "read entry", err)
	}

	e, ok := we.(*tsm1.WriteWALEntry)
	if !ok {
		t.Fatalf("expected WriteWALEntry: got %#v", e)
	}

	for k, v := range e.Values {
		if got, exp := len(v), len(values[k]); got != exp {
			t.Fatalf("values length mismatch: got %v, exp %v", got, exp)
		}

		for i, vv := range v {
			if got, exp := vv.String(), values[k][i].String(); got != exp {
				t.Fatalf("points mismatch: got %v, exp %v", got, exp)
			}
		}
	}

	// Read the delete second
	if !r.Next() {
		t.Fatalf("expected next, got false")
	}

	we, err = r.Read()
	if err != nil {
		fatal(t, "read entry", err)
	}

	de, ok := we.(*tsm1.DeleteWALEntry)
	if !ok {
		t.Fatalf("expected DeleteWALEntry: got %#v", e)
	}

	if got, exp := len(de.Keys), len(deleteEntry.Keys); got != exp {
		t.Fatalf("key length mismatch: got %v, exp %v", got, exp)
	}

	if got, exp := string(de.Keys[0]), string(deleteEntry.Keys[0]); got != exp {
		t.Fatalf("key mismatch: got %v, exp %v", got, exp)
	}
}

func TestWALWriter_WriteMultiDeleteRange_Multiple(t *testing.T) {
	dir := MustTempDir()
	defer os.RemoveAll(dir)
	f := MustTempFile(dir)
	w := tsm1.NewWALSegmentWriter(f)

	p1 := tsm1.NewValue(1, 1.0)
	p2 := tsm1.NewValue(2, 2.0)
	p3 := tsm1.NewValue(3, 3.0)

	values := map[string][]tsm1.Value{
		"cpu,host=A#!~#value": []tsm1.Value{p1, p2, p3},
	}

	writeEntry := &tsm1.WriteWALEntry{
		Values: values,
	}

	if err := w.Write(mustMarshalEntry(writeEntry)); err != nil {
		fatal(t, "write points", err)
	}

	if err := w.Flush(); err != nil {
		fatal(t, "flush", err)
	}

	// Write the delete entry
	deleteEntry := &tsm1.DeleteRangeWALEntry{
		Keys: [][]byte{[]byte("cpu,host=A#!~value")},
		Min:  2,
		Max:  3,
	}

	if err := w.Write(mustMarshalEntry(deleteEntry)); err != nil {
		fatal(t, "write points", err)
	}

	if err := w.Flush(); err != nil {
		fatal(t, "flush", err)
	}

	// Seek back to the beinning of the file for reading
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		fatal(t, "seek", err)
	}

	r := tsm1.NewWALSegmentReader(f)

	// Read the write points first
	if !r.Next() {
		t.Fatalf("expected next, got false")
	}

	we, err := r.Read()
	if err != nil {
		fatal(t, "read entry", err)
	}

	e, ok := we.(*tsm1.WriteWALEntry)
	if !ok {
		t.Fatalf("expected WriteWALEntry: got %#v", e)
	}

	for k, v := range e.Values {
		if got, exp := len(v), len(values[k]); got != exp {
			t.Fatalf("values length mismatch: got %v, exp %v", got, exp)
		}

		for i, vv := range v {
			if got, exp := vv.String(), values[k][i].String(); got != exp {
				t.Fatalf("points mismatch: got %v, exp %v", got, exp)
			}
		}
	}

	// Read the delete second
	if !r.Next() {
		t.Fatalf("expected next, got false")
	}

	we, err = r.Read()
	if err != nil {
		fatal(t, "read entry", err)
	}

	de, ok := we.(*tsm1.DeleteRangeWALEntry)
	if !ok {
		t.Fatalf("expected DeleteWALEntry: got %#v", e)
	}

	if got, exp := len(de.Keys), len(deleteEntry.Keys); got != exp {
		t.Fatalf("key length mismatch: got %v, exp %v", got, exp)
	}

	if got, exp := string(de.Keys[0]), string(deleteEntry.Keys[0]); got != exp {
		t.Fatalf("key mismatch: got %v, exp %v", got, exp)
	}

	if got, exp := de.Min, int64(2); got != exp {
		t.Fatalf("min time mismatch: got %v, exp %v", got, exp)
	}

	if got, exp := de.Max, int64(3); got != exp {
		t.Fatalf("min time mismatch: got %v, exp %v", got, exp)
	}

}

func TestWAL_ClosedSegments(t *testing.T) {
	dir := MustTempDir()
	defer os.RemoveAll(dir)

	w := tsm1.NewWAL(dir)
	if err := w.Open(); err != nil {
		t.Fatalf("error opening WAL: %v", err)
	}

	files, err := w.ClosedSegments()
	if err != nil {
		t.Fatalf("error getting closed segments: %v", err)
	}

	if got, exp := len(files), 0; got != exp {
		t.Fatalf("close segment length mismatch: got %v, exp %v", got, exp)
	}

	if _, err := w.WriteMulti(map[string][]tsm1.Value{
		"cpu,host=A#!~#value": []tsm1.Value{
			tsm1.NewValue(1, 1.1),
		},
	}); err != nil {
		t.Fatalf("error writing points: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("error closing wal: %v", err)
	}

	// Re-open the WAL
	w = tsm1.NewWAL(dir)
	defer w.Close()
	if err := w.Open(); err != nil {
		t.Fatalf("error opening WAL: %v", err)
	}

	files, err = w.ClosedSegments()
	if err != nil {
		t.Fatalf("error getting closed segments: %v", err)
	}
	if got, exp := len(files), 0; got != exp {
		t.Fatalf("close segment length mismatch: got %v, exp %v", got, exp)
	}
}

func TestWAL_Delete(t *testing.T) {
	dir := MustTempDir()
	defer os.RemoveAll(dir)

	w := tsm1.NewWAL(dir)
	if err := w.Open(); err != nil {
		t.Fatalf("error opening WAL: %v", err)
	}

	files, err := w.ClosedSegments()
	if err != nil {
		t.Fatalf("error getting closed segments: %v", err)
	}

	if got, exp := len(files), 0; got != exp {
		t.Fatalf("close segment length mismatch: got %v, exp %v", got, exp)
	}

	if _, err := w.Delete([][]byte{[]byte("cpu")}); err != nil {
		t.Fatalf("error writing points: %v", err)
	}

	if err := w.Close(); err != nil {
		t.Fatalf("error closing wal: %v", err)
	}

	// Re-open the WAL
	w = tsm1.NewWAL(dir)
	defer w.Close()
	if err := w.Open(); err != nil {
		t.Fatalf("error opening WAL: %v", err)
	}

	files, err = w.ClosedSegments()
	if err != nil {
		t.Fatalf("error getting closed segments: %v", err)
	}
	if got, exp := len(files), 0; got != exp {
		t.Fatalf("close segment length mismatch: got %v, exp %v", got, exp)
	}
}

func TestWALWriter_Corrupt(t *testing.T) {
	dir := MustTempDir()
	defer os.RemoveAll(dir)
	f := MustTempFile(dir)
	w := tsm1.NewWALSegmentWriter(f)
	corruption := []byte{1, 4, 0, 0, 0}

	p1 := tsm1.NewValue(1, 1.1)
	values := map[string][]tsm1.Value{
		"cpu,host=A#!~#float": []tsm1.Value{p1},
	}

	entry := &tsm1.WriteWALEntry{
		Values: values,
	}
	if err := w.Write(mustMarshalEntry(entry)); err != nil {
		fatal(t, "write points", err)
	}

	if err := w.Flush(); err != nil {
		fatal(t, "flush", err)
	}

	// Write some random bytes to the file to simulate corruption.
	if _, err := f.Write(corruption); err != nil {
		fatal(t, "corrupt WAL segment", err)
	}

	// Create the WAL segment reader.
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		fatal(t, "seek", err)
	}
	r := tsm1.NewWALSegmentReader(f)

	// Try to decode two entries.

	if !r.Next() {
		t.Fatalf("expected next, got false")
	}
	if _, err := r.Read(); err != nil {
		fatal(t, "read entry", err)
	}

	if !r.Next() {
		t.Fatalf("expected next, got false")
	}
	if _, err := r.Read(); err == nil {
		fatal(t, "read entry did not return err", nil)
	}

	// Count should only return size of valid data.
	expCount := MustReadFileSize(f) - int64(len(corruption))
	if n := r.Count(); n != expCount {
		t.Fatalf("wrong count of bytes read, got %d, exp %d", n, expCount)
	}
}

// Reproduces a `panic: runtime error: makeslice: cap out of range` when run with
// GOARCH=386 go test -run TestWALSegmentReader_Corrupt -v ./tsdb/engine/tsm1/
func TestWALSegmentReader_Corrupt(t *testing.T) {
	dir := MustTempDir()
	defer os.RemoveAll(dir)
	f := MustTempFile(dir)
	w := tsm1.NewWALSegmentWriter(f)

	p4 := tsm1.NewValue(1, "string")

	values := map[string][]tsm1.Value{
		"cpu,host=A#!~#string": []tsm1.Value{p4, p4},
	}

	entry := &tsm1.WriteWALEntry{
		Values: values,
	}

	typ, b := mustMarshalEntry(entry)

	// This causes the nvals field to overflow on 32 bit systems which produces a
	// negative count and a panic when reading the segment.
	b[25] = 255

	if err := w.Write(typ, b); err != nil {
		fatal(t, "write points", err)
	}

	if err := w.Flush(); err != nil {
		fatal(t, "flush", err)
	}

	// Create the WAL segment reader.
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		fatal(t, "seek", err)
	}

	r := tsm1.NewWALSegmentReader(f)
	defer r.Close()

	// Try to decode two entries.
	for r.Next() {
		r.Read()
	}
}

func TestWriteWALSegment_UnmarshalBinary_WriteWALCorrupt(t *testing.T) {
	p1 := tsm1.NewValue(1, 1.1)
	p2 := tsm1.NewValue(1, int64(1))
	p3 := tsm1.NewValue(1, true)
	p4 := tsm1.NewValue(1, "string")
	p5 := tsm1.NewValue(1, uint64(1))

	values := map[string][]tsm1.Value{
		"cpu,host=A#!~#float":    []tsm1.Value{p1, p1},
		"cpu,host=A#!~#int":      []tsm1.Value{p2, p2},
		"cpu,host=A#!~#bool":     []tsm1.Value{p3, p3},
		"cpu,host=A#!~#string":   []tsm1.Value{p4, p4},
		"cpu,host=A#!~#unsigned": []tsm1.Value{p5, p5},
	}

	w := &tsm1.WriteWALEntry{
		Values: values,
	}

	b, err := w.MarshalBinary()
	if err != nil {
		t.Fatalf("unexpected error, got %v", err)
	}

	// Test every possible truncation of a write WAL entry
	for i := 0; i < len(b); i++ {
		// re-allocated to ensure capacity would be exceed if slicing
		truncated := make([]byte, i)
		copy(truncated, b[:i])
		err := w.UnmarshalBinary(truncated)
		if err != nil && err != tsm1.ErrWALCorrupt {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

func TestDeleteWALEntry_UnmarshalBinary(t *testing.T) {
	examples := []struct {
		In  []string
		Out [][]byte
	}{
		{
			In:  []string{""},
			Out: nil,
		},
		{
			In:  []string{"foo"},
			Out: [][]byte{[]byte("foo")},
		},
		{
			In:  []string{"foo", "bar"},
			Out: [][]byte{[]byte("foo"), []byte("bar")},
		},
		{
			In:  []string{"foo", "bar", "z", "abc"},
			Out: [][]byte{[]byte("foo"), []byte("bar"), []byte("z"), []byte("abc")},
		},
		{
			In:  []string{"foo", "bar", "z", "a"},
			Out: [][]byte{[]byte("foo"), []byte("bar"), []byte("z"), []byte("a")},
		},
	}

	for i, example := range examples {
		w := &tsm1.DeleteWALEntry{Keys: slices.StringsToBytes(example.In...)}
		b, err := w.MarshalBinary()
		if err != nil {
			t.Fatalf("[example %d] unexpected error, got %v", i, err)
		}

		out := &tsm1.DeleteWALEntry{}
		if err := out.UnmarshalBinary(b); err != nil {
			t.Fatalf("[example %d] %v", i, err)
		}

		if !reflect.DeepEqual(example.Out, out.Keys) {
			t.Errorf("[example %d] got %v, expected %v", i, out.Keys, example.Out)
		}
	}
}

func TestWriteWALSegment_UnmarshalBinary_DeleteWALCorrupt(t *testing.T) {
	w := &tsm1.DeleteWALEntry{
		Keys: [][]byte{[]byte("foo"), []byte("bar")},
	}

	b, err := w.MarshalBinary()
	if err != nil {
		t.Fatalf("unexpected error, got %v", err)
	}

	// Test every possible truncation of a write WAL entry
	for i := 0; i < len(b); i++ {
		// re-allocated to ensure capacity would be exceed if slicing
		truncated := make([]byte, i)
		copy(truncated, b[:i])
		err := w.UnmarshalBinary(truncated)
		if err != nil && err != tsm1.ErrWALCorrupt {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

func TestWriteWALSegment_UnmarshalBinary_DeleteRangeWALCorrupt(t *testing.T) {
	w := &tsm1.DeleteRangeWALEntry{
		Keys: [][]byte{[]byte("foo"), []byte("bar")},
		Min:  1,
		Max:  2,
	}

	b, err := w.MarshalBinary()
	if err != nil {
		t.Fatalf("unexpected error, got %v", err)
	}

	// Test every possible truncation of a write WAL entry
	for i := 0; i < len(b); i++ {
		// re-allocated to ensure capacity would be exceed if slicing
		truncated := make([]byte, i)
		copy(truncated, b[:i])
		err := w.UnmarshalBinary(truncated)
		if err != nil && err != tsm1.ErrWALCorrupt {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

func TestSegmentSize(t *testing.T) {
	dir := MustTempDir()
	defer os.RemoveAll(dir)

	const segSize = 1024 * 1024
	w := tsm1.NewWAL(dir)
	if err := w.Open(); err != nil {
		t.Fatalf("error opening WAL: %v", err)
	}
	w.SegmentSize = segSize

	var estimate int64

	for i := 0; i < 10000; i++ {
		values := make([]tsm1.Value, 0, 1024)
		for j := 0; j < cap(values); j++ {
			values = append(values, tsm1.NewValue(int64(j), float64(j)))
		}

		if _, err := w.WriteMulti(map[string][]tsm1.Value{
			fmt.Sprintf("series-key-%d", i): values}); err != nil {
			t.Fatalf("error writing points: %v", err)
		}
		if estimate == 0 {
			file := filepath.Join(dir, fmt.Sprintf("%s%05d.%s", tsm1.WALFilePrefix, 1, tsm1.WALFileExtension))
			fi, err := os.Stat(file)
			if err != nil {
				t.Fatalf("error stat file segment file : %v", err)
			}
			estimate = fi.Size()
		}
	}

	files, err := w.ClosedSegments()
	if err != nil {
		t.Fatalf("error getting closed segments: %v", err)
	}

	for _, file := range files {
		fi, err := os.Stat(file)
		if err != nil {
			t.Fatalf("error stat file segment file : %v", err)
		}

		if fi.Size() < segSize || fi.Size() > segSize+estimate {
			t.Fatalf("segment file not rolled as expected:%s size %d", file, fi.Size())
		}
	}

	if err := w.Close(); err != nil {
		t.Fatalf("error closing wal: %v", err)
	}

}

func BenchmarkWALSegmentWriter(b *testing.B) {
	points := map[string][]tsm1.Value{}
	for i := 0; i < 5000; i++ {
		k := "cpu,host=A#!~#value"
		points[k] = append(points[k], tsm1.NewValue(int64(i), 1.1))
	}

	dir := MustTempDir()
	defer os.RemoveAll(dir)

	f := MustTempFile(dir)
	w := tsm1.NewWALSegmentWriter(f)

	write := &tsm1.WriteWALEntry{
		Values: points,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := w.Write(mustMarshalEntry(write)); err != nil {
			b.Fatalf("unexpected error writing entry: %v", err)
		}
	}
}

func BenchmarkWALSegmentReader(b *testing.B) {
	points := map[string][]tsm1.Value{}
	for i := 0; i < 5000; i++ {
		k := "cpu,host=A#!~#value"
		points[k] = append(points[k], tsm1.NewValue(int64(i), 1.1))
	}

	dir := MustTempDir()
	defer os.RemoveAll(dir)

	f := MustTempFile(dir)
	w := tsm1.NewWALSegmentWriter(f)

	write := &tsm1.WriteWALEntry{
		Values: points,
	}

	for i := 0; i < 100; i++ {
		if err := w.Write(mustMarshalEntry(write)); err != nil {
			b.Fatalf("unexpected error writing entry: %v", err)
		}
	}

	r := tsm1.NewWALSegmentReader(f)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		f.Seek(0, io.SeekStart)
		b.StartTimer()

		for r.Next() {
			_, err := r.Read()
			if err != nil {
				b.Fatalf("unexpected error reading entry: %v", err)
			}
		}
	}
}

// MustReadFileSize returns the size of the file, or panics.
func MustReadFileSize(f *os.File) int64 {
	stat, err := os.Stat(f.Name())
	if err != nil {
		panic(fmt.Sprintf("failed to get size of file at %s: %s", f.Name(), err.Error()))
	}
	return stat.Size()
}

func mustMarshalEntry(entry tsm1.WALEntry) (tsm1.WalEntryType, []byte) {
	bytes := make([]byte, 1024<<2)

	b, err := entry.Encode(bytes)
	if err != nil {
		panic(fmt.Sprintf("error encoding: %v", err))
	}

	return entry.Type(), snappy.Encode(b, b)
}
