package main

// Access control. The board (page + live SSE) is gated behind a secret token
// carried in the URL (?k=...), so a stranger on the same Wi-Fi who just hits the
// port gets 403. The token lives in ~/.claude-board/token (0600) and is shared
// between the CLI (which prints the URL) and the server. /push is loopback-only
// — it only ever comes from the local curl hook — so it needs no token.

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func tokenFile() string { return filepath.Join(stateDir(), "token") }

// ensureToken returns the shared secret, generating and persisting it once.
func ensureToken() string {
	if b, err := os.ReadFile(tokenFile()); err == nil {
		if t := strings.TrimSpace(string(b)); t != "" {
			return t
		}
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "insecure" // never expected; degrade rather than crash
	}
	t := hex.EncodeToString(buf)
	_ = os.MkdirAll(stateDir(), 0o755)
	_ = os.WriteFile(tokenFile(), []byte(t+"\n"), 0o600)
	return t
}

func tokenOK(r *http.Request, tok string) bool {
	k := r.URL.Query().Get("k")
	return subtle.ConstantTimeCompare([]byte(k), []byte(tok)) == 1
}

func isLoopback(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func forbid(w http.ResponseWriter) {
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte("forbidden — open the URL printed by `claude-board url`\n"))
}
