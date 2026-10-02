package v2

import (
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const fixtureKey = "ticket-test-only-key-00000000"

func TestTicketBindingExpiryAndRestart(t *testing.T) {
	now := time.UnixMilli(1800000000000)
	params, err := SignOperation("node-a", "operator-a", fixtureKey, MethodAgentFile, FileOperation{UUID: "node-a", RequestID: "request-a", Op: "download_stream", Args: map[string]any{"path": "/srv/work/example.txt", "offset": int64(0), "length": int64(5)}}, now)
	if err != nil {
		t.Fatal(err)
	}
	// The generic JSON maps used by both transports preserve the signature.
	raw, _ := json.Marshal(params)
	var received any
	if err := json.Unmarshal(raw, &received); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyOperation("node-a", fixtureKey, MethodAgentFile, received, now, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyOperation("node-b", fixtureKey, MethodAgentFile, received, now, 0); err == nil {
		t.Fatal("wrong node accepted")
	}
	if _, err := VerifyOperation("node-a", fixtureKey+"different", MethodAgentFile, received, now, 0); err == nil {
		t.Fatal("wrong key accepted")
	}
	if _, err := VerifyOperation("node-a", fixtureKey, MethodAgentFile, received, now.Add(91*time.Second), 0); err == nil {
		t.Fatal("expired ticket accepted")
	}
	if _, err := VerifyOperation("node-a", fixtureKey, MethodAgentFile, received, now.Add(-6*time.Second), 0); err == nil {
		t.Fatal("future ticket accepted")
	}
	restarted := NewTicketVerifier(now.Add(time.Millisecond))
	if err := restarted.Consume("node-a", fixtureKey, MethodAgentFile, received, now.Add(time.Second)); err == nil {
		t.Fatal("pre-restart ticket accepted")
	}
	received.(map[string]any)["args"].(map[string]any)["path"] = "/srv/work/other.txt"
	if _, err := VerifyOperation("node-a", fixtureKey, MethodAgentFile, received, now, 0); err == nil {
		t.Fatal("payload substitution accepted")
	}
}
func TestTicketConcurrentReplayAndUnsigned(t *testing.T) {
	now := time.UnixMilli(1800000000000)
	v := NewTicketVerifier(now.Add(-time.Second))
	p, e := SignOperation("node-a", "owner-a", fixtureKey, MethodAgentExec, ExecParams{TaskID: "task-a", Command: "printf ok"}, now)
	if e != nil {
		t.Fatal(e)
	}
	var accepted atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := v.Consume("node-a", fixtureKey, MethodAgentExec, p, now)
			if err == nil {
				accepted.Add(1)
			} else if !errors.Is(err, ErrReplay) {
				t.Errorf("unexpected error %v", err)
			}
		}()
	}
	wg.Wait()
	if accepted.Load() != 1 {
		t.Fatalf("consumed %d times", accepted.Load())
	}
	if _, e := VerifyOperation("node-a", fixtureKey, MethodAgentExec, ExecParams{TaskID: "unsigned", Command: "printf ok"}, now, 0); e == nil {
		t.Fatal("unsigned operation accepted")
	}
}
func TestTicketBindsAllControlledMethods(t *testing.T) {
	now := time.UnixMilli(1800000000000)
	for _, entry := range []struct {
		method string
		params any
	}{
		{MethodAgentExec, ExecParams{TaskID: "t", Command: "printf ok"}},
		{MethodAgentTerminal, TerminalRequestParams{RequestID: "terminal-a"}},
		{MethodAgentFile, FileOperation{UUID: "node-a", RequestID: "f", Op: "mkdir", Args: map[string]any{"path": "/srv/work/a", "mode": "0755"}}},
	} {
		p, e := SignOperation("node-a", "actor", fixtureKey, entry.method, entry.params, now)
		if e != nil {
			t.Fatal(e)
		}
		if _, e := VerifyOperation("node-a", fixtureKey, entry.method, p, now, 0); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := SignOperation("node-a", "actor", fixtureKey, MethodAgentFile, FileOperation{UUID: "node-a", RequestID: "f", Op: "chown"}, now); e == nil {
		t.Fatal("disabled action signed")
	}
}
