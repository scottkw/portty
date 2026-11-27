package desktop

// runtime_state.go: desktop-state.json, the Manager's own memory across
// restarts: when Portty last saw a fresh Screen Recording approval (lapse
// signal b), whether capture was running (so an exit that did not clean up is
// counted), and the recent abnormal exits (the crash-loop breaker). It is not
// desktop.json: that file's schema stays at its four accepted fields.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/scottkw/portty/internal/screencap"
)

const (
	runtimeStateFile = "desktop-state.json"
	runtimeSchema    = 1

	// approvalPolicy is macOS's monthly re-approval of Screen Recording (R19);
	// the warning starts lapseWarnBefore ahead of it.
	approvalPolicy  = 30 * 24 * time.Hour
	lapseWarnBefore = 3 * 24 * time.Hour

	// breakerTrips abnormal exits within breakerWindow open the breaker: a hard
	// capture crash ends every terminal session, so a capture that keeps
	// crashing must stop restarting itself (T-19-31).
	breakerTrips  = 3
	breakerWindow = time.Hour
)

type runtimeState struct {
	SchemaVersion  int     `json:"schemaVersion"`
	ApprovalSeenAt int64   `json:"approvalSeenAt"`
	CaptureActive  bool    `json:"captureActive"`
	AbnormalExits  []int64 `json:"abnormalExits"`
}

// rtMu serialises read-modify-write of the file inside this process (the
// Manager and NoteTestCapture, which the daemon calls beside it).
var rtMu sync.Mutex

// readRuntimeState reads dir's state file. A missing file is an empty state;
// a symlink, a non-regular or group/world-accessible file, unknown fields or an
// unknown schema are errors, never a silent reset (T-19-32).
func readRuntimeState(dir string) (runtimeState, error) {
	path := filepath.Join(dir, runtimeStateFile)
	fi, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return runtimeState{SchemaVersion: runtimeSchema}, nil
	}
	if err != nil {
		return runtimeState{}, err
	}
	if !fi.Mode().IsRegular() {
		return runtimeState{}, fmt.Errorf("%s is not a regular file", runtimeStateFile)
	}
	if fi.Mode().Perm()&0o077 != 0 {
		return runtimeState{}, fmt.Errorf("%s is accessible to others (mode %v)", runtimeStateFile, fi.Mode().Perm())
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return runtimeState{}, err
	}
	var rs runtimeState
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&rs); err != nil {
		return runtimeState{}, fmt.Errorf("%s: %w", runtimeStateFile, err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return runtimeState{}, fmt.Errorf("%s: trailing data", runtimeStateFile)
	}
	if rs.SchemaVersion != runtimeSchema {
		return runtimeState{}, fmt.Errorf("%s: unknown schemaVersion %d", runtimeStateFile, rs.SchemaVersion)
	}
	return rs, nil
}

// writeRuntimeState writes the file 0600 through a temp file and a rename.
func writeRuntimeState(dir string, rs runtimeState) error {
	rs.SchemaVersion = runtimeSchema
	b, err := json.Marshal(rs)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, runtimeStateFile+".*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, err = f.Write(b)
	if err == nil {
		err = f.Sync() // a power cut after the rename must not leave an empty file
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(tmp, 0o600)
	}
	if err == nil {
		err = os.Rename(tmp, filepath.Join(dir, runtimeStateFile))
	}
	if err != nil {
		_ = os.Remove(tmp)
	}
	return err
}

// updateRuntimeState reads, applies fn and writes. A file that cannot be read
// is returned as an error and left untouched.
func updateRuntimeState(dir string, fn func(*runtimeState)) (runtimeState, error) {
	rtMu.Lock()
	defer rtMu.Unlock()
	rs, err := readRuntimeState(dir)
	if err != nil {
		return runtimeState{}, err
	}
	fn(&rs)
	return rs, writeRuntimeState(dir, rs)
}

// noteApproval is lapse signal (b): record when a fresh approval was last seen,
// at a capture start with no record or a record 30 days old or more (macOS
// raises its prompt on that start and resets its own clock).
func (rs *runtimeState) noteApproval(now time.Time) {
	if rs.ApprovalSeenAt == 0 || now.Sub(time.Unix(rs.ApprovalSeenAt, 0)) >= approvalPolicy {
		rs.ApprovalSeenAt = now.Unix()
	}
}

// pruneExits drops abnormal exits older than breakerWindow.
func (rs *runtimeState) pruneExits(now time.Time) {
	kept := rs.AbnormalExits[:0]
	for _, t := range rs.AbnormalExits {
		if age := now.Sub(time.Unix(t, 0)); age >= 0 && age < breakerWindow {
			kept = append(kept, t)
		}
	}
	rs.AbnormalExits = kept
}

// NoteTestCapture records a successful Test Capture (19-11): the approval rule
// of a capture start, and a cleared breaker. It works with or without a running
// Manager, because it only touches the file.
func NoteTestCapture(configDir string, now time.Time) error {
	_, err := updateRuntimeState(configDir, func(rs *runtimeState) {
		rs.noteApproval(now)
		rs.AbnormalExits = nil
	})
	return err
}

// persist applies fn to the state file when there is one and it is readable;
// ok reports whether the file now holds the result. A failure is a reason in
// Status, never a stop (capture and viewing carry on).
func (m *Manager) persist(fn func(*runtimeState)) (rs runtimeState, ok bool) {
	m.smu.Lock()
	off := m.rtOff
	m.smu.Unlock()
	if m.d.ConfigDir == "" || off {
		return runtimeState{}, false
	}
	rs, err := updateRuntimeState(m.d.ConfigDir, fn)
	if err != nil {
		m.noteReason("state file", err)
		return runtimeState{}, false
	}
	m.clearReason("state file")
	return rs, true
}

