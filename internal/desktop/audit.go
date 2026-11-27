package desktop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// auditFileName is the JSON Lines audit file inside the portty config dir.
const auditFileName = "desktop-audit.jsonl"

// AuditRecord is one audit line. The field order is the persisted v1 format:
// later builds keep appending to the same file, so do not reorder. There is
// deliberately no field that could hold a token, typed input, clipboard text,
// SDP or frame data.
type AuditRecord struct {
	V            int    `json:"v"`
	TS           string `json:"ts"`
	Event        string `json:"event"`
	ShareID      string `json:"share_id"`
	Mode         string `json:"mode"`
	UserID       int64  `json:"user_id"`
	LoginName    string `json:"login_name"`
	DisplayName  string `json:"display_name"`
	NodeName     string `json:"node_name"`
	NodeStableID string `json:"node_stable_id"`
	TailnetIP    string `json:"tailnet_ip"`
	Reason       string `json:"reason,omitempty"`
}

// Audit appends AuditRecord lines to dir/desktop-audit.jsonl.
type Audit struct {
	mu   sync.Mutex
	path string
	f    *os.File
	now  func() time.Time
}

// OpenAudit opens dir/desktop-audit.jsonl for append, mode 0600 (an existing
// file with wider permissions is narrowed).
func OpenAudit(dir string) (*Audit, error) {
	path := filepath.Join(dir, auditFileName)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}
	if err := f.Chmod(0600); err != nil {
		_ = f.Close()
		return nil, err
	}
	return &Audit{path: path, f: f, now: time.Now}, nil
}

// Record appends one line: a single Write call, so concurrent appenders
// cannot interleave inside a line.
func (a *Audit) Record(event, shareID, mode string, id Identity, reason string) {
	line, err := json.Marshal(AuditRecord{
		V:            1,
		TS:           a.now().UTC().Format(time.RFC3339),
		Event:        event,
		ShareID:      shareID,
		Mode:         mode,
		UserID:       id.UserID,
		LoginName:    id.LoginName,
		DisplayName:  id.DisplayName,
		NodeName:     id.NodeName,
		NodeStableID: id.StableID,
		TailnetIP:    id.TailnetIP,
		Reason:       reason,
	})
	if err != nil {
		return
	}
	line = append(line, '\n')
	a.mu.Lock()
	defer a.mu.Unlock()
	_, _ = a.f.Write(line)
}
