package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"image/color"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	xvt "github.com/charmbracelet/x/vt"
	"github.com/scottkw/agenthub/internal/pty"
	"github.com/scottkw/agenthub/internal/relay"
	"github.com/scottkw/agenthub/internal/status"
)

// SessionEngine owns all session state: registry, backend, hub manager,
// tab names, CLI path overrides, and per-session status.
// It is intentionally free of Wails imports — callers supply an onExit
// callback if they need exit notification.
type SessionEngine struct {
	hostname  string // machine hostname, captured at startup
	configDir string // cached config dir for settings persistence

	registry *pty.SessionRegistry
	backend  pty.SessionBackend
	manager  *relay.HubManager

	mu              sync.RWMutex
	tabNames        map[string]string     // sessionID -> display name
	sessionWorkDirs map[string]string     // sessionID -> EvalSymlinks-resolved absolute WorkDir (Phase 118 / FS-02)
	cliPaths        map[string]string     // cli name -> custom path override
	chatStores      map[string]*ChatStore // Phase 151 / PERSIST-01,PERSIST-03: per-session chat store (guarded by e.mu)
	chatsBaseDir    string                // Phase 151: base directory for chat JSONL files; init from chatsDir() in NewSessionEngine, overrideable via ChatsBaseDirForTest
	aliasStore      *AliasStore           // Phase 152 / IDENT-01,IDENT-02: global alias store (per-person, not per-session); constructed over daemonConfigDir in NewSessionEngine

	startMinimized              bool           // persisted start-minimized preference
	notifyOnWaiting             bool           // Phase 167 NTF-04: persisted native-notification-on-waiting preference (default OFF)
	stayOnHubAfterCreate        bool           // Phase 168 UX-01: persisted "stay on Hub after creating a session" preference (default OFF)
	shellWebShareWarned         bool           // Phase 101 SHELL-08: user has acknowledged the shell web-share security banner
	shellWebShareWarningEnabled *bool          // Phase 150 SET-01: master warning switch; nil = default (true per D-08)
	shellPath                   string         // Phase 107 SHELL-11: user-configured shell binary path; empty = use platform default
	autoCloseSession            *bool          // nil = default (true); persisted pointer
	pluginSettings              PluginSettings // populated by loadSettingsFromDisk via defaults-merge

	// pluginSettingsListener (if non-nil) is invoked synchronously by
	// SetPluginSettings AFTER the new value is persisted, while the engine
	// mutex is still held. Phase 93 PLUG-04 — webserver registers
	// BroadcastPluginConfig here so SSE subscribers receive a frame on every
	// change. The listener MUST be non-blocking (BroadcastPluginConfig uses
	// non-blocking channel sends with drop-on-slow-consumer).
	pluginSettingsListener func()

	// spawnCommand returns the binary and argv every new session runs.
	// Defaults to (GetShellPath(), ["-l"]); tests override via SetSpawnCommandForTest.
	spawnCommand func() (string, []string)

	statusMu        sync.RWMutex
	sessionStatuses map[string]status.SessionStatus // sessionID -> current status
}

// daemonConfigDir returns ~/.config/agenthub/, creating it if needed.
// Mirrors app.go configDir() — internal packages cannot import main.
func daemonConfigDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		base = os.TempDir()
	}
	dir := filepath.Join(base, "agenthub")
	_ = os.MkdirAll(dir, 0700)
	return dir
}

// daemonSettings is the persisted settings structure.
//
// Plugins and SchemaVersion are intentionally NOT tagged `omitempty`:
// the defaults-merge load (Pitfall #14 mitigation) requires the plugins
// block + schemaVersion to always serialize so future loads observe them
// even when every plugin is at its zero value (all-false).
//
// Legacy AgentHub keys that are no longer struct fields (e.g. the removed
// files write default) are ignored by json.Unmarshal and dropped on save.
type daemonSettings struct {
	CLIPaths                    map[string]string `json:"cliPaths,omitempty"`
	StartMinimized              bool              `json:"startMinimized,omitempty"`
	NotifyOnWaiting             bool              `json:"notifyOnWaiting,omitempty"`      // Phase 167 NTF-04: default OFF; zero-value is the correct default (no defaults-merge needed)
	StayOnHubAfterCreate        bool              `json:"stayOnHubAfterCreate,omitempty"` // Phase 168 UX-01: default OFF; zero-value is the correct default (no defaults-merge needed)
	ShellWebShareWarned         bool              `json:"shellWebShareWarned,omitempty"`
	ShellPath                   string            `json:"shellPath,omitempty"`
	AutoCloseSession            *bool             `json:"autoCloseSession,omitempty"`
	ShellWebShareWarningEnabled *bool             `json:"shellWebShareWarningEnabled,omitempty"` // Phase 150 SET-01: *bool so omitempty omits false; nil → default true (D-08)
	Plugins                     PluginSettings    `json:"plugins"`
	SchemaVersion               int               `json:"schemaVersion"`
}

// settingsPath returns the path to settings.json inside the config dir.
func settingsPath(dir string) string {
	return filepath.Join(dir, "settings.json")
}

// knownShells maps basenames of common shell interpreters. A stored CLI path
// whose basename is a shell and whose key is NOT that shell is almost certainly
// stale/wrong (e.g. "claude" → "/bin/sh") and should be discarded on load.
//
// Phase 100-02 (Pitfall 6 mitigation): pwsh/powershell + .exe forms added so
// stale "claude → pwsh.exe" overrides are filtered on load. On Windows
// filepath.Base("/path/to/pwsh.exe") yields "pwsh.exe" (not "pwsh"), so both
// bare and .exe basenames must be present.
var knownShells = map[string]bool{
	"sh": true, "bash": true, "zsh": true, "fish": true,
	"csh": true, "tcsh": true, "dash": true, "ksh": true,
	"pwsh": true, "pwsh.exe": true,
	"powershell": true, "powershell.exe": true,
}

