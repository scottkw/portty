package desktop

import (
	"context"
	"errors"
	"log/slog"

	"github.com/coder/websocket"
)

// Close codes of a guest's signalling socket, beside 4401, 4410 and 4429. The
// reasons are fixed codes the guest page reads, never text it must parse.
const (
	CloseGuestDenied  websocket.StatusCode = 4430 // the sharer said no, or the user was denied earlier
	CloseGuestTimeout websocket.StatusCode = 4431 // nobody answered in admitTimeout
	CloseGuestRevoked websocket.StatusCode = 4432 // the sharer cut the guest, or its media path was not its own
	CloseShareEnded   websocket.StatusCode = 4433 // the share ended
)

// Close reasons of a guest socket. A newer connection from the same node
// closes the older one with a normal close (1000) and guestReasonReplaced.
const (
	guestReasonDenied       = "denied"
	guestReasonTimeout      = "admit_timeout"
	guestReasonRevoked      = "revoked"
	guestReasonShareEnded   = "share_ended"
	guestReasonTokenInvalid = "token_invalid"
	guestReasonReplaced     = "replaced"
	guestReasonFull         = "full"
)

// Additive signalling messages (CONTEXT [Signalling]); ProtocolVersion stays 1.
const (
	typeAdmission = "admission"
	typeRights    = "rights"
)

// Admission states a guest is told.
const (
	admissionWaiting  = "waiting"
	admissionAdmitted = "admitted"
)

type msgAdmission struct {
	Type  string `json:"type"`
	State string `json:"state"`
}

// msgRights tells a guest what it may do now: the same map the hello carries.
type msgRights struct {
	Type   string          `json:"type"`
	Rights map[string]bool `json:"rights"`
}

func rightsMap(r Rights) map[string]bool {
	return map[string]bool{
		"control":   r&RightsControl != 0,
		"clipboard": r&RightsClipboard != 0,
		"surface":   r&RightsSurface != 0,
	}
}

// ServeGuestViewer runs one guest's signalling session. The route layer has
// already checked the tailnet peer and the token and resolved id from WhoIs;
// admit re-checks the token at the join. The guest knocks at Admit and waits
// with no PeerConnection and no offer accepted (only ping is answered). Once
// admitted it gets its rights and the same offer, answer and read loop as an
// own-account viewer, as a view-only viewer until grants arrive.
func (m *Manager) ServeGuestViewer(ctx context.Context, c *websocket.Conn, admit func() bool, gs *GuestShare, id Identity) {
	defer recoverLog()
	defer func() { _ = c.CloseNow() }()
	c.SetReadLimit(readLimit)

	if m.Full() {
		_ = c.Close(CloseFull, guestReasonFull)
		return
	}
	// The gate has vouched for who this is; an identity that cannot key an
	// admission fails closed. A token that stopped verifying cannot knock.
	if gs == nil || id.UserID == 0 || id.StableID == "" || admit == nil || !admit() {
		_ = c.Close(CloseTokenRejected, guestReasonTokenInvalid)
		return
	}

	in, err := read(ctx, c, helloTimeout)
	if err != nil {
		if errors.Is(err, errBadMessage) {
			reject(ctx, c, errCodeBadMessage)
		}
		return
	}
	if in.Type != typeHello {
		reject(ctx, c, errCodeBadMessage)
		return
	}
	if in.V != ProtocolVersion {
		reject(ctx, c, errCodeVersion)
		return
	}
	vid, err := newViewerID()
	if err != nil {
		slog.Error("desktop: viewer id", "err", err)
		return
	}

	req, err := gs.Knock(id)
	switch {
	case errors.Is(err, ErrShareFull):
		_ = c.Close(CloseFull, guestReasonFull)
		return
	case errors.Is(err, ErrDenied):
		_ = c.Close(CloseGuestDenied, guestReasonDenied)
		return
	case errors.Is(err, ErrShareEnded):
		_ = c.Close(CloseShareEnded, guestReasonShareEnded)
		return
	case err != nil:
		slog.Error("desktop: guest knock", "err", err)
		return
	}

	hello := msgHello{
		Type: typeHello, V: ProtocolVersion, Viewer: vid,
		Caps:   map[string]int{"video": 1, "input": 1},
		Rights: rightsMap(RightsViewOnly), // the rights message follows an admission
	}
	if send(ctx, c, hello) != nil {
		gs.Withdraw(req)
		return
	}
	select {
	case <-req.done: // a re-admission is already answered: no waiting card
	default:
		if send(ctx, c, msgAdmission{Type: typeAdmission, State: admissionWaiting}) != nil {
			gs.Withdraw(req)
			return
		}
	}

	out, pend, ok := m.guestWait(ctx, c, gs, req)
	if !ok {
		return
	}
	switch out.Result {
	case ResultAdmitted:
	case ResultDenied:
		_ = c.Close(CloseGuestDenied, guestReasonDenied)
		return
	case ResultTimeout:
		_ = c.Close(CloseGuestTimeout, guestReasonTimeout)
		return
	case ResultReplaced:
		_ = c.Close(websocket.StatusNormalClosure, guestReasonReplaced)
		return
	case ResultEnded:
		_ = c.Close(CloseShareEnded, guestReasonShareEnded)
		return
	default:
		return
	}

	// A guest never holds RightsSurface, whatever was stored.
	rights := out.Grants & (RightsControl | RightsClipboard)
	if send(ctx, c, msgAdmission{Type: typeAdmission, State: admissionAdmitted}) != nil {
		return
	}
	if send(ctx, c, msgRights{Type: typeRights, Rights: rightsMap(rights)}) != nil {
		return
	}
	v := &viewer{
		Viewer: Viewer{ID: ViewerID(vid), Rights: rights, JoinedAt: m.d.Now(), Mode: ModeGuest, Identity: id},
		ws:     c, inputCap: in.Caps["input"], in: &viewerInput{}, gs: gs,
	}
	v.setRightsBits(rights)
	m.serveAdmitted(ctx, c, v, admit, pend)
}

// guestWait holds the socket while the guest waits at Admit. Only ping is
// answered; anything else ends the socket with bad_message. It returns the
// request's outcome and the read still in flight (the offer may be its answer),
// or false when the socket or ctx ended first, in which case it has cleaned up.
//
// A blocked websocket read cannot be cancelled without closing the socket, so
// one read stays in flight and is handed to serveAdmitted.
func (m *Manager) guestWait(ctx context.Context, c *websocket.Conn, gs *GuestShare, req *Request) (Outcome, <-chan readResult, bool) {
	reads := make(chan readResult, 1)
	startRead := func() {
		go func() {
			defer recoverLog()
			in, err := read(ctx, c, 0)
			reads <- readResult{in, err}
		}()
	}
	startRead()
	type awaited struct {
		out Outcome
		ok  bool
	}
	outc := make(chan awaited, 1)
	go func() {
		defer recoverLog()
		o, ok := gs.Await(ctx, req)
		outc <- awaited{o, ok}
	}()
	for {
		select {
		case r := <-reads:
			if r.err == nil && r.in.Type == typePing {
				if send(ctx, c, msgPong{Type: typePong, N: r.in.N}) != nil {
					gs.Withdraw(req)
					return Outcome{}, nil, false
				}
				startRead()
				continue
			}
			gs.Withdraw(req)
			if r.err == nil || errors.Is(r.err, errBadMessage) {
				reject(ctx, c, errCodeBadMessage)
			}
			return Outcome{}, nil, false
		case a := <-outc:
			if !a.ok {
				gs.Withdraw(req)
				return Outcome{}, nil, false
			}
			return a.out, reads, true
		}
	}
}
