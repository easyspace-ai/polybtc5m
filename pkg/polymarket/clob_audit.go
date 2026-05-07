package polymarket

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// maxClobAuditBodyBytes caps stored response JSON so one bad payload cannot blow the file.
const maxClobAuditBodyBytes = 48 << 10

type clobAuditWriter struct {
	mu sync.Mutex
	f  *os.File
}

func newClobAuditWriter(path string) (*clobAuditWriter, error) {
	path = filepath.Clean(path)
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return &clobAuditWriter{f: f}, nil
}

func (w *clobAuditWriter) appendRecord(rec map[string]any) {
	if w == nil {
		return
	}
	rec["ts"] = time.Now().UTC().Format(time.RFC3339Nano)
	b, err := json.Marshal(rec)
	if err != nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	_, _ = w.f.Write(append(b, '\n'))
}

func truncateClobAuditBody(s string) string {
	if len(s) <= maxClobAuditBodyBytes {
		return s
	}
	return s[:maxClobAuditBodyBytes] + "…(truncated)"
}
