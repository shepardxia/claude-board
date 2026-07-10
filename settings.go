package main

// Manage the PostToolUse hook in ~/.claude/settings.json.
//
// The hook is a plain curl that pipes each tool call's raw JSON to the server:
//   curl -s -m 1 -X POST --data-binary @- http://127.0.0.1:<port>/push
// Editing keeps the top-level key order and leaves every untouched section
// byte-for-byte intact; only the "hooks" block is rewritten.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

const hookMark = "/push" // substring identifying our curl hook

func hookCommand(port int) string {
	return fmt.Sprintf("curl -s -m 1 -X POST --data-binary @- http://127.0.0.1:%d/push", port)
}

// topKeyOrder returns object keys in source order (Go maps lose it).
func topKeyOrder(raw []byte) []string {
	dec := json.NewDecoder(bytes.NewReader(raw))
	if _, err := dec.Token(); err != nil { // opening '{'
		return nil
	}
	var ks []string
	for dec.More() {
		kt, err := dec.Token()
		if err != nil {
			break
		}
		ks = append(ks, kt.(string))
		var skip json.RawMessage
		_ = dec.Decode(&skip)
	}
	return ks
}

// renderTop emits a 2-space object in the given key order; each value is taken
// verbatim from vals and re-indented to sit at depth 1.
func renderTop(order []string, vals map[string]json.RawMessage) []byte {
	var b bytes.Buffer
	b.WriteString("{")
	n := 0
	for _, k := range order {
		v, ok := vals[k]
		if !ok {
			continue
		}
		if n > 0 {
			b.WriteString(",")
		}
		n++
		b.WriteString("\n  ")
		kb, _ := json.Marshal(k)
		b.Write(kb)
		b.WriteString(": ")
		var iv bytes.Buffer
		if json.Indent(&iv, v, "  ", "  ") == nil {
			b.Write(iv.Bytes())
		} else {
			b.Write(v)
		}
	}
	if n > 0 {
		b.WriteString("\n")
	}
	b.WriteString("}\n")
	return b.Bytes()
}

// editHooks loads settings.json, hands the PostToolUse list to mut, and writes
// it back preserving everything else. mut returns the new list.
func editHooks(mut func(ptu []any) []any) error {
	raw, _ := os.ReadFile(settingsPath())
	top := map[string]json.RawMessage{}
	var order []string
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, &top); err != nil {
			return fmt.Errorf("settings.json is not valid JSON: %w", err)
		}
		order = topKeyOrder(raw)
	}

	hooks := map[string]any{}
	if hb, ok := top["hooks"]; ok {
		_ = json.Unmarshal(hb, &hooks)
	}
	var ptu []any
	if pv, ok := hooks["PostToolUse"].([]any); ok {
		ptu = pv
	}
	hooks["PostToolUse"] = mut(ptu)

	hb := jmarshal(hooks, false)
	top["hooks"] = hb
	found := false
	for _, k := range order {
		if k == "hooks" {
			found = true
			break
		}
	}
	if !found {
		order = append(order, "hooks")
	}

	out := renderTop(order, top)
	dir := claudeDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if len(raw) > 0 {
		_ = os.WriteFile(settingsPath()+".bak", raw, 0o644)
	}
	return os.WriteFile(settingsPath(), out, 0o644)
}

func findStarEntry(ptu []any) map[string]any {
	for _, e := range ptu {
		if em, ok := e.(map[string]any); ok {
			m, has := em["matcher"]
			if !has || m == "*" {
				return em
			}
		}
	}
	return nil
}

func installHook(port int) error {
	cmd := hookCommand(port)
	return editHooks(func(ptu []any) []any {
		entry := findStarEntry(ptu)
		if entry == nil {
			entry = map[string]any{"matcher": "*", "hooks": []any{}}
			ptu = append(ptu, entry)
		}
		hl, _ := entry["hooks"].([]any)
		found := false
		for _, h := range hl {
			if hm, ok := h.(map[string]any); ok {
				if s, _ := hm["command"].(string); containsSub(s, hookMark) {
					hm["command"] = cmd
					found = true
				}
			}
		}
		if !found {
			hl = append(hl, map[string]any{"type": "command", "command": cmd, "async": true})
		}
		entry["hooks"] = hl
		return ptu
	})
}

func uninstallHook() (int, error) {
	removed := 0
	err := editHooks(func(ptu []any) []any {
		var keptEntries []any
		for _, e := range ptu {
			em, ok := e.(map[string]any)
			if !ok {
				keptEntries = append(keptEntries, e)
				continue
			}
			hl, _ := em["hooks"].([]any)
			var kept []any
			for _, h := range hl {
				hm, ok := h.(map[string]any)
				if ok {
					if s, _ := hm["command"].(string); containsSub(s, hookMark) {
						removed++
						continue
					}
				}
				kept = append(kept, h)
			}
			em["hooks"] = kept
			if len(kept) == 0 {
				continue // drop now-empty entry
			}
			keptEntries = append(keptEntries, em)
		}
		return keptEntries
	})
	return removed, err
}

func hookInstalled() bool {
	raw, err := os.ReadFile(settingsPath())
	if err != nil {
		return false
	}
	var s struct {
		Hooks struct {
			PostToolUse []struct {
				Hooks []struct {
					Command string `json:"command"`
				} `json:"hooks"`
			} `json:"PostToolUse"`
		} `json:"hooks"`
	}
	if json.Unmarshal(raw, &s) != nil {
		return false
	}
	for _, e := range s.Hooks.PostToolUse {
		for _, h := range e.Hooks {
			if containsSub(h.Command, hookMark) {
				return true
			}
		}
	}
	return false
}

func containsSub(s, sub string) bool { return bytes.Contains([]byte(s), []byte(sub)) }
