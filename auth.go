package main

// Optional "challenge" gate: a tappable multiple-choice quiz that unlocks the
// board and drops a session cookie, so you can bookmark the plain URL (no token
// in it) and just tap through on the iPad. The strong token URL still works.
//
// This is convenience, not real security: a few multiple-choice questions are
// guessable/brute-forceable by someone already on your LAN. A small delay per
// wrong answer slows that down. Enable only if you accept the tradeoff; the
// token remains the default. Configure via ~/.claude-board/challenge.json.

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const cookieName = "cb"

type question struct {
	Q       string   `json:"q"`
	Options []string `json:"options"`
	Answer  int      `json:"answer"` // 0-based index of the correct option
}

type challenge struct {
	Questions []question `json:"questions"`
}

func challengePath() string { return filepath.Join(stateDir(), "challenge.json") }

// loadChallenge returns the configured quiz, or nil if none/invalid (→ token-only).
func loadChallenge() *challenge {
	b, err := os.ReadFile(challengePath())
	if err != nil {
		return nil
	}
	var c challenge
	if json.Unmarshal(b, &c) != nil || len(c.Questions) == 0 {
		return nil
	}
	for _, q := range c.Questions {
		if q.Answer < 0 || q.Answer >= len(q.Options) {
			return nil
		}
	}
	return &c
}

func cookieOK(r *http.Request, tok string) bool {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(c.Value), []byte(tok)) == 1
}

// authed accepts either the token in the URL (?k=) or a valid session cookie.
func authed(r *http.Request, tok string) bool {
	return tokenOK(r, tok) || cookieOK(r, tok)
}

func setSession(w http.ResponseWriter, tok string) {
	http.SetCookie(w, &http.Cookie{
		Name: cookieName, Value: tok, Path: "/", HttpOnly: true,
		SameSite: http.SameSiteLaxMode, MaxAge: 365 * 24 * 3600,
	})
}

func quizPage(ch *challenge, wrong bool) string {
	var b strings.Builder
	b.WriteString(`<!doctype html><html lang=en><head><meta charset=utf-8>
<meta name=viewport content="width=device-width, initial-scale=1, viewport-fit=cover">
<title>Claude Board</title>
<style>
:root{--bg:#eceae3;--ink:#0b0b0b;--faint:#7b786f;--fail:#c8352a;--mono:ui-monospace,Menlo,monospace}
*{box-sizing:border-box}html,body{margin:0;height:100%}
body{background:var(--bg);color:var(--ink);font-family:var(--mono);display:flex;align-items:center;justify-content:center;min-height:100dvh;padding:24px}
form{width:100%;max-width:520px}
h1{font-size:18px;font-weight:600;margin:0 0 4px}
.hint{color:var(--faint);font-size:13px;margin:0 0 24px}
.q{margin:0 0 22px}
.qt{font-size:16px;margin:0 0 10px}
.opts{display:flex;flex-wrap:wrap;gap:10px}
label{border:1px solid var(--ink);padding:12px 16px;font-size:16px;cursor:pointer;flex:1 1 auto;text-align:center}
label:has(input:checked){background:var(--ink);color:var(--bg)}
input{position:absolute;opacity:0;pointer-events:none}
button{margin-top:8px;width:100%;border:1px solid var(--ink);background:var(--ink);color:var(--bg);font-family:var(--mono);font-size:16px;padding:14px;cursor:pointer}
.err{color:var(--fail);font-size:14px;margin:0 0 16px}
</style></head><body>
<form method=POST action="/auth">
<h1>Claude Board</h1>
<p class=hint>answer to view</p>
`)
	if wrong {
		b.WriteString(`<p class=err>not quite — try again</p>`)
	}
	for i, q := range ch.Questions {
		fmt.Fprintf(&b, `<div class=q><p class=qt>%s</p><div class=opts>`, htmlEsc(q.Q))
		for j, opt := range q.Options {
			fmt.Fprintf(&b, `<label><input type=radio name="q%d" value="%d">%s</label>`,
				i, j, htmlEsc(opt))
		}
		b.WriteString(`</div></div>`)
	}
	b.WriteString(`<button type=submit>enter</button></form></body></html>`)
	return b.String()
}

func htmlEsc(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}

// serveChallenge renders the quiz (GET) for an unauthenticated visitor.
func serveChallenge(w http.ResponseWriter, ch *challenge, wrong bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if wrong {
		w.WriteHeader(http.StatusUnauthorized)
	}
	fmt.Fprint(w, quizPage(ch, wrong))
}

// handleAuth checks submitted answers; on success sets the cookie and redirects.
func handleAuth(w http.ResponseWriter, r *http.Request, ch *challenge, tok string) {
	if r.Method != http.MethodPost {
		w.WriteHeader(405)
		return
	}
	_ = r.ParseForm()
	ok := true
	for i, q := range ch.Questions {
		if r.FormValue("q"+strconv.Itoa(i)) != strconv.Itoa(q.Answer) {
			ok = false
			break
		}
	}
	if !ok {
		time.Sleep(1 * time.Second) // slow brute force
		serveChallenge(w, ch, true)
		return
	}
	setSession(w, tok)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func challengeConfigured() bool { return loadChallenge() != nil }

// exampleChallenge is scaffolded by `claude-board challenge` for the user to edit.
const exampleChallenge = `{
  "questions": [
    { "q": "What is 6 x 7?",
      "options": ["36", "67", "48", "13", "42"], "answer": 1 }
  ]
}
`