// loadSettingsFromDisk reads settings.json and populates engine state.
// Missing file is not an error (first run).
//
// Defaults-merge: the daemonSettings literal is pre-populated with
// CurrentSchemaVersion + defaultPluginSettings() BEFORE Unmarshal so
// that v3.1 settings.json files (no plugins key, no schemaVersion key)
// round-trip with v3.2 defaults instead of zero-value (all-false) plugins.
// Pitfall #14 mitigation.
func (e *SessionEngine) loadSettingsFromDisk(dir string) {
	data, err := os.ReadFile(settingsPath(dir))
	if err != nil {
		return // file not found or unreadable — not an error
	}
	// Pre-populate defaults BEFORE Unmarshal. Go stdlib leaves missing JSON
	// keys untouched, so v3.1 files (no plugins key) inherit defaultPluginSettings()
	// while v3.2+ files (with explicit plugins block) overwrite the defaults
	// with user choices.
	//
	// SchemaVersion intentionally NOT pre-populated: it must remain 0 for a
	// v3.1 file (no schemaVersion key) so the needsUpgradeWrite check below
	// correctly detects that an upgrade re-write is required.
	s := daemonSettings{
		Plugins: defaultPluginSettings(),
	}
	if json.Unmarshal(data, &s) != nil {
		return
	}
	e.mu.Lock()
	dirty := false
	if s.CLIPaths != nil {
		for k, v := range s.CLIPaths {
			base := filepath.Base(v)
			if knownShells[base] && base != k {
				log.Printf("daemon: dropping stale CLI path override %q=%q (shell mismatch)", k, v)
				delete(s.CLIPaths, k)
				dirty = true
				continue
			}
			e.cliPaths[k] = v
		}
	}
	e.startMinimized = s.StartMinimized
	e.notifyOnWaiting = s.NotifyOnWaiting
	e.stayOnHubAfterCreate = s.StayOnHubAfterCreate
	e.shellWebShareWarned = s.ShellWebShareWarned
	e.shellPath = s.ShellPath
	e.autoCloseSession = s.AutoCloseSession
	e.shellWebShareWarningEnabled = s.ShellWebShareWarningEnabled
	// nil pointer = not set in settings.json = fresh install → default ON (D-08).
	// Unlike shellWebShareWarned (plain bool, default false), this new field defaults
	// to true, so we must use *bool and handle nil explicitly.
	if e.shellWebShareWarningEnabled == nil {
		t := true
		e.shellWebShareWarningEnabled = &t
	}
	e.pluginSettings = s.Plugins
	// Detect upgrade-path: the on-disk schemaVersion was below
	// CurrentSchemaVersion (e.g. v3.1 file with no key → 0). Re-save so
	// the next load observes the populated plugins block + schemaVersion.
	// Idempotent: on second start s.SchemaVersion == CurrentSchemaVersion
	// and this branch does not fire.
	needsUpgradeWrite := s.SchemaVersion < CurrentSchemaVersion
	if dirty || needsUpgradeWrite {
		// Rewrite settings.json without stale entries / with new schema.
		// saveSettingsToDisk runs INSIDE e.mu.Lock() per its contract
		// ("Caller holds e.mu.Lock()."). Mirrors the existing
		// SetStartMinimized pattern.
		e.saveSettingsToDisk()
	}
	e.mu.Unlock()
}

// saveSettingsToDisk writes current settings to settings.json.
// Caller holds e.mu.Lock().
func (e *SessionEngine) saveSettingsToDisk() {
	s := daemonSettings{
		CLIPaths:                    e.cliPaths,
		StartMinimized:              e.startMinimized,
		NotifyOnWaiting:             e.notifyOnWaiting,
		StayOnHubAfterCreate:        e.stayOnHubAfterCreate,
		ShellWebShareWarned:         e.shellWebShareWarned,
		ShellPath:                   e.shellPath,
		AutoCloseSession:            e.autoCloseSession,
		ShellWebShareWarningEnabled: e.shellWebShareWarningEnabled,
		Plugins:                     e.pluginSettings,
		SchemaVersion:               CurrentSchemaVersion,
	}
	data, err := json.Marshal(s)
	if err != nil {
		return
	}
	_ = os.WriteFile(settingsPath(e.configDir), data, 0600)
}

// ConfigDirForTest overrides the engine's configDir field with the supplied
// path. Test-only — internal daemon tests already mutate engine.configDir
// directly (engine_migration_test.go, api_test.go), but external test
// packages (e.g. the Phase 122-05 parity test in package daemon_test) need
// a public setter. Production code must never call this; configDir is
// derived from daemonConfigDir() during NewSessionEngine.
//
// Phase 152: also reconstructs aliasStore over the new dir so tests that
// call ConfigDirForTest get isolated alias persistence (analogous to how
// ChatsBaseDirForTest isolates chat JSONL storage). A failed NewAliasStore
// in the test dir leaves aliasStore nil — test isolation takes precedence
// over the alias feature.
func (e *SessionEngine) ConfigDirForTest(dir string) {
	e.configDir = dir
	if store, err := NewAliasStore(dir); err == nil {
		e.aliasStore = store
	}
}

// ChatsBaseDirForTest overrides the engine's chatsBaseDir so that chat JSONL
// files created during tests land under a t.TempDir() instead of the real
// daemonConfigDir()/chats path. This mirrors ConfigDirForTest (which only
// affects settings persistence and does NOT affect chatsDir()). Test-only;
// production code must never call this.
func (e *SessionEngine) ChatsBaseDirForTest(dir string) {
	e.mu.Lock()
	e.chatsBaseDir = dir
	e.mu.Unlock()
}

