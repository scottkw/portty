package desktop

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sort"
	"sync"
	"time"
)

// Admission limits (CONTEXT Claude's Discretion and After research). They stop
// prompt spam and squatting: one person with several devices cannot fill the
// share's slots, and a stale prompt cannot wait forever.
const (
	admitTimeout       = 5 * time.Minute
	maxPendingPerShare = 8
	maxPendingPerUser  = 2

	// Knocks per user: a burst of knockBurst, then one per knockEvery. A
	// re-knock costs one socket and a hello, and each fires the Admit sheet,
	// the window and two audit lines, so it must not be free (WR-02).
	knockBurst = 3
	knockEvery = 2 * time.Second
)

// Errors of the admission queue.
var (
	ErrNoRequest  = errors.New("desktop: no such admission request") // unknown, answered or ended
	ErrShareFull  = errors.New("desktop: too many pending admission requests")
	ErrDenied     = errors.New("desktop: denied for this share")
	ErrShareEnded = errors.New("desktop: the guest share has ended")
)

// AdmissionKey is who an admission belongs to within one share: the numeric
// user ID and the stable node ID. A reconnect from the same key is re-admitted;
// another device of the same person is a new key and waits at Admit.
type AdmissionKey struct {
	UserID   int64
	StableID string
}

// Result is how an admission request ended.
type Result string

// The results of a request. ResultWithdrawn is the guest's own socket going
// away while it waited.
const (
	ResultAdmitted  Result = "admitted"
	ResultDenied    Result = "denied"
	ResultTimeout   Result = "timeout"
	ResultReplaced  Result = "replaced"
	ResultEnded     Result = "share_ended"
	ResultWithdrawn Result = "withdrawn"
)

// Outcome is a request's answer. Grants carries the stored control and
// clipboard bits of an admission that is being re-used.
type Outcome struct {
	Result Result
	Grants Rights
}

// Request is one guest waiting at Admit. The public fields are what the sharer
// sees; Seq orders the queue (arrival order, not clock order).
type Request struct {
	ID    string
	Ident Identity
	Seq   uint64
	Since time.Time

	done chan struct{} // closed once out is set
	out  Outcome       // set under GuestShare.mu before done closes
}

// GuestEvent is one admission event for the Notify hook: Kind is an Audit*
// event name (admit_request, admitted, denied, revoked) and Reason a Reason*
// constant. Ident is already sanitized.
type GuestEvent struct {
	Kind      string
	RequestID string
	Reason    string
	Ident     Identity
}

type liveViewer struct {
	id      ViewerID
	closeFn func()
}

type admission struct {
	grants Rights
	live   *liveViewer
}

// timerFn starts a timer; tests replace it so no test sleeps.
type timerFn func(d time.Duration) (c <-chan time.Time, stop func())

// GuestShare is one share's admission state, in daemon memory only: it dies
// with the share and is never written anywhere (CONTEXT [Grants]).
type GuestShare struct {
	// ID is the 128-bit share ID; admissions and the audit log are keyed by it.
	ID string

	now    func() time.Time
	notify func(GuestEvent)
	timer  timerFn

	mu        sync.Mutex
	seq       uint64
	pending   map[string]*Request
	denied    map[int64]struct{}
	knockAt   map[int64]time.Time // per user: when the knock bucket is next full (knockAllowedLocked)
	admitted  map[AdmissionKey]*admission
	ended     bool
	endReason string
	endedCh   chan struct{}
}

func randHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// NewGuestShare starts an empty share. now is the clock (nil is time.Now) and
// notify receives every admission event (nil drops them); it is called outside
// the share's lock.
func NewGuestShare(now func() time.Time, notify func(GuestEvent)) (*GuestShare, error) {
	id, err := randHex(16)
	if err != nil {
		return nil, err
	}
	if now == nil {
		now = time.Now
	}
	return &GuestShare{
		ID: id, now: now, notify: notify,
		timer: func(d time.Duration) (<-chan time.Time, func()) {
			t := time.NewTimer(d)
			return t.C, func() { t.Stop() }
		},
		pending:  make(map[string]*Request),
		denied:   make(map[int64]struct{}),
		knockAt:  make(map[int64]time.Time),
		admitted: make(map[AdmissionKey]*admission),
		endedCh:  make(chan struct{}),
	}, nil
}

func (g *GuestShare) fire(evs []GuestEvent) {
	if g.notify == nil {
		return
	}
	for _, e := range evs {
		g.notify(e)
	}
}

// emit sends one event that did not come from the queue (a media identity
// mismatch). Safe on a nil share.
func (g *GuestShare) emit(e GuestEvent) {
	if g != nil {
		g.fire([]GuestEvent{e})
	}
}

