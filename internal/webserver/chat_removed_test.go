package webserver

import (
	"net/http"
	"testing"
)

// TestChatRemoved_WebServerMux proves the guest-facing webserver no longer
// serves the chat history/export routes (CHAT-01). The request carries a valid
// read,write cap for an enabled session, so a 404/405 here means the route
// itself is gone, not that the capability gate rejected it.
func TestChatRemoved_WebServerMux(t *testing.T) {
	ws, client := testServer(t)
	ws.SetSigningKey(capTestKey)
	ws.EnableSession("sess-chat")
	token := issueCapFor(t, ws, "sess-chat", "read,write")

	for _, path := range []string{"/api/chat/sess-chat/history", "/api/chat/sess-chat/export"} {
		for _, method := range []string{http.MethodGet, http.MethodOptions} {
			req, _ := http.NewRequest(method, ws.BaseURL()+path+"?cap="+token, nil)
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("%s %s: %v", method, path, err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusMethodNotAllowed {
				t.Errorf("%s %s = %d; want 404 or 405 (chat API removed)", method, path, resp.StatusCode)
			}
		}
	}
}