// SetSpawnCommandForTest makes every subsequent CreateSession spawn path with
// args instead of the login shell (e.g. "/bin/cat" for a deterministic echo
// process). Test-only; production code must never call this.
func (e *SessionEngine) SetSpawnCommandForTest(path string, args []string) {
	e.spawnCommand = func() (string, []string) { return path, args }
}

// ChatStoreFor returns the ChatStore for the given session ID. It is safe to
// call concurrently with CreateSession and KillSession. Returns ok==false if
// no store exists for the session (either the session does not exist, or
// NewChatStore failed non-fatally during CreateSession).
// Used by the Plan 03 REST endpoints to read/append/export chat messages.
func (e *SessionEngine) ChatStoreFor(sessionID string) (*ChatStore, bool) {
	e.mu.RLock()
	store, ok := e.chatStores[sessionID]
	e.mu.RUnlock()
	return store, ok
}

// NewSessionEngine creates a SessionEngine with all subsystems initialised.
func NewSessionEngine() *SessionEngine {
	hostname, _ := os.Hostname()
	cfgDir := daemonConfigDir()
	// Phase 152 / IDENT-01: construct the global AliasStore. A failed load
	// (e.g. corrupt aliases.json) must not crash daemon startup — tolerate
	// the error here and leave aliasStore nil; identity stamping falls back
	// to the engine hostname as the default alias. Mirror loadSettingsFromDisk:
	// OS failures are non-fatal (first-run scenario).
	aliasStore, _ := NewAliasStore(cfgDir)
	e := &SessionEngine{
		hostname:        hostname,
		configDir:       cfgDir,
		registry:        pty.NewSessionRegistry(),
		backend:         pty.NewNativePTYBackend(),
		manager:         relay.NewHubManager(),
		tabNames:        make(map[string]string),
		sessionWorkDirs: make(map[string]string),
		cliPaths:        make(map[string]string),
		sessionStatuses: make(map[string]status.SessionStatus),
		chatStores:      make(map[string]*ChatStore), // Phase 151 / PERSIST-01
		chatsBaseDir:    chatsDir(),                  // Phase 151: production path = daemonConfigDir()/chats
		aliasStore:      aliasStore,                  // Phase 152 / IDENT-01: global per-person alias persistence
	}
	// -l because go-pty discards Args[0], so the argv0 "-zsh" login trick is impossible.
	e.spawnCommand = func() (string, []string) { return e.GetShellPath(), []string{"-l"} }
	e.loadSettingsFromDisk(cfgDir)
	return e
}

// Aliases returns the engine's global AliasStore. May be nil if AliasStore
// construction failed at startup (disk error). Callers must nil-guard.
// Phase 152 / IDENT-01.
func (e *SessionEngine) Aliases() *AliasStore {
	return e.aliasStore
}

// CreateSession spawns the user's login shell in workDir and returns its ID.
// Empty workDir starts in the home directory.
// onExit is called with (sessionID, exitCode) when the process exits naturally; pass nil if not needed.
func (e *SessionEngine) CreateSession(ctx context.Context, name, workDir string, cols, rows int, onExit func(string, int)) (string, error) {
	// Fixed argv: callers never supply process arguments (T-01-15).
	cliPath, args := e.spawnCommand()

	if workDir == "" {
		// WR-01: validate $HOME against known-useless values. On
		// service-mode daemons (RESEARCH.md Pitfall 4) $HOME is often
		// "/" or "."; writing shell history at those locations would
		// either fail (read-only root) or pollute the filesystem root.
		// If UserHomeDir fails or is unreliable, keep the empty workDir.
		if home, err := os.UserHomeDir(); err == nil && home != "" && home != "/" && home != "." {
			workDir = home
		}
	}

	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}

	sess, err := e.backend.Create(ctx, pty.CreateRequest{
		CLI:     cliPath,
		Args:    args,
		Cols:    cols,
		Rows:    rows,
		WorkDir: workDir,
	})
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}

	e.registry.Add(sess)

	id := sess.ID
	resizeFn := func(cols, rows int) error {
		return e.backend.Resize(id, cols, rows)
	}
	hub := e.manager.Create(id, sess, sess, resizeFn)

	// Phase 118 / FS-02: resolve workDir ONCE here (the Hub groups cards by
	// SessionInfo.WorkDir, GRID-02). EvalSymlinks runs OUTSIDE the lock — it
	// does filesystem I/O — and we cache the result alongside
	// tabNames. Fallback to raw workDir on resolution error so
	// session creation never fails because of resolution.
	resolvedWD := ""
	if workDir != "" {
		if r, err := filepath.EvalSymlinks(workDir); err == nil {
			resolvedWD = r
		} else {
			resolvedWD = workDir
		}
	}

	// Phase 151 / PERSIST-01: construct the ChatStore BEFORE taking e.mu. The
	// constructor does os.MkdirAll plus, on a pre-existing file, a full
	// loadFromDisk scan — holding the engine-wide lock across that filesystem I/O
	// would block every concurrent ChatStoreFor/ListSessions reader (IN-03).
	// Snapshot chatsBaseDir under the lock first so the read stays race-free with
	// ChatsBaseDirForTest (which writes it under e.mu).
	e.mu.Lock()
	chatsBaseDir := e.chatsBaseDir
	e.mu.Unlock()
	// Failure is non-fatal — the session still starts; chat is a side channel.
	// Log only metadata (sessionID + error), never message content (T-151-05).
	chatStore, chatStoreErr := NewChatStore(chatsBaseDir, id)

	e.mu.Lock()
	e.tabNames[id] = name
	e.sessionWorkDirs[id] = resolvedWD
	if chatStoreErr != nil {
		log.Printf("chat: NewChatStore for session %q: %v (chat unavailable for this session)", id, chatStoreErr)
	} else {
		e.chatStores[id] = chatStore
	}
	e.mu.Unlock()

	// Phase 153: wire the inject persist callback so the relay read pump can
	// append SessionInject messages without importing daemon (import-cycle break).
	// Nil-guard: chatStore is nil when NewChatStore failed (non-fatal; PTY write still occurs).
	if chatStore != nil {
		hub.SetChatAppendFn(func(msg relay.ChatMessage) (relay.ChatMessage, error) {
			return chatStore.AppendMessage(msg)
		})
	}

	// Watch for natural process exit (PTY EOF -> hub.Done closes).
	// Transitions session state to StateStopped and captures exit code.
	// Calls onExit callback if provided (used by API layer for web grace period per D-12).
	//
	// Does NOT call cmd.Wait() — killSession may be running concurrently and
	// go-pty's cmd.Wait() is not safe for concurrent callers. Instead, reads
	// the cached exit code from ExitCode() which checks ProcessState under mutex.
	go func() {
		<-hub.Done() // blocks until PTY read loop returns EOF (D-07)
		if sess.IsKilled() {
			return // killSession handles state transition
		}
		// Natural exit: reap the child and cache its real exit code. On
		// macOS/BSD this is the ONLY thing that populates ProcessState —
		// go-pty has no unix waitOnContext goroutine and these platforms run
		// no exit detector, so without this the code is lost as a cached -1
		// and normalized to 0 below (making CARD-08 stopped-err unreachable).
		// On Linux/Windows ReapNaturalExit is a no-op: the platform exit
		// detector already cached the code before Hub.Done fired.
		sess.ReapNaturalExit()
		// Cancel the context to release exec.Cmd's context goroutine (the
		// child is already reaped, so its Kill is a harmless no-op).
		sess.CancelContext()
		exitCode := sess.ExitCode()
		if exitCode == -1 {
			exitCode = 0 // conservative default per D-10 (signal-terminated / unknown)
		}
		sess.SetState(pty.StateStopped)
		if onExit != nil {
			onExit(id, exitCode)
		}
	}()

	return id, nil
}