// loadRuntimeState runs at NewManager: a capture that was active when the last
// process died is one abnormal exit (a clean stop clears captureActive), and
// breakerTrips of them within breakerWindow open the breaker. An unreadable
// file switches persistence off and says why; it is never reset (T-19-32).
func (m *Manager) loadRuntimeState() {
	if m.d.ConfigDir == "" {
		return
	}
	now := m.d.Now().Round(0)
	crashed := false
	rs, err := updateRuntimeState(m.d.ConfigDir, func(rs *runtimeState) {
		if rs.CaptureActive {
			crashed = true
			rs.AbnormalExits = append(rs.AbnormalExits, now.Unix())
			rs.CaptureActive = false
		}
		rs.pruneExits(now)
	})
	if err != nil {
		m.rtOff = true
		m.reasons["state file"] = "state file: " + err.Error()
		slog.Warn("desktop: state file unusable, persistence off", "err", err)
		return
	}
	if crashed {
		slog.Warn("desktop: previous capture ended abnormally", "recent", len(rs.AbnormalExits))
	}
	if rs.ApprovalSeenAt != 0 {
		m.approvalSeen = time.Unix(rs.ApprovalSeenAt, 0)
	}
	if m.paused = len(rs.AbnormalExits) >= breakerTrips; m.paused {
		slog.Error("desktop: capture paused after repeated abnormal exits; a successful Test Capture resumes it", "exits", len(rs.AbnormalExits))
	}
}

func (m *Manager) isPaused() bool {
	m.smu.Lock()
	defer m.smu.Unlock()
	return m.paused
}

// ResetBreaker clears the abnormal exits and the paused state (a successful
// Test Capture in 19-11 calls it); the next join starts capture.
func (m *Manager) ResetBreaker() error {
	_, _ = m.persist(func(rs *runtimeState) { rs.AbnormalExits = nil })
	m.smu.Lock()
	m.paused = false
	m.smu.Unlock()
	m.kickLoop()
	return nil
}

// noteStartFailure classifies a failed capture start: -3801 is needs_permission,
// -3815 is no display (display_asleep); either way the next try waits.
func (m *Manager) noteStartFailure(err error, now time.Time) {
	m.smu.Lock()
	switch {
	case errors.Is(err, screencap.ErrPermissionDenied):
		m.permDenied = true
	case errors.Is(err, screencap.ErrNoDisplay):
		m.noDisplay = true
	}
	m.nextStartAt = now.Add(retryStartEvery)
	m.smu.Unlock()
	slog.Warn("desktop: capture start failed", "err", err)
}

// noteStartSuccess marks a capture start: the stall clock begins, the failure
// flags clear, the approval record rule runs (signal b) and captureActive is
// written so an exit that skips the clean stop is counted next time.
func (m *Manager) noteStartSuccess(src screencap.Source, now time.Time) {
	m.smu.Lock()
	m.lastFrameAt = now
	m.permDenied, m.noDisplay = false, false
	m.smu.Unlock()
	if h, ok := src.(interface{ HardwareEncoder() (hardware, known bool) }); ok {
		if hw, known := h.HardwareEncoder(); known && !hw {
			m.noteDiagnostic("encoder", "software encoder")
		} else {
			m.clearReason("encoder")
		}
	}
	rs, ok := m.persist(func(rs *runtimeState) {
		// Read under rtMu, which Close's clear also takes: a Close that set the
		// flag first is never undone by a start finishing after it.
		rs.CaptureActive = !m.closed.Load()
		rs.noteApproval(now)
	})
	m.smu.Lock()
	switch {
	case ok && rs.ApprovalSeenAt != 0:
		m.approvalSeen = time.Unix(rs.ApprovalSeenAt, 0)
	case !ok && (m.approvalSeen.IsZero() || now.Sub(m.approvalSeen) >= approvalPolicy):
		m.approvalSeen = now
	}
	m.smu.Unlock()
	m.refreshApproval(m.ctx, now, true)
}

// noteCaptureStopped is the clean stop: captureActive false, lapse over.
func (m *Manager) noteCaptureStopped() {
	_, _ = m.persist(func(rs *runtimeState) { rs.CaptureActive = false })
	m.smu.Lock()
	m.awaiting = false
	m.smu.Unlock()
}

// refreshApproval recomputes ApproachingLapse (signals a and b): from the
// approval store's LastAlerted when that store is readable (it needs Full Disk
// Access, which Portty never asks for, so usually it is not), otherwise from
// Portty's own record. It runs at most every idleProbeEvery unless forced.
func (m *Manager) refreshApproval(ctx context.Context, now time.Time, force bool) {
	m.smu.Lock()
	if age := now.Sub(m.approvalAt); !force && !m.approvalAt.IsZero() && age >= 0 && age < idleProbeEvery {
		m.smu.Unlock()
		return
	}
	m.approvalAt = now
	m.smu.Unlock()

	store, err := m.d.ApprovalStore(ctx)
	storeOK := err == nil && !store.IsZero()
	var own time.Time
	if m.d.ConfigDir != "" {
		if rs, err := readRuntimeState(m.d.ConfigDir); err == nil && rs.ApprovalSeenAt != 0 {
			own = time.Unix(rs.ApprovalSeenAt, 0)
		}
	}

	m.smu.Lock()
	defer m.smu.Unlock()
	if !own.IsZero() {
		m.approvalSeen = own
	}
	base := m.approvalSeen
	if storeOK {
		base = store
	}
	age := now.Sub(base)
	m.approaching = !base.IsZero() && age >= approvalPolicy-lapseWarnBefore && age < approvalPolicy
}
