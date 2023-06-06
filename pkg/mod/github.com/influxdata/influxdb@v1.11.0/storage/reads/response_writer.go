package reads

import (
	"fmt"

	"github.com/influxdata/influxdb/models"
	"github.com/influxdata/influxdb/storage/reads/datatypes"
	"github.com/influxdata/influxdb/tsdb/cursors"

	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

type ResponseStream interface {
	Send(*datatypes.ReadResponse) error
	// SetTrailer sets the trailer metadata which will be sent with the RPC status.
	// When called more than once, all the provided metadata will be merged.
	SetTrailer(metadata.MD)
}

const (
	batchSize  = 1000
	frameCount = 50
	writeSize  = 64 << 10 // 64k
)

type ResponseWriter struct {
	stream ResponseStream
	res    *datatypes.ReadResponse
	err    error

	// sz is an estimated size in bytes for pending writes to flush periodically
	// when the size exceeds writeSize.
	sz int

	vc int // total value count

	buffer struct {
		Float          []*datatypes.ReadResponse_Frame_FloatPoints
		Integer        []*datatypes.ReadResponse_Frame_IntegerPoints
		Unsigned       []*datatypes.ReadResponse_Frame_UnsignedPoints
		Boolean        []*datatypes.ReadResponse_Frame_BooleanPoints
		String         []*datatypes.ReadResponse_Frame_StringPoints
		Series         []*datatypes.ReadResponse_Frame_Series
		Group          []*datatypes.ReadResponse_Frame_Group
		Multi          []*datatypes.ReadResponse_Frame_MultiPoints
		FloatValues    []*datatypes.ReadResponse_AnyPoints_Floats
		IntegerValues  []*datatypes.ReadResponse_AnyPoints_Integers
		UnsignedValues []*datatypes.ReadResponse_AnyPoints_Unsigneds
		BooleanValues  []*datatypes.ReadResponse_AnyPoints_Booleans
		StringValues   []*datatypes.ReadResponse_AnyPoints_Strings
	}

	hints datatypes.HintFlags
}

func NewResponseWriter(stream ResponseStream, hints datatypes.HintFlags) *ResponseWriter {
	rw := &ResponseWriter{stream: stream,
		res: &datatypes.ReadResponse{
			Frames: make([]*datatypes.ReadResponse_Frame, 0, frameCount),
		},
		hints: hints,
	}

	return rw
}

// WrittenN returns the number of values written to the response stream.
func (w *ResponseWriter) WrittenN() int { return w.vc }

func (w *ResponseWriter) WriteResultSet(rs ResultSet) error {
	for rs.Next() {
		cur := rs.Cursor()
		if cur == nil {
			// no data for series key + field combination
			continue
		}

		w.streamCursor(rs.Tags(), cur)
		if w.err != nil {
			cur.Close()
			return w.err
		}
	}

	stats := rs.Stats()
	w.stream.SetTrailer(metadata.Pairs(
		"scanned-bytes", fmt.Sprint(stats.ScannedBytes),
		"scanned-values", fmt.Sprint(stats.ScannedValues)))

	return rs.Err()
}

func (w *ResponseWriter) WriteGroupResultSet(rs GroupResultSet) error {
	stats := cursors.CursorStats{}
	gc := rs.Next()
	var err error
	for gc != nil {
		w.startGroup(gc.Keys(), gc.PartitionKeyVals())
		for gc.Next() {

			cur := gc.Cursor()
			if cur == nil {
				// no data for series key + field combination
				continue
			}

			w.streamCursor(gc.Tags(), cur)
			if w.err != nil {
				gc.Close()
				return w.err
			}
			stats.Add(gc.Stats())
		}
		gc.Close()
		if err = gc.Err(); err != nil {
			break
		}
		gc = rs.Next()
	}

	w.stream.SetTrailer(metadata.Pairs(
		"scanned-bytes", fmt.Sprint(stats.ScannedBytes),
		"scanned-values", fmt.Sprint(stats.ScannedValues)))

	return err
}

func (w *ResponseWriter) Err() error { return w.err }

func (w *ResponseWriter) getGroupFrame(keys, partitionKey [][]byte) *datatypes.ReadResponse_Frame_Group {
	var res *datatypes.ReadResponse_Frame_Group
	if len(w.buffer.Group) > 0 {
		i := len(w.buffer.Group) - 1
		res = w.buffer.Group[i]
		w.buffer.Group[i] = nil
		w.buffer.Group = w.buffer.Group[:i]
	} else {
		res = &datatypes.ReadResponse_Frame_Group{Group: &datatypes.ReadResponse_GroupFrame{}}
	}

	if cap(res.Group.TagKeys) < len(keys) {
		res.Group.TagKeys = make([][]byte, len(keys))
	} else if len(res.Group.TagKeys) != len(keys) {
		res.Group.TagKeys = res.Group.TagKeys[:len(keys)]
	}

	if cap(res.Group.PartitionKeyVals) < len(partitionKey) {
		res.Group.PartitionKeyVals = make([][]byte, len(partitionKey))
	} else if len(res.Group.PartitionKeyVals) != len(partitionKey) {
		res.Group.PartitionKeyVals = res.Group.PartitionKeyVals[:len(partitionKey)]
	}

	return res
}

func (w *ResponseWriter) putGroupFrame(f *datatypes.ReadResponse_Frame_Group) {
	for i := range f.Group.TagKeys {
		f.Group.TagKeys[i] = nil
	}
	for i := range f.Group.PartitionKeyVals {
		f.Group.PartitionKeyVals[i] = nil
	}
	w.buffer.Group = append(w.buffer.Group, f)
}

func (w *ResponseWriter) getSeriesFrame(lenTags int) *datatypes.ReadResponse_Frame_Series {
	var res *datatypes.ReadResponse_Frame_Series
	if len(w.buffer.Series) > 0 {
		i := len(w.buffer.Series) - 1
		res = w.buffer.Series[i]
		w.buffer.Series[i] = nil
		w.buffer.Series = w.buffer.Series[:i]
	} else {
		res = &datatypes.ReadResponse_Frame_Series{Series: &datatypes.ReadResponse_SeriesFrame{}}
	}

	if cap(res.Series.GetTags()) < lenTags {
		res.Series.Tags = make([]*datatypes.Tag, lenTags)
	} else if len(res.Series.GetTags()) != lenTags {
		res.Series.Tags = res.Series.Tags[:lenTags]
	}

	return res
}

func (w *ResponseWriter) putSeriesFrame(f *datatypes.ReadResponse_Frame_Series) {
	tags := f.Series.GetTags()
	for i := range tags {
		tags[i].Key = nil
		tags[i].Value = nil
	}
	w.buffer.Series = append(w.buffer.Series, f)
}

func (w *ResponseWriter) startGroup(keys, partitionKey [][]byte) {
	f := w.getGroupFrame(keys, partitionKey)
	copy(f.Group.TagKeys, keys)
	copy(f.Group.PartitionKeyVals, partitionKey)
	fr := &datatypes.ReadResponse_Frame{Data: f}
	w.res.Frames = append(w.res.Frames, fr)
	w.sz += proto.Size(fr)
}

func (w *ResponseWriter) startSeries(dataType datatypes.ReadResponse_DataType, next models.Tags) {
	f := w.getSeriesFrame(len(next))
	sf := f.Series
	sf.DataType = dataType
	for i, t := range next {
		sf.Tags[i] = &datatypes.Tag{
			Key:   t.Key,
			Value: t.Value,
		}
	}
	w.res.Frames = append(w.res.Frames, &datatypes.ReadResponse_Frame{Data: f})
	w.sz += proto.Size(sf)
}

func (w *ResponseWriter) streamCursor(tags models.Tags, cur cursors.Cursor) {
	switch {
	case w.hints.NoSeries():
		// skip
	case w.hints.NoPoints():
		switch cur := cur.(type) {
		case cursors.IntegerArrayCursor:
			w.streamIntegerArraySeries(tags, cur)
		case cursors.FloatArrayCursor:
			w.streamFloatArraySeries(tags, cur)
		case cursors.UnsignedArrayCursor:
			w.streamUnsignedArraySeries(tags, cur)
		case cursors.BooleanArrayCursor:
			w.streamBooleanArraySeries(tags, cur)
		case cursors.StringArrayCursor:
			w.streamStringArraySeries(tags, cur)
		case cursors.MeanCountArrayCursor:
			w.streamMeanCountArraySeries(tags, cur)
		default:
			panic(fmt.Sprintf("unreachable: %T", cur))
		}

	default:
		switch cur := cur.(type) {
		case cursors.IntegerArrayCursor:
			w.streamIntegerArrayPoints(tags, cur)
		case cursors.FloatArrayCursor:
			w.streamFloatArrayPoints(tags, cur)
		case cursors.UnsignedArrayCursor:
			w.streamUnsignedArrayPoints(tags, cur)
		case cursors.BooleanArrayCursor:
			w.streamBooleanArrayPoints(tags, cur)
		case cursors.StringArrayCursor:
			w.streamStringArrayPoints(tags, cur)
		case cursors.MeanCountArrayCursor:
			w.streamMeanCountArrayPoints(tags, cur)
		default:
			panic(fmt.Sprintf("unreachable: %T", cur))
		}
	}
	cur.Close()
}

func (w *ResponseWriter) Flush() {
	// We defer the clearing of the frames array,
	// as we should attempt that regardless of other errors.
	defer func() {
		w.sz = 0
		for i := range w.res.Frames {
			d := w.res.Frames[i].Data
			w.res.Frames[i].Data = nil
			switch p := d.(type) {
			case *datatypes.ReadResponse_Frame_FloatPoints:
				w.putFloatPointsFrame(p)
			case *datatypes.ReadResponse_Frame_IntegerPoints:
				w.putIntegerPointsFrame(p)
			case *datatypes.ReadResponse_Frame_UnsignedPoints:
				w.putUnsignedPointsFrame(p)
			case *datatypes.ReadResponse_Frame_BooleanPoints:
				w.putBooleanPointsFrame(p)
			case *datatypes.ReadResponse_Frame_StringPoints:
				w.putStringPointsFrame(p)
			case *datatypes.ReadResponse_Frame_MultiPoints:
				w.putMultiPointsFrame(p)
			case *datatypes.ReadResponse_Frame_Series:
				w.putSeriesFrame(p)
			case *datatypes.ReadResponse_Frame_Group:
				w.putGroupFrame(p)
			}
		}
		w.res.Frames = w.res.Frames[:0]
	}()

	if w.err != nil || w.sz == 0 {
		return
	}

	if w.err = w.stream.Send(w.res); w.err != nil {
		return
	}
}

// The MultiPoints <==> MeanCount converters do not fit the codegen pattern in response_writer.gen.go

func (w *ResponseWriter) getMultiPointsFrameForMeanCount() *datatypes.ReadResponse_Frame_MultiPoints {
	var res *datatypes.ReadResponse_Frame_MultiPoints
	if len(w.buffer.Multi) > 0 {
		i := len(w.buffer.Multi) - 1
		res = w.buffer.Multi[i]
		w.buffer.Multi[i] = nil
		w.buffer.Multi = w.buffer.Multi[:i]
	} else {
		res = &datatypes.ReadResponse_Frame_MultiPoints{
			MultiPoints: &datatypes.ReadResponse_MultiPointsFrame{
				Timestamps: make([]int64, 0, batchSize),
			},
		}
	}
	res.MultiPoints.ValueArrays = append(res.MultiPoints.ValueArrays, &datatypes.ReadResponse_AnyPoints{Data: w.getFloatValues()})
	res.MultiPoints.ValueArrays = append(res.MultiPoints.ValueArrays, &datatypes.ReadResponse_AnyPoints{Data: w.getIntegerValues()})
	return res
}

func (w *ResponseWriter) putMultiPointsFrame(f *datatypes.ReadResponse_Frame_MultiPoints) {
	f.MultiPoints.Timestamps = f.MultiPoints.Timestamps[:0]
	for _, v := range f.MultiPoints.GetValueArrays() {
		switch v := v.Data.(type) {
		case *datatypes.ReadResponse_AnyPoints_Floats:
			w.putFloatValues(v)
		case *datatypes.ReadResponse_AnyPoints_Integers:
			w.putIntegerValues(v)
		case *datatypes.ReadResponse_AnyPoints_Unsigneds:
			w.putUnsignedValues(v)
		case *datatypes.ReadResponse_AnyPoints_Booleans:
			w.putBooleanValues(v)
		case *datatypes.ReadResponse_AnyPoints_Strings:
			w.putStringValues(v)
		}
	}
	f.MultiPoints.ValueArrays = f.MultiPoints.ValueArrays[:0]
	w.buffer.Multi = append(w.buffer.Multi, f)
}

func (w *ResponseWriter) streamMeanCountArraySeries(tags models.Tags, cur cursors.MeanCountArrayCursor) {
	a := cur.Next()
	if a.Len() != 0 {
		w.startSeries(datatypes.ReadResponse_DataTypeMulti, tags)
	} else if w.sz > writeSize {
		w.Flush()
	}
}

func (w *ResponseWriter) streamMeanCountArrayPoints(tags models.Tags, cur cursors.MeanCountArrayCursor) {

	var frame *datatypes.ReadResponse_MultiPointsFrame

	var seriesValueCount = 0
	for {
		// If the number of values produced by cur > 1000,
		// cur.Next() will produce batches of values that are of
		// length ≤ 1000.
		// We attempt to limit the frame Timestamps / Values lengths
		// the same to avoid allocations. These frames are recycled
		// after flushing so that on repeated use there should be enough space
		// to append values from a into frame without additional allocations.
		a := cur.Next()

		if a.Len() == 0 {
			break
		}

		if seriesValueCount == 0 {
			w.startSeries(datatypes.ReadResponse_DataTypeMulti, tags)
		}

		seriesValueCount += a.Len()

		if frame == nil {
			p := w.getMultiPointsFrameForMeanCount()
			frame = p.MultiPoints
			w.res.Frames = append(w.res.Frames, &datatypes.ReadResponse_Frame{Data: p})
		}

		// As specified in the struct definition, w.sz is an estimated
		// size (in bytes) of the buffered data. It is therefore a
		// deliberate choice to accumulate using the array Size, which is
		// cheap to calculate. Calling frame.Size() can be expensive
		// when using varint encoding for numbers.
		w.sz += a.Size()

		frame.Timestamps = append(frame.Timestamps, a.Timestamps...)
		// This is guaranteed to be the right layout since we called getMultiPointsFrameForMeanCount.
		frame.ValueArrays[0].GetFloats().Values = append(frame.ValueArrays[0].GetFloats().Values, a.Values0...)
		frame.ValueArrays[1].GetIntegers().Values = append(frame.ValueArrays[1].GetIntegers().Values, a.Values1...)

		// given the expectation of cur.Next, we attempt to limit
		// the number of values appended to the frame to batchSize (1000)
		if len(frame.Timestamps) >= batchSize {
			frame = nil
		}

		if w.sz >= writeSize {
			frame = nil
			w.Flush()
			if w.err != nil {
				break
			}
		}
	}
	w.vc += seriesValueCount
	if w.sz > writeSize {
		w.Flush()
	}
}
