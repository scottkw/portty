package daemon

import (
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/scottkw/portty/internal/relay"
)

// handlePreview relays a file's bytes a same-user local process handed us to
// that session's viewers (PREV-01). Registered only on the unix socket mux:
// the body is the file, the headers name it; nothing here takes or opens a
// path (PREV-03, TestPreview_SourceGate). The body is buffered and the
// response sent before streaming, so the freeze gate's write lock is held
// only for the local copy and a handoff drain is never blocked (Pitfall 4).
func (a *API) handlePreview(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	hub, ok := a.engine.Manager().Get(id)
	if !ok {
		http.Error(w, "no such session", http.StatusNotFound)
		return
	}
	select {
	case <-hub.Done():
		http.Error(w, "the session has ended", http.StatusConflict)
		return
	default:
	}
	mime := r.Header.Get("X-Portty-Mime")
	if !slices.Contains(relay.PreviewMIMEs, mime) {
		http.Error(w, "unsupported file type", http.StatusUnsupportedMediaType)
		return
	}
	key := r.Header.Get("X-Portty-Key")
	if len(key) != 32 || strings.Trim(key, "0123456789abcdef") != "" {
		http.Error(w, "invalid key", http.StatusBadRequest)
		return
	}
	raw, err := url.QueryUnescape(r.Header.Get("X-Portty-Name"))
	name := cleanPreviewName(raw)
	if err != nil || name == "" {
		http.Error(w, "invalid name", http.StatusBadRequest)
		return
	}
	if r.ContentLength > relay.PreviewMaxBytes {
		http.Error(w, "file too large", http.StatusRequestEntityTooLarge)
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, relay.PreviewMaxBytes))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			http.Error(w, "file too large", http.StatusRequestEntityTooLarge)
		} else {
			http.Error(w, "reading body: "+err.Error(), http.StatusBadRequest)
		}
		return
	}
	sent, err := hub.SendPreview(relay.Preview{Name: name, MIME: mime, Key: key, Data: data})
	switch {
	case errors.Is(err, relay.ErrPreviewBusy):
		w.Header().Set("Retry-After", "1")
		http.Error(w, "a preview is already streaming", http.StatusTooManyRequests)
		return
	case errors.Is(err, relay.ErrHubEnded):
		http.Error(w, "the session has ended", http.StatusConflict)
		return
	case err != nil:
		http.Error(w, "invalid preview", http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusAccepted, PreviewResponse{Viewers: sent.Viewers})
	log.Printf("viewer-diag: preview accepted session=%s bytes=%d mime=%s viewers=%d",
		relay.DiagID(id), len(data), mime, sent.Viewers)
}

// cleanPreviewName reduces a name to its base name without control, bidi
// override or isolate characters, trimmed and capped at 255 bytes on a rune
// boundary. Viewers show it as text and clean it again (11-05).
func cleanPreviewName(s string) string {
	s = s[strings.LastIndex(s, "/")+1:]
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == utf8.RuneError || r >= 0x202A && r <= 0x202E || r >= 0x2066 && r <= 0x2069 {
			return -1
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	for len(s) > 255 {
		_, n := utf8.DecodeLastRuneInString(s)
		s = s[:len(s)-n]
	}
	return strings.TrimSpace(s)
}