// ListSessions returns a snapshot of all registered sessions.
func (e *SessionEngine) ListSessions() []SessionInfo {
	sessions := e.registry.List()
	result := make([]SessionInfo, 0, len(sessions))

	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, s := range sessions {
		state := "running"
		if s.GetState() == pty.StateStopped {
			state = "stopped"
		}
		name := e.tabNames[s.ID]

		// FIX-04 (#121): populate viewer count from RemoteViewerCount, NOT
		// SubscriberCount — the Hub card must reflect only real remote/shared
		// viewers (Origin=="web"), excluding the app's own internal Origin=="local"
		// subscribers (TerminalPanel, ChatPanel, status watcher, Hub-card preview),
		// or a never-shared local session shows phantom viewers.
		// manager.Get acquires HubManager.mu; RemoteViewerCount acquires hub.mu.
		// Both are safe to call while holding e.mu.RLock (no lock ordering conflict).
		viewerCount := 0
		if hub, ok := e.manager.Get(s.ID); ok {
			viewerCount = hub.RemoteViewerCount()
		}

		// Heuristic status from detector (running/idle/waiting/errored).
		heuristicStatus := string(status.StatusRunning) // conservative default
		e.statusMu.RLock()
		if hs, ok := e.sessionStatuses[s.ID]; ok {
			heuristicStatus = string(hs)
		}
		e.statusMu.RUnlock()

		var exitCodePtr *int
		var durationPtr *int
		if state == "stopped" && !s.IsKilled() {
			// Only read exit code for natural exits. Killed sessions have
			// cmd.Wait() still running in killSession — reading ProcessState
			// would race with go-pty's internal write.
			ec := s.ExitCode()
			if ec == -1 {
				ec = 0 // SHELL-12: mirror the natural-exit goroutine's -1→0 normalization so GUI consumers never see PTY-EOF as an error code.
			}
			exitCodePtr = &ec
			dur := int(time.Since(s.CreatedAt).Seconds())
			durationPtr = &dur
		}

		result = append(result, SessionInfo{
			ID:          s.ID,
			CLI:         s.CLI,
			Name:        name,
			State:       state,
			Status:      heuristicStatus,
			CreatedAt:   s.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			Hostname:    e.hostname,
			ViewerCount: viewerCount,
			ExitCode:    exitCodePtr,
			Duration:    durationPtr,
			// Phase 131 / GRID-02: e.mu.RLock is already held via defer at the top
			// of this function — map read is safe without acquiring a new lock.
			WorkDir: e.sessionWorkDirs[s.ID],
		})
	}
	return result
}

// RenameSession updates the display name of a session.
func (e *SessionEngine) RenameSession(id, name string) error {
	if _, ok := e.registry.Get(id); !ok {
		return fmt.Errorf("session %q not found", id)
	}
	e.mu.Lock()
	e.tabNames[id] = name
	e.mu.Unlock()
	return nil
}

// KillSession terminates the session and removes it from all registries.
func (e *SessionEngine) KillSession(id string) error {
	if err := e.backend.Kill(id); err != nil {
		return fmt.Errorf("kill session: %w", err)
	}
	e.manager.Remove(id)
	e.registry.Remove(id)

	e.mu.Lock()
	delete(e.tabNames, id)
	delete(e.sessionWorkDirs, id) // Phase 118 / FS-02
	// Phase 151 / PERSIST-03: delete the ChatStore under e.mu so a recycled
	// session ID never inherits a stale thread (T-151-07 mitigation) and no
	// orphaned JSONL file remains (T-151-06 mitigation).
	if store, ok := e.chatStores[id]; ok {
		if delErr := store.Delete(); delErr != nil {
			log.Printf("chat: Delete for session %q: %v", id, delErr)
		}
		delete(e.chatStores, id)
	}
	e.mu.Unlock()

	e.statusMu.Lock()
	delete(e.sessionStatuses, id)
	e.statusMu.Unlock()

	return nil
}