// resolveLocked answers r once; it reports false when r was already answered.
func (g *GuestShare) resolveLocked(r *Request, out Outcome) bool {
	if g.pending[r.ID] != r {
		return false
	}
	delete(g.pending, r.ID)
	r.out = out
	close(r.done)
	return true
}

func (r *Request) event(kind, reason string) GuestEvent {
	return GuestEvent{Kind: kind, RequestID: r.ID, Reason: reason, Ident: r.Ident}
}

// Knock registers a guest at Admit. A key that was admitted before gets an
// already-answered request carrying its stored grants, and its older live
// connection is closed (one grant never serves two connections). A user denied
// earlier is refused at once. The limits are applied here: a user knocking
// faster than the knock rate is refused with ErrShareFull and no event, a newer
// knock from the same (user, node) replaces the older, a third from one user
// replaces that user's oldest, and a ninth in the share is refused with
// ErrShareFull before anything is replaced.
func (g *GuestShare) Knock(id Identity) (*Request, error) {
	rid, err := randHex(8)
	if err != nil {
		return nil, err
	}
	key := AdmissionKey{UserID: id.UserID, StableID: id.StableID}

	g.mu.Lock()
	if g.ended {
		g.mu.Unlock()
		return nil, ErrShareEnded
	}
	if _, ok := g.denied[id.UserID]; ok {
		g.mu.Unlock()
		return nil, ErrDenied
	}
	if !g.knockAllowedLocked(id.UserID) {
		g.mu.Unlock()
		return nil, ErrShareFull
	}
	g.seq++
	r := &Request{ID: rid, Ident: id, Seq: g.seq, Since: g.now(), done: make(chan struct{})}

	if a, ok := g.admitted[key]; ok {
		r.out = Outcome{Result: ResultAdmitted, Grants: a.grants}
		close(r.done)
		old := a.live
		a.live = nil
		g.mu.Unlock()
		if old != nil {
			old.closeFn()
		}
		return r, nil
	}

	var victims, others []*Request
	for _, p := range g.pending {
		if p.Ident.UserID != id.UserID {
			continue
		}
		if p.Ident.StableID == id.StableID {
			victims = append(victims, p)
		} else {
			others = append(others, p)
		}
	}
	sort.Slice(others, func(i, j int) bool { return others[i].Seq < others[j].Seq })
	if len(others) >= maxPendingPerUser {
		victims = append(victims, others[0])
	}
	if len(g.pending)-len(victims) >= maxPendingPerShare {
		g.mu.Unlock()
		return nil, ErrShareFull
	}
	var evs []GuestEvent
	for _, p := range victims {
		if g.resolveLocked(p, Outcome{Result: ResultReplaced}) {
			evs = append(evs, p.event(AuditDenied, ReasonReplaced))
		}
	}
	g.pending[r.ID] = r
	evs = append(evs, r.event(AuditAdmitRequest, ""))
	g.mu.Unlock()
	g.fire(evs)
	return r, nil
}

// knockAllowedLocked is a per-user token bucket kept as one time (GCRA): the
// bucket is full again at knockAt, and a knock is allowed while that is at most
// knockBurst-1 intervals ahead of now.
func (g *GuestShare) knockAllowedLocked(userID int64) bool {
	now := g.now()
	at := g.knockAt[userID]
	if at.Before(now) {
		at = now
	}
	if at.Sub(now) > (knockBurst-1)*knockEvery {
		return false
	}
	g.knockAt[userID] = at.Add(knockEvery)
	return true
}

// Await blocks until r is answered, times out (5 minutes from the knock, by
// the share's clock) or ctx ends; the bool is false only for ctx. It
// subscribes to the answer first and the answer is a closed channel, so one
// that arrives before the wait begins is not missed.
func (g *GuestShare) Await(ctx context.Context, r *Request) (Outcome, bool) {
	var tc <-chan time.Time
	if left := admitTimeout - g.now().Sub(r.Since); left <= 0 {
		g.expire(r)
	} else {
		c, stop := g.timer(left)
		defer stop()
		tc = c
	}
	select {
	case <-r.done:
	case <-tc:
		g.expire(r)
	case <-ctx.Done():
		return Outcome{}, false
	}
	<-r.done
	return r.out, true
}

func (g *GuestShare) expire(r *Request) {
	g.mu.Lock()
	var evs []GuestEvent
	if g.resolveLocked(r, Outcome{Result: ResultTimeout}) {
		evs = append(evs, r.event(AuditDenied, ReasonTimeout))
	}
	g.mu.Unlock()
	g.fire(evs)
}

// Withdraw drops r because its socket went away while it waited. A request that
// was already answered is left as it is.
func (g *GuestShare) Withdraw(r *Request) {
	g.mu.Lock()
	var evs []GuestEvent
	if g.resolveLocked(r, Outcome{Result: ResultWithdrawn}) {
		evs = append(evs, r.event(AuditDenied, ReasonViewerClosed))
	}
	g.mu.Unlock()
	g.fire(evs)
}

