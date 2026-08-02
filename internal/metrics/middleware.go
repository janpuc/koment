package metrics

import (
	"net/http"
	"time"
)

// Instrument records a handler's requests. It takes a Recorder rather than
// *Metrics so that a server with metrics off pays a nil-ish interface call and
// nothing else.
func Instrument(recorder Recorder, name string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recording := &recordingWriter{ResponseWriter: w, code: http.StatusOK}
		next.ServeHTTP(recording, r)
		recorder.ObserveHTTP(name, recording.code, time.Since(started))
	})
}

// recordingWriter remembers the status code, which net/http otherwise discards
// once it has been written.
type recordingWriter struct {
	http.ResponseWriter
	code    int
	written bool
}

func (w *recordingWriter) WriteHeader(code int) {
	if !w.written {
		w.code = code
		w.written = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *recordingWriter) Write(b []byte) (int, error) {
	w.written = true
	return w.ResponseWriter.Write(b)
}