// GetSessionStatus returns the current heuristic status of the session.
// Returns "running" if the session is not found (conservative default).
func (e *SessionEngine) GetSessionStatus(sessionID string) string {
	e.statusMu.RLock()
	s, ok := e.sessionStatuses[sessionID]
	e.statusMu.RUnlock()
	if !ok {
		return string(status.StatusRunning)
	}
	return string(s)
}

// ansiEscape matches CSI sequences (e.g. \x1b[32m) and OSC sequences
// (e.g. \x1b]0;title\x07 or \x1b]8;;url\x1b\).
// Covers the ANSI vocabulary emitted by common terminal programs.
// Pattern mirrors frontend/src/lib/stripAnsi.ts extended to include OSC.
//
// CR-01 fix: OSC sequences come in two forms:
//
//	BEL-terminated:  ESC ] <body> BEL         — [^\x07\x1b]*\x07
//	ST-terminated:   ESC ] <body> ESC \        — [^\x1b]*\x1b\\
//
// The original [^\x07\x1b]* stopped at the first \x1b in both branches,
// leaving the \x5c (backslash) of the ST terminator as a literal character.
// The fix uses two separate OSC branches: the BEL branch excludes both BEL and
// ESC from the body; the ST branch only excludes ESC (BEL in the body is
// consumed by the BEL branch first via alternation, so no exclusion needed).
var ansiEscape = regexp.MustCompile(
	`\x1b(?:` +
		`\[[0-9;?]*[a-zA-Z]` + // CSI sequences (e.g. \x1b[32m color codes)
		`|\][^\x07\x1b]*\x07` + // OSC terminated by BEL  (e.g. \x1b]0;title\x07)
		`|\][^\x1b]*\x1b\\` + // OSC terminated by ST   (e.g. \x1b]8;;url\x1b\)
		`)`,
)

// queryStripPattern matches terminal-query and in-band-resize escape sequences
// that elicit a blocking response write into charmbracelet/x/vt's unbuffered
// response pipe (Emulator.pw). Stripping these BEFORE emu.Write lets Write run
// synchronously with no drain goroutine — which also removes the concurrent
// Read/Close data race (#100). SGR/color sequences are deliberately preserved
// so the styled grid keeps its colors and bold attributes.
//
// Covered sequences:
//   - DA1  (ESC[c / ESC[0c)       — Primary Device Attributes
//   - DA2  (ESC[>c)               — Secondary Device Attributes
//   - DSR  (ESC[5n / ESC[6n)      — Device Status Report / Cursor Position
//   - DECXCPR (ESC[?6n)           — Extended Cursor Position Report
//   - DECRQM ANSI (ESC[...$p)     — Request ANSI Mode
//   - DECRQM DEC  (ESC[?...$p)    — Request DEC Mode
//   - OSC color queries (ESC]10|11|12;?) — Foreground/Background/Cursor color query
//   - DEC mode 2048 set/reset (ESC[?2048h/l) — In-band resize (fires a pw write on set)
//
// NOT stripped: SGR (color/bold/etc.), cursor movement, erase, scroll — these are
// rendering sequences and must reach the emulator to produce correct styled output.
var queryStripPattern = regexp.MustCompile(
	`\x1b\[[0-9;]*c` + // DA1 (and any params before c)
		`|\x1b\[>[0-9;]*c` + // DA2
		`|\x1b\[[0-9;]*n` + // DSR (5n/6n and any CSI n)
		`|\x1b\[\?[0-9;]*n` + // DECXCPR (ESC[?...n)
		`|\x1b\[[0-9;]*\$p` + // DECRQM ANSI
		`|\x1b\[\?[0-9;]*\$p` + // DECRQM DEC
		`|\x1b\[\?2048[hl]` + // in-band resize set/reset (set triggers a pw write)
		`|\x1b\]1[012];\?(?:\x07|\x1b\\)`, // OSC 10/11/12 color query, BEL- or ST-terminated
)

// GetSessionTailLines returns the last n plain-text lines from the session's
// scrollback buffer. Relay framing bytes (0x01 / relay.MsgOutput) and ANSI/OSC
// escape sequences are stripped before splitting on newlines. Trailing empty
// lines are trimmed. Returns []string{} (never nil) if the session has no hub.
// IN-01: returns empty slice (not nil) to avoid forcing callers to nil-guard.
// Phase 132 / CARD-07.
func (e *SessionEngine) GetSessionTailLines(id string, n int) []string {
	hub, ok := e.manager.Get(id)
	if !ok {
		return []string{} // IN-01: defensive — never nil; callers need not nil-guard
	}
	raw := hub.ScrollbackSnapshot()
	// Strip relay.MsgOutput (0x01) framing bytes — pattern from engine_test.go lines 463-471.
	stripped := make([]byte, 0, len(raw))
	for _, b := range raw {
		if b != relay.MsgOutput {
			stripped = append(stripped, b)
		}
	}
	// Strip ANSI escape sequences.
	text := ansiEscape.ReplaceAllString(string(stripped), "")
	lines := strings.Split(text, "\n")
	// Remove empty trailing lines.
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}

