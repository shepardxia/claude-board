package main

// HTTP relay: receives raw Claude Code PostToolUse payloads at /push, parses them
// into display cards, and streams them live to browsers over SSE (/events).
//
// No storage, no per-call interpreter. The global hook is just a `curl` that
// pipes Claude Code's raw hook JSON here; ALL parsing (extract output / summary /
// strip ANSI / pretty JSON) happens in THIS long-running process. Only a tiny
// RING of recent cards is kept so a freshly opened page isn't blank. No files,
// no DB, no time-window sink. Restart = clean slate.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	ring = 50        // tiny catch-up buffer for late-joining browsers
	capB = 200_000   // max body chars kept per card
)

var ansi = regexp.MustCompile("\x1b\\[[0-9;?]*[ -/]*[@-~]|\x1b\\][^\x07\x1b]*(?:\x07|\x1b\\\\)")

// --- transform: raw hook payload -> display card -----------------------------
type hookPayload struct {
	ToolName     string          `json:"tool_name"`
	Cwd          string          `json:"cwd"`
	SessionID    string          `json:"session_id"`
	ToolInput    json.RawMessage `json:"tool_input"`
	ToolResponse json.RawMessage `json:"tool_response"`
}

type card struct {
	Kind  string  `json:"kind"`
	Label string  `json:"label"`
	Sub   string  `json:"sub"`
	Body  string  `json:"body"`
	Tag   string  `json:"tag"`
	Sid   string  `json:"sid"`
	JSON  bool    `json:"json"`
	Ts    float64 `json:"ts"`
}

func asStr(v any) string { s, _ := v.(string); return s }

func summary(tool string, inp map[string]any) string {
	get := func(k string) string { return asStr(inp[k]) }
	switch tool {
	case "Read", "Edit", "Write", "NotebookEdit":
		return get("file_path")
	case "Bash":
		return get("command")
	case "Grep", "Glob":
		s := get("pattern")
		if p := get("path"); p != "" {
			s += "  in " + p
		}
		return s
	case "Task", "Agent":
		return get("description")
	case "WebFetch", "WebSearch":
		if u := get("url"); u != "" {
			return u
		}
		return get("query")
	}
	b := jmarshal(inp, false)
	if len(b) > 400 {
		b = b[:400]
	}
	return string(b)
}

func extract(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case []any:
		var out []string
		for _, b := range t {
			if m, ok := b.(map[string]any); ok {
				switch {
				case m["type"] == "text":
					out = append(out, asStr(m["text"]))
				case m["type"] == "image":
					out = append(out, "[image]")
				case m["text"] != nil:
					out = append(out, asStr(m["text"]))
				default:
					out = append(out, string(jmarshal(m, false)))
				}
			} else {
				out = append(out, fmt.Sprint(b))
			}
		}
		return strings.Join(out, "\n")
	case map[string]any:
		if f, ok := t["file"].(map[string]any); ok {
			if c, ok := f["content"]; ok {
				return asStr(c)
			}
		}
		_, hasOut := t["stdout"]
		_, hasErr := t["stderr"]
		if hasOut || hasErr {
			s := asStr(t["stdout"])
			e := asStr(t["stderr"])
			if strings.TrimSpace(e) != "" {
				return s + "\n[stderr]\n" + e
			}
			return s
		}
		if c, ok := t["content"]; ok {
			return extract(c)
		}
		for _, k := range []string{"text", "output", "result"} {
			if x, ok := t[k]; ok {
				return extract(x)
			}
		}
		return string(jmarshal(t, true))
	}
	return fmt.Sprint(v)
}

func maybeJSON(body string) (string, bool) {
	b := strings.TrimSpace(body)
	if len(b) > 0 && (b[0] == '{' || b[0] == '[') {
		var obj any
		if json.Unmarshal([]byte(b), &obj) == nil {
			switch obj.(type) {
			case map[string]any, []any:
				return string(jmarshal(obj, true)), true
			}
		}
	}
	return body, false
}

// jmarshal marshals without HTML-escaping (so <, >, & stay literal like Python's
// json.dumps), optionally 2-space indented.
func jmarshal(v any, indent bool) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if indent {
		enc.SetIndent("", "  ")
	}
	if enc.Encode(v) != nil {
		return nil
	}
	return bytes.TrimRight(buf.Bytes(), "\n")
}

func cap200(s string) string {
	if len(s) <= capB {
		return s
	}
	r := []rune(s)
	if len(r) > capB {
		r = r[:capB]
	}
	return string(r) + "\n… [truncated]"
}

