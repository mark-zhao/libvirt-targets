package opentsdb

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/influxdata/influxdb"
	"github.com/influxdata/influxdb/models"
	"github.com/influxdata/influxdb/tsdb"
	"go.uber.org/zap"
)

// Handler is an http.Handler for the OpenTSDB service.
type Handler struct {
	Database        string
	RetentionPolicy string

	PointsWriter interface {
		WritePointsPrivileged(ctx tsdb.WriteContext, database, retentionPolicy string, consistencyLevel models.ConsistencyLevel, points []models.Point) error
	}

	Logger *zap.Logger

	stats *Statistics
}

// ServeHTTP handles an HTTP request of the OpenTSDB REST API.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/metadata/put":
		w.WriteHeader(http.StatusNoContent)
	case "/api/put":
		h.servePut(w, r)
	default:
		http.NotFound(w, r)
	}
}

// servePut implements OpenTSDB's HTTP /api/put endpoint.
func (h *Handler) servePut(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	// Require POST method.
	if r.Method != "POST" {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	// Wrap reader if it's gzip encoded.
	var br *bufio.Reader
	if r.Header.Get("Content-Encoding") == "gzip" {
		zr, err := gzip.NewReader(r.Body)
		if err != nil {
			http.Error(w, "could not read gzip, "+err.Error(), http.StatusBadRequest)
			return
		}

		br = bufio.NewReader(zr)
	} else {
		br = bufio.NewReader(r.Body)
	}

	// Lookahead at the first byte.
	f, err := br.Peek(1)
	if err != nil || len(f) != 1 {
		http.Error(w, "peek error: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Peek to see if this is a JSON array.
	var multi bool
	switch f[0] {
	case '{':
	case '[':
		multi = true
	default:
		http.Error(w, "expected JSON array or hash", http.StatusBadRequest)
		return
	}

	// Decode JSON data into slice of points.
	dps := make([]point, 1)
	if dec := json.NewDecoder(br); multi {
		if err = dec.Decode(&dps); err != nil {
			http.Error(w, "json array decode error", http.StatusBadRequest)
			return
		}
	} else {
		if err = dec.Decode(&dps[0]); err != nil {
			http.Error(w, "json object decode error", http.StatusBadRequest)
			return
		}
	}

	// Convert points into TSDB points.
	points := make([]models.Point, 0, len(dps))
	for i := range dps {
		p := dps[i]

		// Convert timestamp to Go time.
		// If time value is over a billion then it's microseconds.
		var ts time.Time
		if p.Time < 10000000000 {
			ts = time.Unix(p.Time, 0)
		} else {
			ts = time.Unix(p.Time/1000, (p.Time%1000)*1000)
		}

		pt, err := models.NewPoint(p.Metric, models.NewTags(p.Tags), map[string]interface{}{"value": p.Value}, ts)
		if err != nil {
			h.Logger.Info("Dropping point", zap.String("name", p.Metric), zap.Error(err))
			if h.stats != nil {
				atomic.AddInt64(&h.stats.InvalidDroppedPoints, 1)
			}
			continue
		}
		points = append(points, pt)
	}

	writeCtx := tsdb.WriteContext{
		UserId: tsdb.OpenTsdbUser,
	}

	// Write points.
	if err := h.PointsWriter.WritePointsPrivileged(writeCtx, h.Database, h.RetentionPolicy, models.ConsistencyLevelAny, points); influxdb.IsClientError(err) {
		h.Logger.Info("Write series error", zap.Error(err))
		http.Error(w, "write series error: "+err.Error(), http.StatusBadRequest)
		return
	} else if err != nil {
		h.Logger.Info("Write series error", zap.Error(err))
		http.Error(w, "write series error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// chanListener represents a listener that receives connections through a channel.
type chanListener struct {
	addr   net.Addr
	ch     chan net.Conn
	done   chan struct{}
	closer sync.Once // closer ensures that Close is idempotent.
}

// newChanListener returns a new instance of chanListener.
func newChanListener(addr net.Addr) *chanListener {
	return &chanListener{
		addr: addr,
		ch:   make(chan net.Conn),
		done: make(chan struct{}),
	}
}

func (ln *chanListener) Accept() (net.Conn, error) {
	errClosed := errors.New("network connection closed")
	select {
	case <-ln.done:
		return nil, errClosed
	case conn, ok := <-ln.ch:
		if !ok {
			return nil, errClosed
		}
		return conn, nil
	}
}

// Close closes the connection channel.
func (ln *chanListener) Close() error {
	ln.closer.Do(func() {
		close(ln.done)
	})
	return nil
}

// Addr returns the network address of the listener.
func (ln *chanListener) Addr() net.Addr { return ln.addr }

// readerConn represents a net.Conn with an assignable reader.
type readerConn struct {
	net.Conn
	r io.Reader
}

// Read implements the io.Reader interface.
func (conn *readerConn) Read(b []byte) (n int, err error) { return conn.r.Read(b) }

// point represents an incoming JSON data point.
type point struct {
	Metric string            `json:"metric"`
	Time   int64             `json:"timestamp"`
	Value  float64           `json:"value"`
	Tags   map[string]string `json:"tags,omitempty"`
}
