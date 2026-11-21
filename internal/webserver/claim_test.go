package webserver

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/scottkw/portty/internal/relay"
)

// INPT-06 through the share server: a write-cap viewer's claim takes the
// grid, a read-only cap can never size the PTY, and a 4-byte frame (an older
// client's report) never moves a known grid.

type claimScene struct {
	hub   *relay.Hub
	local *relay.Subscriber
	conn  *websocket.Conn
	ctx   context.Context
	grids chan [2]int   // 0x02 frames the share viewer read
	pongs chan struct{} // MsgPong frames the share viewer read
}

// newClaimScene starts a hub at 100x40 (claimed by a local viewer) and dials
// a share viewer with the given cap perms.
func newClaimScene(t *testing.T, sessionID, perms string) *claimScene {
	t.Helper()
	ws, client, _, _ := testServerWithHub(t, sessionID)
	ws.SetSigningKey(capTestKey)
	hub, ok := ws.manager.Get(sessionID)
	if !ok {
		t.Fatal("hub not found")
	}
	local := &relay.Subscriber{Msgs: make(chan []byte, 256), CloseSlow: func() {}, Origin: "local"}
	hub.Subscribe(local)
	if err := hub.ClaimGrid(local, 100, 40); err != nil {
		t.Fatal(err)
	}
	hdr := http.Header{}
	hdr.Set("Origin", ws.BaseURL())
	conn := dialWebServerWS(t, client, ws.BaseURL(), "/sessions/"+sessionID+"/ws?cap="+issueCapFor(t, ws, sessionID, perms), hdr)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	s := &claimScene{hub: hub, local: local, conn: conn, ctx: ctx, grids: make(chan [2]int, 64), pongs: make(chan struct{}, 8)}
	go func() {
		for {
			_, msg, err := conn.Read(ctx)
			if err != nil {
				return
			}
			switch {
			case len(msg) == 5 && msg[0] == relay.MsgResize:
				s.grids <- [2]int{int(msg[1])<<8 | int(msg[2]), int(msg[3])<<8 | int(msg[4])}
			case len(msg) == 1 && msg[0] == relay.MsgPong:
				s.pongs <- struct{}{}
			}
		}
	}()
	s.waitGrid(t, [2]int{100, 40}) // the join push
	return s
}

func (s *claimScene) send(t *testing.T, frame ...byte) {
	t.Helper()
	if err := s.conn.Write(s.ctx, websocket.MessageBinary, frame); err != nil {
		t.Fatalf("write %v: %v", frame, err)
	}
}

// sync sends a ping and waits for its pong: the read pump handled every
// frame sent before it.
func (s *claimScene) sync(t *testing.T) {
	t.Helper()
	s.send(t, relay.MsgPing)
	select {
	case <-s.pongs:
	case <-s.ctx.Done():
		t.Fatal("no pong")
	}
}

func (s *claimScene) waitGrid(t *testing.T, want [2]int) {
	t.Helper()
	for {
		select {
		case g := <-s.grids:
			if g == want {
				return
			}
		case <-s.ctx.Done():
			t.Fatalf("the viewer never read a %dx%d grid frame", want[0], want[1])
		}
	}
}

func (s *claimScene) requireGrid(t *testing.T, cols, rows int) {
	t.Helper()
	if s.hub.Cols() != cols || s.hub.Rows() != rows {
		t.Fatalf("hub grid %dx%d, want %dx%d", s.hub.Cols(), s.hub.Rows(), cols, rows)
	}
}

func TestWebClaim_WriteCapTakesGrid(t *testing.T) {
	s := newClaimScene(t, "claim-write", "read,write")
	s.send(t, relay.MsgResize2, 0, 120, 0, 30, relay.ResizeClaim)
	s.sync(t)
	s.requireGrid(t, 120, 30)
	s.waitGrid(t, [2]int{120, 30})

	if err := s.hub.ClaimGrid(s.local, 100, 40); err != nil { // the host takes it back
		t.Fatal(err)
	}
	s.requireGrid(t, 100, 40)
	s.waitGrid(t, [2]int{100, 40})
}

func TestWebClaim_ReadOnlyCapIgnored(t *testing.T) {
	s := newClaimScene(t, "claim-readonly", "read")
	s.send(t, relay.MsgResize2, 0, 120, 0, 30, relay.ResizeClaim)
	s.send(t, relay.MsgResize2, 0, 90, 0, 20)
	s.sync(t)
	s.requireGrid(t, 100, 40)
}

func TestWebReport_KnownGridUnchanged(t *testing.T) {
	s := newClaimScene(t, "claim-report", "read,write")
	s.send(t, relay.MsgResize2, 0, 120, 0, 30) // an older client's 4-byte report
	s.sync(t)
	s.requireGrid(t, 100, 40)
}

// INPT-06 typing through the share pump: a write-cap viewer's keystroke claims
// with the size it reported; a terminal auto-reply and a viewer that never
// reported a size do not.
func TestWebTyping_TakesGrid(t *testing.T) {
	s := newClaimScene(t, "typing-takes", "read,write")
	s.send(t, relay.MsgResize2, 0, 120, 0, 30) // a report: moves nothing
	s.sync(t)
	s.requireGrid(t, 100, 40)
	s.send(t, relay.MakeInputFrame([]byte("x"))...)
	s.sync(t)
	s.requireGrid(t, 120, 30)
	s.waitGrid(t, [2]int{120, 30})

	if err := s.hub.NoteTyping(s.local); err != nil { // the host types: it takes the grid back
		t.Fatal(err)
	}
	s.requireGrid(t, 100, 40)
	s.waitGrid(t, [2]int{100, 40})
}

func TestWebTyping_AutoReplyNoClaim(t *testing.T) {
	s := newClaimScene(t, "typing-reply", "read,write")
	s.send(t, relay.MsgResize2, 0, 120, 0, 30)
	s.send(t, relay.MakeInputFrame([]byte("\x1b[12;40R"))...) // a cursor position report
	s.sync(t)
	s.requireGrid(t, 100, 40)
}

func TestWebTyping_NoReportNoClaim(t *testing.T) {
	s := newClaimScene(t, "typing-noreport", "read,write")
	s.send(t, relay.MakeInputFrame([]byte("x"))...)
	s.sync(t)
	s.requireGrid(t, 100, 40)
}

func TestWebClaim_Clamped(t *testing.T) {
	s := newClaimScene(t, "claim-clamp", "read,write")
	s.send(t, relay.MsgResize2, 0x0f, 0xa0, 0x0f, 0xa0, relay.ResizeClaim) // 4000x4000
	s.sync(t)
	s.requireGrid(t, relay.MaxGridCols, relay.MaxGridRows)
	s.waitGrid(t, [2]int{500, 200})
}