func cardFromHook(raw []byte) (card, bool) {
	var p hookPayload
	if json.Unmarshal(raw, &p) != nil {
		return card{}, false
	}
	var inp map[string]any
	_ = json.Unmarshal(p.ToolInput, &inp)
	var resp any
	_ = json.Unmarshal(p.ToolResponse, &resp)

	body := ansi.ReplaceAllString(extract(resp), "")
	body = strings.ReplaceAll(body, "\r", "")
	body, isJSON := maybeJSON(body)
	body = cap200(body)

	tool := p.ToolName
	if tool == "" {
		tool = "?"
	}
	tag := filepath.Base(strings.TrimRight(p.Cwd, "/"))
	if tag == "" || tag == "." || tag == "/" {
		tag = "?"
	}
	sid := p.SessionID
	if len(sid) > 8 {
		sid = sid[:8]
	}
	return card{
		Kind: "card", Label: tool, Sub: summary(tool, inp), Body: body,
		Tag: tag, Sid: sid, JSON: isJSON,
		Ts: float64(time.Now().UnixNano()) / 1e9,
	}, true
}

// --- SSE hub -----------------------------------------------------------------
var clearMsg = []byte(`{"kind":"clear"}`)

type hub struct {
	mu      sync.Mutex
	recent  [][]byte
	clients map[chan []byte]bool
}

func newHub() *hub { return &hub{clients: map[chan []byte]bool{}} }

func (h *hub) emit(msg []byte, keep bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if keep {
		h.recent = append(h.recent, msg)
		if len(h.recent) > ring {
			h.recent = h.recent[len(h.recent)-ring:]
		}
	}
	for c := range h.clients {
		select {
		case c <- msg:
		default: // slow client: drop it, it will reconnect
			delete(h.clients, c)
			close(c)
		}
	}
}

func (h *hub) clear() {
	h.mu.Lock()
	h.recent = nil
	for c := range h.clients {
		select {
		case c <- clearMsg:
		default:
			delete(h.clients, c)
			close(c)
		}
	}
	h.mu.Unlock()
}

func (h *hub) events(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "no streaming", 500)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(200)

	ch := make(chan []byte, 256)
	h.mu.Lock()
	backlog := make([][]byte, len(h.recent))
	copy(backlog, h.recent)
	h.clients[ch] = true
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		if h.clients[ch] {
			delete(h.clients, ch)
			close(ch)
		}
		h.mu.Unlock()
	}()

	for _, m := range backlog {
		fmt.Fprintf(w, "data: %s\n\n", m)
	}
	fl.Flush()

	ping := time.NewTicker(15 * time.Second)
	defer ping.Stop()
	for {
		select {
		case m, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", m)
			fl.Flush()
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
			fl.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (h *hub) push(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(404)
		return
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, 64<<20))
	c, ok := cardFromHook(body)
	if !ok {
		w.WriteHeader(400)
		return
	}
	h.emit(jmarshal(c, false), true)
	w.WriteHeader(204)
}

func lanIP() string {
	c, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer c.Close()
	return c.LocalAddr().(*net.UDPAddr).IP.String()
}

func serve(port int) error {
	h := newHub()
	tok := ensureToken()
	ch := loadChallenge() // optional tappable quiz gate; nil = token-only
	mux := http.NewServeMux()

	// read side (page + live stream + clear): token in URL OR a session cookie
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		if !authed(r, tok) {
			forbid(w)
			return
		}
		h.events(w, r)
	})
	mux.HandleFunc("/clear", func(w http.ResponseWriter, r *http.Request) {
		if !authed(r, tok) {
			forbid(w)
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(404)
			return
		}
		h.clear()
		w.WriteHeader(204)
	})
	mux.HandleFunc("/auth", func(w http.ResponseWriter, r *http.Request) {
		if ch == nil {
			forbid(w)
			return
		}
		handleAuth(w, r, ch, tok)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if !authed(r, tok) {
			if ch != nil {
				serveChallenge(w, ch, false) // show the quiz instead of a bare 403
			} else {
				forbid(w)
			}
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, page)
	})

	// write side: loopback only (the local curl hook); no token needed
	mux.HandleFunc("/push", func(w http.ResponseWriter, r *http.Request) {
		if !isLoopback(r) {
			w.WriteHeader(403)
			return
		}
		h.push(w, r)
	})

	gate := "token in URL"
	if ch != nil {
		gate = "token in URL or quiz"
	}
	fmt.Printf("claude-board on http://0.0.0.0:%d  (open http://%s:%d/?k=%s on your iPad; auth: %s)\n",
		port, lanIP(), port, tok, gate)
	return http.ListenAndServe(fmt.Sprintf(":%d", port), mux)
}