// colorToHex converts an image/color.Color to a string suitable for the
// StyledSpan wire type:
//   - nil → "" (terminal default)
//   - ansi.BasicColor (ANSI 0-15) → "ansi:N"
//   - ansi.IndexedColor (ANSI 16-255) → "ansi:N"
//   - all others → "#rrggbb" (true-color hex)
//
// Phase 139 / CARD-05.
func colorToHex(c color.Color) string {
	if c == nil {
		return ""
	}
	switch v := c.(type) {
	case ansi.BasicColor:
		return fmt.Sprintf("ansi:%d", int(v))
	case ansi.IndexedColor:
		return fmt.Sprintf("ansi:%d", int(v))
	default:
		r, g, b, _ := c.RGBA()
		return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
	}
}

// GetSessionStyledTailLines returns the last n styled-cell lines from the
// session's scrollback buffer. Relay framing bytes (0x01 / relay.MsgOutput)
// are stripped, then the raw bytes are fed into a headless charmbracelet/x/vt
// emulator that produces a per-cell styled grid. Returns [][]StyledSpan{}
// (never nil) if the session has no hub.
//
// The emulator column width is taken from Hub.Cols() (the PTY's actual
// terminal width, or 220 if no resize has been applied) to avoid spurious
// line-wrapping artifacts from Pitfall 1.
//
// Phase 139 / CARD-05.
func (e *SessionEngine) GetSessionStyledTailLines(id string, n int) [][]StyledSpan {
	hub, ok := e.manager.Get(id)
	if !ok {
		return [][]StyledSpan{} // IN-01: defensive — never nil
	}
	raw := hub.ScrollbackSnapshot()

	// Strip relay.MsgOutput (0x01) framing bytes — same as GetSessionTailLines.
	stripped := make([]byte, 0, len(raw))
	for _, b := range raw {
		if b != relay.MsgOutput {
			stripped = append(stripped, b)
		}
	}

	// Feed into headless VT emulator using the real PTY column width.
	// 50 rows is enough to hold the tail we need; only the last n rows
	// from the active screen are returned after blank-row trimming.
	const emuRows = 50
	cols := hub.Cols()
	emu := xvt.NewEmulator(cols, emuRows)

	// Strip terminal-query and in-band-resize sequences that would block
	// emu.Write on the emulator's unbuffered response pipe (#96). With these
	// removed, Write returns synchronously and no drain goroutine is needed —
	// which also removes the concurrent Read/Close data race (#100).
	// SGR/color sequences are preserved so the styled grid renders correctly.
	clean := queryStripPattern.ReplaceAll(stripped, nil)
	emu.Write(clean) //nolint:errcheck // emulator Write never returns a meaningful error
	_ = emu.Close()

	// Extract the active screen rows as StyledSpan slices.
	var rows [][]StyledSpan
	for y := 0; y < emuRows; y++ {
		var row []StyledSpan
		for x := 0; x < cols; x++ {
			cell := emu.CellAt(x, y)
			if cell == nil {
				break
			}
			row = append(row, StyledSpan{
				Char: cell.Content,
				Bold: cell.Style.Attrs&uv.AttrBold != 0,
				FG:   colorToHex(cell.Style.Fg),
				BG:   colorToHex(cell.Style.Bg),
			})
		}
		rows = append(rows, row)
	}

	// Trim trailing blank rows (rows where all cells are whitespace or empty).
	for len(rows) > 0 {
		last := rows[len(rows)-1]
		var text string
		for _, span := range last {
			text += span.Char
		}
		if strings.TrimSpace(text) == "" {
			rows = rows[:len(rows)-1]
		} else {
			break
		}
	}

	// Return last n rows.
	if len(rows) > n {
		rows = rows[len(rows)-n:]
	}
	if rows == nil {
		return [][]StyledSpan{} // IN-01: ensure non-nil
	}
	return rows
}

// GetSessionWorkDir returns the EvalSymlinks-resolved absolute WorkDir for
// a session, or empty string if the session is unknown. Phase 118 / FS-02.
func (e *SessionEngine) GetSessionWorkDir(id string) string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.sessionWorkDirs[id]
}

// ResolveCLI returns the executable path for the named CLI.
// Checks custom overrides first, then returns the name as-is.
func (e *SessionEngine) ResolveCLI(name string) string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if path, ok := e.cliPaths[name]; ok {
		return path
	}
	return name
}

// DiscoverShells is a thin engine-mediated wrapper around pty.DiscoverShells.
// IN-03 / PATTERNS.md line 508: route the GET /shells handler through the
// engine so HTTP tests can DI a fake without coupling to the filesystem,
// and so future engine-level caching or rate-limiting can be applied
// without restructuring callers. The wrapper is currently a passthrough.
func (e *SessionEngine) DiscoverShells() []pty.DetectedShell {
	return pty.DiscoverShells()
}

// UpdateCLIPath stores a custom executable path for the named CLI.
// The path must exist on disk. Persists to settings.json.
func (e *SessionEngine) UpdateCLIPath(name, path string) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("custom CLI path %q: %w", path, err)
	}
	e.mu.Lock()
	e.cliPaths[name] = path
	e.saveSettingsToDisk()
	e.mu.Unlock()
	return nil
}

// GetCLIPaths returns a copy of the current CLI path overrides map.
func (e *SessionEngine) GetCLIPaths() map[string]string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make(map[string]string, len(e.cliPaths))
	for k, v := range e.cliPaths {
		out[k] = v
	}
	return out
}

// GetStartMinimized returns the persisted start-minimized preference.
func (e *SessionEngine) GetStartMinimized() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.startMinimized
}

// SetStartMinimized updates and persists the start-minimized preference.
func (e *SessionEngine) SetStartMinimized(val bool) {
	e.mu.Lock()
	e.startMinimized = val
	e.saveSettingsToDisk()
	e.mu.Unlock()
}