// Admit lets the waiting guest in. The admission is created with no grants.
func (g *GuestShare) Admit(reqID string) error {
	g.mu.Lock()
	r, ok := g.pending[reqID]
	if !ok {
		g.mu.Unlock()
		return ErrNoRequest
	}
	key := AdmissionKey{UserID: r.Ident.UserID, StableID: r.Ident.StableID}
	a := g.admitted[key]
	if a == nil {
		a = &admission{}
		g.admitted[key] = a
	}
	g.resolveLocked(r, Outcome{Result: ResultAdmitted, Grants: a.grants})
	g.mu.Unlock()
	g.fire([]GuestEvent{r.event(AuditAdmitted, "")})
	return nil
}

// Deny refuses the waiting guest, and the person for the rest of the share:
// the deny-list is by numeric user ID, so another device knocking is refused
// at once, and the same person's other waiting requests are refused with it.
func (g *GuestShare) Deny(reqID string) error {
	g.mu.Lock()
	r, ok := g.pending[reqID]
	if !ok {
		g.mu.Unlock()
		return ErrNoRequest
	}
	out := g.denyUserLocked(r.Ident.UserID)
	g.mu.Unlock()
	g.fire(out)
	return nil
}

// denyUserLocked puts the user on the deny-list and refuses every request they
// have waiting, oldest first, returning the events to fire.
func (g *GuestShare) denyUserLocked(userID int64) []GuestEvent {
	g.denied[userID] = struct{}{}
	var same []*Request
	for _, p := range g.pending {
		if p.Ident.UserID == userID {
			same = append(same, p)
		}
	}
	sort.Slice(same, func(i, j int) bool { return same[i].Seq < same[j].Seq })
	var out []GuestEvent
	for _, p := range same {
		if g.resolveLocked(p, Outcome{Result: ResultDenied}) {
			out = append(out, p.event(AuditDenied, ReasonDenied))
		}
	}
	return out
}

// RevokeUser is what the sharer's revoke does to a person already admitted: the
// deny-list now covers them (every device, every later knock), their waiting
// requests are refused and their stored admissions are forgotten, so a
// reconnect from any node is turned away.
func (g *GuestShare) RevokeUser(userID int64) {
	g.mu.Lock()
	out := g.denyUserLocked(userID)
	for k := range g.admitted {
		if k.UserID == userID {
			delete(g.admitted, k)
		}
	}
	g.mu.Unlock()
	g.fire(out)
}

// Pending lists the waiting requests in arrival order.
func (g *GuestShare) Pending() []Request {
	g.mu.Lock()
	out := make([]Request, 0, len(g.pending))
	for _, r := range g.pending {
		out = append(out, Request{ID: r.ID, Ident: r.Ident, Seq: r.Seq, Since: r.Since})
	}
	g.mu.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Seq < out[j].Seq })
	return out
}

// IsDenied reports whether the user was denied in this share.
func (g *GuestShare) IsDenied(userID int64) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	_, ok := g.denied[userID]
	return ok
}

// SetGrants stores the control and clipboard bits of an admission, so a
// reconnect from the same key comes back with them. Surface is never stored:
// a guest never picks the display. It reports false when the key was never
// admitted.
func (g *GuestShare) SetGrants(key AdmissionKey, r Rights) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	a, ok := g.admitted[key]
	if ok {
		a.grants = r & (RightsControl | RightsClipboard)
	}
	return ok
}

// End ends the share: every waiting request is answered share_ended and later
// knocks are refused. reason is kept for the caller's own record.
func (g *GuestShare) End(reason string) {
	g.mu.Lock()
	if g.ended {
		g.mu.Unlock()
		return
	}
	g.ended, g.endReason = true, reason
	for _, r := range g.pending {
		g.resolveLocked(r, Outcome{Result: ResultEnded})
	}
	close(g.endedCh)
	g.mu.Unlock()
}

// Ended is closed when the share ends.
func (g *GuestShare) Ended() <-chan struct{} { return g.endedCh }

// attach records v as the live connection of key; a different one already
// recorded is closed, so a grant never serves two connections.
func (g *GuestShare) attach(key AdmissionKey, id ViewerID, closeFn func()) {
	g.mu.Lock()
	a := g.admitted[key]
	if a == nil {
		g.mu.Unlock()
		return
	}
	old := a.live
	a.live = &liveViewer{id: id, closeFn: closeFn}
	g.mu.Unlock()
	if old != nil && old.id != id {
		old.closeFn()
	}
}

// detach forgets the live connection when it is still id; the admission itself
// stays, so a reconnect is admitted at once.
func (g *GuestShare) detach(key AdmissionKey, id ViewerID) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if a := g.admitted[key]; a != nil && a.live != nil && a.live.id == id {
		a.live = nil
	}
}
