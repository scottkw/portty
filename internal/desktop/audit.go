package desktop

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// auditFileName is the JSON Lines audit file inside the portty config dir.
const auditFileName = "desktop-audit.jsonl"

// maxAuditBytes caps the audit file; past it the file rolls over to
// desktop-audit.jsonl.1 (one old file, so history older than about two caps is
// deleted).
const maxAuditBytes = 5 << 20

// auditLogInterval is the minimum gap between "audit write failed" log lines.
const auditLogInterval = time.Minute

// Audit event names. The set is part of the persisted v1 format.
const (
	AuditShareStart       = "share_start"
	AuditShareStop        = "share_stop"
	AuditAdmitRequest     = "admit_request"
	AuditAdmitted         = "admitted"
	AuditDenied           = "denied"
	AuditJoin             = "join"
	AuditLeave            = "leave"
	AuditRevoked          = "revoked"
	AuditControlGranted   = "control_granted"
	AuditControlRevoked   = "control_revoked"
	AuditClipboardGranted = "clipboard_granted"
	AuditClipboardRevoked = "clipboard_revoked"

	// Remote unlock (UNLK-01): one line per attempt that typed, one when the Mac
	// was read locked again after the last own-account viewer left, and one when
	// the re-lock's chords did not lock it (the rule keeps trying).
	AuditUnlocked     = "unlocked"
	AuditUnlockFailed = "unlock_failed"
	AuditRelocked     = "relocked"
	AuditRelockFailed = "relock_failed"
)

// Audit modes: guest events carry a share ID, own-account events do not.
const (
	AuditModeGuest = "guest"
	AuditModeOwn   = "own"
)

// Audit reasons (the additive v1 `reason` key). Fixed constants only; never
// free text, so a reason cannot carry a secret or a name.
const (
	ReasonViewerClosed          = "viewer_closed"
	ReasonRevoked               = "revoked"
	ReasonShareStopped          = "share_stopped"
	ReasonGuiGone               = "gui_gone"
	ReasonDaemonExit            = "daemon_exit"
	ReasonKeyRegenerated        = "key_regenerated"
	ReasonTimeout               = "timeout"
	ReasonReplaced              = "replaced"
	ReasonMediaIdentityMismatch = "media_identity_mismatch"
	ReasonDenied                = "denied"
	ReasonStillLocked           = "still_locked" // unlock_failed: the Mac was still locked after the retry
	ReasonUnlockError           = "error"        // unlock_failed: typing failed part-way
)

// AuditRecord is one audit line. The field order is the persisted v1 format:
// later builds keep appending to the same file, so do not reorder. There is
// deliberately no field that could hold a token, typed input, key codes,
// clipboard text, SDP or frame data.
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

// Audit appends AuditRecord lines to dir/desktop-audit.jsonl. Record never
// returns an error and never blocks a join: audit is best effort, because a
// full disk must not lock a guest (or the sharer) out.
type Audit struct {
	mu      sync.Mutex
	path    string
	f       *os.File
	size    int64
	max     int64
	now     func() time.Time
	logf    func(op string, err error)
	lastLog time.Time
}

// OpenAudit opens dir/desktop-audit.jsonl for append, mode 0600 (an existing
// file with wider permissions is narrowed). The size is seeded from Stat so a
// restart does not rotate a small file.
func OpenAudit(dir string) (*Audit, error) {
	path := filepath.Join(dir, auditFileName)
	f, size, err := openAuditFile(path)
	if err != nil {
		return nil, err
	}
	return &Audit{path: path, f: f, size: size, max: maxAuditBytes, now: time.Now, logf: logAuditFailure}, nil
}

func openAuditFile(path string) (*os.File, int64, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, 0, err
	}
	if err := f.Chmod(0600); err != nil {
		_ = f.Close()
		return nil, 0, err
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, 0, err
	}
	return f, st.Size(), nil
}

// Record appends one line with a single Write call, so concurrent appenders
// (an old and a new daemon overlapping during an upgrade takeover) cannot
// interleave inside a line.
func (a *Audit) Record(event, shareID, mode string, id Identity, reason string) {
	a.mu.Lock()
	defer a.mu.Unlock()
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
		a.fail("marshal", err)
		return
	}
	line = append(line, '\n')
	if a.size+int64(len(line)) > a.max {
		a.rotate()
	}
	n, err := a.f.Write(line)
	a.size += int64(n)
	if err != nil {
		a.fail("write", err)
	}
}

// rotate must be called with mu held. It renames the file to path.1 (replacing
// an older one) and starts a fresh 0600 file. On failure it keeps writing to
// the current file and retries after another max bytes, as daemon_log.go does.
func (a *Audit) rotate() {
	a.size = 0
	if err := os.Rename(a.path, a.path+".1"); err != nil {
		a.fail("rotate", err)
		return
	}
	f, size, err := openAuditFile(a.path)
	if err != nil {
		a.fail("rotate", err)
		return
	}
	_ = a.f.Close()
	a.f, a.size = f, size
}

// fail logs at most once per auditLogInterval and drops the event. Must be
// called with mu held.
func (a *Audit) fail(op string, err error) {
	now := a.now()
	if !a.lastLog.IsZero() && now.Sub(a.lastLog) < auditLogInterval {
		return
	}
	a.lastLog = now
	a.logf(op, err)
}

// Close closes the audit file.
func (a *Audit) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.f.Close()
}

// logAuditFailure logs the failing operation and the error class (the
// underlying errno, not the path-bearing wrapper), never the record.
func logAuditFailure(op string, err error) {
	var pe *fs.PathError
	var le *os.LinkError
	switch {
	case errors.As(err, &pe):
		err = pe.Err
	case errors.As(err, &le):
		err = le.Err
	}
	slog.Warn("desktop audit: event dropped", "op", op, "err", err)
}