// GetNotifyOnWaiting returns the persisted native-notification-on-waiting
// preference. Phase 167 NTF-04; default OFF (zero-value false).
func (e *SessionEngine) GetNotifyOnWaiting() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.notifyOnWaiting
}

// SetNotifyOnWaiting updates and persists the native-notification-on-waiting
// preference. Phase 167 NTF-04.
func (e *SessionEngine) SetNotifyOnWaiting(val bool) {
	e.mu.Lock()
	e.notifyOnWaiting = val
	e.saveSettingsToDisk()
	e.mu.Unlock()
}

// GetStayOnHubAfterCreate returns the persisted "stay on Hub after creating
// a session" preference. Phase 168 UX-01; default OFF (zero-value false).
func (e *SessionEngine) GetStayOnHubAfterCreate() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.stayOnHubAfterCreate
}

// SetStayOnHubAfterCreate updates and persists the "stay on Hub after
// creating a session" preference. Phase 168 UX-01.
func (e *SessionEngine) SetStayOnHubAfterCreate(val bool) {
	e.mu.Lock()
	e.stayOnHubAfterCreate = val
	e.saveSettingsToDisk()
	e.mu.Unlock()
}

// GetShellWebShareWarned returns the persisted "user has acknowledged the
// shell web-share security banner" flag. Phase 101 SHELL-08.
func (e *SessionEngine) GetShellWebShareWarned() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.shellWebShareWarned
}

// SetShellWebShareWarned persists the shell-web-share-warned flag.
// Returns an error path for symmetry with future-proofed callers; the
// underlying saveSettingsToDisk swallows IO errors today, so this currently
// always returns nil.
func (e *SessionEngine) SetShellWebShareWarned(val bool) error {
	e.mu.Lock()
	e.shellWebShareWarned = val
	e.saveSettingsToDisk()
	e.mu.Unlock()
	return nil
}

// GetShellWebShareWarningEnabled returns the warning-enabled master switch.
// Returns true when not set (nil pointer = default ON per D-08). Phase 150 SET-01.
func (e *SessionEngine) GetShellWebShareWarningEnabled() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.shellWebShareWarningEnabled == nil {
		return true
	}
	return *e.shellWebShareWarningEnabled
}

// SetShellWebShareWarningEnabled persists the warning-enabled master switch.
// D-03 re-arm: when enabling (val=true), atomically resets shellWebShareWarned
// to false so the one-time banner shows again on the next shell web-share.
// Both fields are written in a single saveSettingsToDisk call (atomic).
// Phase 150 SET-01.
func (e *SessionEngine) SetShellWebShareWarningEnabled(val bool) error {
	e.mu.Lock()
	e.shellWebShareWarningEnabled = &val
	if val {
		// Re-arm (D-03): reset the one-time acknowledged flag.
		e.shellWebShareWarned = false
	}
	e.saveSettingsToDisk()
	e.mu.Unlock()
	return nil
}

// resolveDefaultShellPath returns the shell to use when no shellPath setting
// is configured: $SHELL when it is an absolute path to an existing,
// executable, non-directory file; otherwise /bin/zsh (the macOS default).
// NEVER returns an empty string. GetShellPath (display) and the spawn command
// both go through here, so the shell shown is the shell that runs.
func resolveDefaultShellPath() string {
	if s := os.Getenv("SHELL"); filepath.IsAbs(s) {
		if info, err := os.Stat(s); err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
			return s
		}
	}
	return "/bin/zsh"
}

// GetShellPath returns the persisted shell binary path. When no path has been
// set by the user (shellPath is empty), it resolves and returns the platform
// default via resolveDefaultShellPath(). NEVER returns an empty string.
// Phase 107 SHELL-11.
func (e *SessionEngine) GetShellPath() string {
	e.mu.RLock()
	path := e.shellPath
	e.mu.RUnlock()
	if path == "" {
		return resolveDefaultShellPath()
	}
	return path
}

// SetShellPath updates and persists the shell binary path override.
// When path is empty, the override is cleared (subsequent GetShellPath calls
// will return the platform default). When path is non-empty, it must point to
// an existing executable file; otherwise an error is returned and the field is
// left unchanged. Phase 107 SHELL-11.
func (e *SessionEngine) SetShellPath(path string) error {
	if path != "" {
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("path %q does not exist or is not executable", path)
		}
		if info.IsDir() {
			return fmt.Errorf("path %q is a directory, not an executable", path)
		}
		if info.Mode()&0111 == 0 {
			return fmt.Errorf("path %q does not exist or is not executable", path)
		}
	}
	e.mu.Lock()
	e.shellPath = path
	e.saveSettingsToDisk()
	e.mu.Unlock()
	return nil
}

// GetAutoCloseSession returns the auto-close-on-exit preference.
// Returns true when the setting is absent (nil pointer = default enabled per D-11).
func (e *SessionEngine) GetAutoCloseSession() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.autoCloseSession == nil {
		return true // default: enabled
	}
	return *e.autoCloseSession
}

// SetAutoCloseSession updates and persists the auto-close-on-exit preference.
func (e *SessionEngine) SetAutoCloseSession(val bool) {
	e.mu.Lock()
	e.autoCloseSession = &val
	e.saveSettingsToDisk()
	e.mu.Unlock()
}

// GetPluginSettings returns the current plugin enable/disable preferences.
func (e *SessionEngine) GetPluginSettings() PluginSettings {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.pluginSettings
}

