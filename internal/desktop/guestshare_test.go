package desktop_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/scottkw/portty/internal/desktop"
)

var shareT0 = time.Date(2026, 11, 27, 9, 0, 0, 0, time.UTC)

// shareRig is a GuestShare on a frozen clock (so every request has the same
// Since) that records its events and never starts a real timer.
type shareRig struct {
	gs     *desktop.GuestShare
	mu     sync.Mutex
	now    time.Time
	events []desktop.GuestEvent
}

func newShareRig(t *testing.T) *shareRig {
	t.Helper()
	r := &shareRig{now: shareT0}
	gs, err := desktop.NewGuestShare(func() time.Time {
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.now
	}, func(e desktop.GuestEvent) {
		r.mu.Lock()
		r.events = append(r.events, e)
		r.mu.Unlock()
	})
	if err != nil {
		t.Fatal(err)
	}
	gs.SetTimerForTest(func(time.Duration) (<-chan time.Time, func()) { return nil, func() {} })
	r.gs = gs
	return r
}

func (r *shareRig) advance(d time.Duration) {
	r.mu.Lock()
	r.now = r.now.Add(d)
	r.mu.Unlock()
}

// kinds lists the recorded events as "kind" or "kind/reason".
func (r *shareRig) kinds() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, e := range r.events {
		s := e.Kind
		if e.Reason != "" {
			s += "/" + e.Reason
		}
		out = append(out, s)
	}
	return out
}

func (r *shareRig) count(kind string) int {
	n := 0
	for _, k := range r.kinds() {
		if k == kind {
			n++
		}
	}
	return n
}

func mustKnock(t *testing.T, gs *desktop.GuestShare, uid int64, node string) *desktop.Request {
	t.Helper()
	req, err := gs.Knock(guestIdent(uid, node))
	if err != nil {
		t.Fatalf("Knock(%d,%s): %v", uid, node, err)
	}
	return req
}

func result(t *testing.T, gs *desktop.GuestShare, req *desktop.Request) desktop.Outcome {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, ok := gs.Await(ctx, req)
	if !ok {
		t.Fatal("request was not answered")
	}
	return out
}

func TestGuestShare_PendingKeepsArrivalOrder(t *testing.T) {
	r := newShareRig(t)
	// Frozen clock: every Since is equal, so Seq alone orders the queue.
	var ids []string
	for i := int64(0); i < 5; i++ {
		ids = append(ids, mustKnock(t, r.gs, 2000+i, "n").ID)
	}
	got := r.gs.Pending()
	if len(got) != 5 {
		t.Fatalf("Pending has %d requests, want 5", len(got))
	}
	for i, p := range got {
		if p.ID != ids[i] {
			t.Errorf("Pending()[%d] is %s, want the %dth knock %s", i, p.ID, i, ids[i])
		}
		if i > 0 && p.Seq <= got[i-1].Seq {
			t.Errorf("Seq not increasing at %d: %d after %d", i, p.Seq, got[i-1].Seq)
		}
	}
}

func TestGuestShare_NewKnockFromSameNodeReplacesOlder(t *testing.T) {
	r := newShareRig(t)
	first := mustKnock(t, r.gs, 1001, "nodeA")
	second := mustKnock(t, r.gs, 1001, "nodeA")
	if out := result(t, r.gs, first); out.Result != desktop.ResultReplaced {
		t.Errorf("older request result %q, want replaced", out.Result)
	}
	if p := r.gs.Pending(); len(p) != 1 || p[0].ID != second.ID {
		t.Errorf("Pending() = %+v, want only the newer request", p)
	}
	if got, want := r.count("denied/replaced"), 1; got != want {
		t.Errorf("denied/replaced events %d, want %d (%v)", got, want, r.kinds())
	}
}

func TestGuestShare_ThirdKnockFromOneUserReplacesTheOldest(t *testing.T) {
	r := newShareRig(t)
	a := mustKnock(t, r.gs, 1001, "n1")
	b := mustKnock(t, r.gs, 1001, "n2")
	c := mustKnock(t, r.gs, 1001, "n3")
	if out := result(t, r.gs, a); out.Result != desktop.ResultReplaced {
		t.Errorf("user's oldest result %q, want replaced", out.Result)
	}
	p := r.gs.Pending()
	if len(p) != 2 || p[0].ID != b.ID || p[1].ID != c.ID {
		t.Errorf("Pending() = %+v, want the second and third requests", p)
	}
}

