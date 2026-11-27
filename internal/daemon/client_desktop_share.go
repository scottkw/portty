package daemon

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client side of the click-to-share routes (desktop_share_api.go). A refusal the
// daemon words comes back as a *ShareError carrying its code (gui_required,
// gui_busy, no_request, no_viewer, own_viewer, tailscale_off, ...); any other
// failure is a plain error. Bodies are never logged.

// maxShareLine bounds one line of the events stream.
const maxShareLine = 1 << 20

// doShare is doJSON with the daemon's {"error": code} refusals turned into a
// *ShareError. A 503 with Retry-After while a handoff freezes writes is waited
// out by doRetrying, as for the other writes.
func (c *DaemonClient) doShare(method, path string, body, result any) error {
	var b []byte
	if body != nil {
		var err error
		if b, err = json.Marshal(body); err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
	}
	resp, err := c.doRetrying(c.http, time.Now().Add(frozenRetryBudget), func() (*http.Request, error) {
		var r io.Reader
		if b != nil {
			r = bytes.NewReader(b)
		}
		req, err := http.NewRequestWithContext(c.reqCtx(), method, c.base+path, r)
		if err == nil && b != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		return req, err
	})
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		var e ShareErrorResponse
		if json.Unmarshal(data, &e) == nil && e.Error != "" {
			return &ShareError{Code: e.Error, Detail: e.Detail}
		}
		return fmt.Errorf("daemon API %s %s: status %d", method, path, resp.StatusCode)
	}
	if result != nil && resp.StatusCode != http.StatusNoContent {
		if err := json.Unmarshal(data, result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

// StartDesktopShare starts the click-to-share, or returns the running one's link.
func (c *DaemonClient) StartDesktopShare() (DesktopShareStart, error) {
	var out DesktopShareStart
	err := c.doShare(http.MethodPost, "/desktop/share", nil, &out)
	return out, err
}

// DesktopShare reads the share's state and, while one is active, its link.
func (c *DaemonClient) DesktopShare() (DesktopShareStatus, error) {
	var out DesktopShareStatus
	err := c.doShare(http.MethodGet, "/desktop/share", nil, &out)
	return out, err
}

// StopDesktopShare ends the guest share. Ending a share that is not running is
// harmless.
func (c *DaemonClient) StopDesktopShare() error {
	return c.doShare(http.MethodDelete, "/desktop/share", nil, nil)
}

// AdmitDesktopGuest lets the waiting guest in.
func (c *DaemonClient) AdmitDesktopGuest(requestID string) error {
	return c.doShare(http.MethodPost, "/desktop/share/requests/"+url.PathEscape(requestID)+"/admit", nil, nil)
}

// DenyDesktopGuest refuses the waiting guest, and the person for the rest of the share.
func (c *DaemonClient) DenyDesktopGuest(requestID string) error {
	return c.doShare(http.MethodPost, "/desktop/share/requests/"+url.PathEscape(requestID)+"/deny", nil, nil)
}

// SetDesktopViewerRights sets a guest's control and clipboard rights.
func (c *DaemonClient) SetDesktopViewerRights(viewerID string, control, clipboard bool) error {
	return c.doShare(http.MethodPut, "/desktop/share/viewers/"+url.PathEscape(viewerID)+"/rights",
		DesktopRightsRequest{Control: control, Clipboard: clipboard}, nil)
}

// DisconnectDesktopViewer cuts one viewer at once.
func (c *DaemonClient) DisconnectDesktopViewer(viewerID string) error {
	return c.doShare(http.MethodDelete, "/desktop/share/viewers/"+url.PathEscape(viewerID), nil, nil)
}

// StreamDesktopShareEvents holds the GUI's events stream and calls fn once per
// line, in order, until ctx ends (it then returns nil). While it runs this
// client is the GUI the daemon watches: the stream ending, however it ends,
// ends the guest share. A second holder gets a *ShareError with code gui_busy;
// the daemon closing the stream is io.ErrUnexpectedEOF.
func (c *DaemonClient) StreamDesktopShareEvents(ctx context.Context, fn func(DesktopShareLine)) error {
	hc := *c.http
	hc.Timeout = 0 // the stream outlives the client's 30 s bound on bounded calls
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/desktop/share/events?gui=1", nil)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	resp, err := hc.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("do request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		var e ShareErrorResponse
		if json.Unmarshal(data, &e) == nil && e.Error != "" {
			return &ShareError{Code: e.Error, Detail: e.Detail}
		}
		return fmt.Errorf("daemon API GET /desktop/share/events: status %d", resp.StatusCode)
	}
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64<<10), maxShareLine)
	for sc.Scan() {
		var l DesktopShareLine
		if err := json.Unmarshal(sc.Bytes(), &l); err != nil {
			return fmt.Errorf("decode stream line: %w", err)
		}
		fn(l)
	}
	if ctx.Err() != nil {
		return nil
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("read stream: %w", err)
	}
	return io.ErrUnexpectedEOF
}