// SetPluginSettings updates and persists the plugin enable/disable preferences.
// Settings are immediately written to disk while the engine mutex is held
// (saveSettingsToDisk's contract requires the caller to hold e.mu.Lock()).
//
// Phase 93 PLUG-04: after persistence, invoke pluginSettingsListener (if set)
// outside the engine mutex so a slow listener cannot deadlock with concurrent
// engine operations. The listener is the SSE BroadcastPluginConfig hook.
func (e *SessionEngine) SetPluginSettings(s PluginSettings) {
	e.mu.Lock()
	e.pluginSettings = s
	e.saveSettingsToDisk()
	listener := e.pluginSettingsListener
	e.mu.Unlock()
	if listener != nil {
		listener()
	}
}

// SetSearchConfig updates and persists ONLY the SearchConfig sub-key of
// PluginSettings, leaving the rest of PluginSettings (WebGL, Unicode11,
// Search, WebLinks, Image, Serialize, Clipboard, Progress) untouched.
//
// Phase 94-07 WR-03 (gap closure) — handleSearchOptionsChange used to call
// SetPluginSettings with a full PluginSettings constructed from the
// App-level prop, racing PluginsSection's stale local edit buffer.
// SetSearchConfig is the surgical sub-key writer that preserves
// PluginsSection's edit buffer semantics: a find-bar toggle change can no
// longer overwrite an in-flight Plugins-tab boolean edit.
//
// Concurrency / persistence contract is identical to SetPluginSettings
// (mutate under e.mu.Lock(), saveSettingsToDisk while held, capture and
// invoke listener after release). The Phase 93 PLUG-04 SSE
// pluginSettingsListener is invoked so /api/plugin-config/stream
// subscribers (web terminal) receive a frame on every search-option
// change — preserving SRC-05 web parity (94-04 behavior unchanged).
func (e *SessionEngine) SetSearchConfig(cfg SearchConfig) {
	e.mu.Lock()
	e.pluginSettings.SearchConfig = cfg
	e.saveSettingsToDisk()
	listener := e.pluginSettingsListener
	e.mu.Unlock()
	if listener != nil {
		listener()
	}
}

// SetWebLinksConfig updates and persists ONLY the WebLinksConfig sub-key
// of PluginSettings, leaving the rest of PluginSettings (WebGL, Unicode11,
// Search, SearchConfig, WebLinks boolean, Image, Serialize, Clipboard,
// Progress) untouched.
//
// Phase 95 LNK-05 / LNK-06 — mirrors Phase 94 Plan 07's SetSearchConfig
// sub-key writer verbatim. Concurrency / persistence contract is identical
// to SetPluginSettings: mutate under e.mu.Lock(), saveSettingsToDisk while
// held, capture and invoke listener after release. The Phase 93 PLUG-04
// pluginSettingsListener is invoked so /api/plugin-config/stream
// subscribers (web terminal) receive a frame on every WebLinksConfig
// change — preserving live-toggle web parity (Plan 95-06 wires the
// SSE-driven hot-swap arm in terminal.js).
//
// The sub-key writer is callable from v3.2 (this plan ships the path) but
// the in-app UI for editing the sub-fields ships in Phase 99 / PUI-03;
// until then the boolean WebLinks toggle in PluginsSection routes through
// the full SetPluginSettings path.
func (e *SessionEngine) SetWebLinksConfig(cfg WebLinksConfig) {
	e.mu.Lock()
	e.pluginSettings.WebLinksConfig = cfg
	e.saveSettingsToDisk()
	listener := e.pluginSettingsListener
	e.mu.Unlock()
	if listener != nil {
		listener()
	}
}

// SetImageConfig updates and persists ONLY the ImageConfig sub-key of
// PluginSettings, leaving the rest of PluginSettings (WebGL, Unicode11,
// Search, SearchConfig, WebLinks, WebLinksConfig, Image bool, Serialize,
// Clipboard, Progress) untouched.
//
// Phase 96 IMG-02 — mirrors Phase 95 SetWebLinksConfig and Phase 94-07
// SetSearchConfig sub-key writers verbatim. Concurrency contract:
// mutate under e.mu.Lock(); saveSettingsToDisk while held; capture
// listener; release lock; invoke listener after release (avoids
// re-entrancy deadlock if listener calls back into the engine).
//
// Note on next-session-only semantics: the listener fires (so web SSE
// consumers receive the frame and downstream UIs can update their
// internal pluginConfig prop), but the desktop TerminalPanel hot-swap
// useEffect intentionally does NOT include `imageConfig` in its dep
// array — only newly-mounted sessions pick up the new StorageLimit
// (per ROADMAP IMG-01 italic caption affordance).
func (e *SessionEngine) SetImageConfig(cfg ImageConfig) {
	e.mu.Lock()
	e.pluginSettings.ImageConfig = cfg
	e.saveSettingsToDisk()
	listener := e.pluginSettingsListener
	e.mu.Unlock()
	if listener != nil {
		listener()
	}
}

// SetPluginSettingsListener registers a callback invoked synchronously by
// SetPluginSettings AFTER the new value is persisted. Phase 93 PLUG-04 push
// channel — webserver registers BroadcastPluginConfig here so SSE subscribers
// receive a frame on every change.
//
// Single-listener slot: the two NewWebServer call sites in api.go are mutually
// exclusive at runtime (one for AutoStartWebServer / Tailscale-mode, one for
// handleWebServerStart / mode-switch), so a single slot is safe.
func (e *SessionEngine) SetPluginSettingsListener(fn func()) {
	e.mu.Lock()
	e.pluginSettingsListener = fn
	e.mu.Unlock()
}

// Manager returns the HubManager (needed by webserver and relay server).
func (e *SessionEngine) Manager() *relay.HubManager {
	return e.manager
}

// Registry returns the session registry (needed for Get lookups).
func (e *SessionEngine) Registry() *pty.SessionRegistry {
	return e.registry
}

// Backend returns the session backend (needed for resize in relay).
func (e *SessionEngine) Backend() pty.SessionBackend {
	return e.backend
}