func TestGuestShare_NinthPendingKnockIsRefusedAndReplacesNobody(t *testing.T) {
	r := newShareRig(t)
	for i := int64(0); i < 8; i++ {
		mustKnock(t, r.gs, 3000+i, "n")
	}
	if _, err := r.gs.Knock(guestIdent(3999, "n")); !errors.Is(err, desktop.ErrShareFull) {
		t.Fatalf("ninth knock error %v, want ErrShareFull", err)
	}
	if n := len(r.gs.Pending()); n != 8 {
		t.Errorf("%d pending after a refused knock, want 8", n)
	}
	// A user already holding a slot may still replace their own request at the cap.
	if _, err := r.gs.Knock(guestIdent(3000, "n")); err != nil {
		t.Errorf("a same-node knock at the cap failed: %v", err)
	}
	if n := len(r.gs.Pending()); n != 8 {
		t.Errorf("%d pending after the replacement, want 8", n)
	}
}

func TestGuestShare_TimeoutFromTheInjectedTimer(t *testing.T) {
	r := newShareRig(t)
	fire := make(chan time.Time)
	var asked time.Duration
	r.gs.SetTimerForTest(func(d time.Duration) (<-chan time.Time, func()) {
		asked = d
		return fire, func() {}
	})
	req := mustKnock(t, r.gs, 1001, "nodeA")
	done := make(chan desktop.Outcome, 1)
	go func() {
		out, _ := r.gs.Await(context.Background(), req)
		done <- out
	}()
	fire <- shareT0.Add(5 * time.Minute)
	select {
	case out := <-done:
		if out.Result != desktop.ResultTimeout {
			t.Errorf("result %q, want timeout", out.Result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Await did not return after the timer fired")
	}
	if asked != 5*time.Minute {
		t.Errorf("timer asked for %v, want 5m", asked)
	}
	if r.count("denied/timeout") != 1 || len(r.gs.Pending()) != 0 {
		t.Errorf("events %v, pending %d: want one denied/timeout and an empty queue", r.kinds(), len(r.gs.Pending()))
	}
}

func TestGuestShare_RequestOlderThanFiveMinutesTimesOutByTheClock(t *testing.T) {
	r := newShareRig(t)
	req := mustKnock(t, r.gs, 1001, "nodeA")
	r.advance(5*time.Minute + time.Second)
	if out := result(t, r.gs, req); out.Result != desktop.ResultTimeout {
		t.Errorf("result %q, want timeout", out.Result)
	}
	if err := r.gs.Admit(req.ID); !errors.Is(err, desktop.ErrNoRequest) {
		t.Errorf("Admit after the timeout: %v, want ErrNoRequest", err)
	}
}

func TestGuestShare_DenyRefusesTheUserFromAnyNode(t *testing.T) {
	r := newShareRig(t)
	a := mustKnock(t, r.gs, 1001, "n1")
	other := mustKnock(t, r.gs, 1001, "n2")
	bystander := mustKnock(t, r.gs, 1002, "n1")
	if err := r.gs.Deny(a.ID); err != nil {
		t.Fatal(err)
	}
	if !r.gs.IsDenied(1001) || r.gs.IsDenied(1002) {
		t.Errorf("IsDenied: 1001=%v 1002=%v, want true and false", r.gs.IsDenied(1001), r.gs.IsDenied(1002))
	}
	if out := result(t, r.gs, other); out.Result != desktop.ResultDenied {
		t.Errorf("the same person's other request: %q, want denied", out.Result)
	}
	if p := r.gs.Pending(); len(p) != 1 || p[0].ID != bystander.ID {
		t.Errorf("Pending() = %+v, want only the other user", p)
	}
	before := r.count("admit_request")
	if _, err := r.gs.Knock(guestIdent(1001, "n3")); !errors.Is(err, desktop.ErrDenied) {
		t.Errorf("knock after Deny from a third node: %v, want ErrDenied", err)
	}
	if got := r.count("admit_request"); got != before {
		t.Errorf("a refused knock fired admit_request (%d, was %d)", got, before)
	}
}

func TestGuestShare_ReadmitsTheSameKeyWithItsGrantsAndClosesTheOlderSocket(t *testing.T) {
	r := newShareRig(t)
	first := mustKnock(t, r.gs, 1001, "nodeA")
	if err := r.gs.Admit(first.ID); err != nil {
		t.Fatal(err)
	}
	key := desktop.AdmissionKey{UserID: 1001, StableID: "stable-nodeA"}
	if !r.gs.SetGrants(key, desktop.RightsControl|desktop.RightsClipboard|desktop.RightsSurface) {
		t.Fatal("SetGrants: the key was not admitted")
	}
	var closed int
	r.gs.AttachForTest(key, "v1", func() { closed++ })

	requestsBefore := r.count("admit_request")
	again := mustKnock(t, r.gs, 1001, "nodeA")
	out := result(t, r.gs, again)
	if out.Result != desktop.ResultAdmitted {
		t.Fatalf("reconnect result %q, want admitted at once", out.Result)
	}
	// Surface is never stored: a guest never picks the display.
	if out.Grants != desktop.RightsControl|desktop.RightsClipboard {
		t.Errorf("stored grants %v, want control and clipboard only", out.Grants)
	}
	if closed != 1 {
		t.Errorf("the older socket was closed %d times, want 1", closed)
	}
	if got := r.count("admit_request"); got != requestsBefore {
		t.Errorf("a re-admission fired admit_request")
	}
	if len(r.gs.Pending()) != 0 {
		t.Errorf("a re-admission left a pending request")
	}
}

func TestGuestShare_SecondNodeOfAnAdmittedUserWaits(t *testing.T) {
	r := newShareRig(t)
	first := mustKnock(t, r.gs, 1001, "nodeA")
	if err := r.gs.Admit(first.ID); err != nil {
		t.Fatal(err)
	}
	second := mustKnock(t, r.gs, 1001, "nodeB")
	if p := r.gs.Pending(); len(p) != 1 || p[0].ID != second.ID {
		t.Errorf("Pending() = %+v, want the second node waiting", p)
	}
	// And a stranger's grants are not inherited: admitted keys stay per node.
	if err := r.gs.Admit(second.ID); err != nil {
		t.Fatal(err)
	}
	if out := result(t, r.gs, second); out.Grants != desktop.RightsViewOnly {
		t.Errorf("a new node came in with grants %v, want none", out.Grants)
	}
}

func TestGuestShare_EndAnswersEveryWaiterAndRefusesLaterKnocks(t *testing.T) {
	r := newShareRig(t)
	a := mustKnock(t, r.gs, 1001, "n")
	b := mustKnock(t, r.gs, 1002, "n")
	r.gs.End("test")
	r.gs.End("again") // idempotent
	for _, req := range []*desktop.Request{a, b} {
		if out := result(t, r.gs, req); out.Result != desktop.ResultEnded {
			t.Errorf("waiter result %q, want share_ended", out.Result)
		}
	}
	select {
	case <-r.gs.Ended():
	default:
		t.Error("Ended() is not closed after End")
	}
	if _, err := r.gs.Knock(guestIdent(1003, "n")); !errors.Is(err, desktop.ErrShareEnded) {
		t.Errorf("knock after End: %v, want ErrShareEnded", err)
	}
	if err := r.gs.Admit(a.ID); !errors.Is(err, desktop.ErrNoRequest) {
		t.Errorf("Admit after End: %v, want ErrNoRequest", err)
	}
}

func TestGuestShare_UnknownAnsweredAndEndedRequestsChangeNothing(t *testing.T) {
	r := newShareRig(t)
	req := mustKnock(t, r.gs, 1001, "n")
	if err := r.gs.Admit("nope"); !errors.Is(err, desktop.ErrNoRequest) {
		t.Errorf("Admit(unknown): %v", err)
	}
	if err := r.gs.Deny("nope"); !errors.Is(err, desktop.ErrNoRequest) {
		t.Errorf("Deny(unknown): %v", err)
	}
	if err := r.gs.Admit(req.ID); err != nil {
		t.Fatal(err)
	}
	events := len(r.kinds())
	if err := r.gs.Deny(req.ID); !errors.Is(err, desktop.ErrNoRequest) {
		t.Errorf("Deny(answered): %v, want ErrNoRequest", err)
	}
	if err := r.gs.Admit(req.ID); !errors.Is(err, desktop.ErrNoRequest) {
		t.Errorf("Admit(answered): %v, want ErrNoRequest", err)
	}
	if r.gs.IsDenied(1001) || len(r.kinds()) != events {
		t.Errorf("a refused call changed state: denied=%v events %v", r.gs.IsDenied(1001), r.kinds())
	}
}

func TestGuestShare_EventsCarryTheSanitizedIdentity(t *testing.T) {
	r := newShareRig(t)
	req, err := r.gs.Knock(desktop.NewIdentity(1001, "ada@example.com", "Ada\u202e Lovelace", "mac\nbook", "stable-n", "100.64.0.9"))
	if err != nil {
		t.Fatal(err)
	}
	if err := r.gs.Admit(req.ID); err != nil {
		t.Fatal(err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.events) != 2 || r.events[0].Kind != "admit_request" || r.events[1].Kind != "admitted" {
		t.Fatalf("events %+v, want admit_request then admitted", r.events)
	}
	for _, e := range r.events {
		if e.Ident.DisplayName != "Ada Lovelace" || e.Ident.NodeName != "mac book" || e.RequestID != req.ID {
			t.Errorf("event %+v: want the sanitized names and the request ID", e)
		}
	}
}
