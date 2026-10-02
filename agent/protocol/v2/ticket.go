package v2

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"time"
)

const TicketLifetime = 90 * time.Second
const MaxTicketLifetime = 120 * time.Second

var ErrReplay = errors.New("operation ticket already consumed")

// OperationTicket authenticates one exact operation. It contains no credentials
// or raw command/file contents. The node's existing private token is the HMAC key.
type OperationTicket struct {
	AgentEpoch string `json:"agent_epoch"`
	Version    int    `json:"version"`
	Node       string `json:"node"`
	Method     string `json:"method"`
	Action     string `json:"action"`
	RequestID  string `json:"request_id"`
	Actor      string `json:"actor"`
	Digest     string `json:"digest"`
	Nonce      string `json:"nonce"`
	IssuedAt   int64  `json:"issued_at"`
	ExpiresAt  int64  `json:"expires_at"`
	Signature  string `json:"signature"`
}

type ControlPolicy struct {
	Epoch        string   `json:"epoch"`
	Version      int      `json:"version"`
	Node         string   `json:"node"`
	Capabilities []string `json:"capabilities"`
}

func FileAction(op string) string {
	switch op {
	case "list", "list_roots", "stat", "search", "download_stream":
		return "file.read"
	case "create", "mkdir", "delete", "move", "copy", "chmod", "upload_stream", "upload_commit", "upload_cancel":
		return "file.write"
	}
	return ""
}
func IsControlledMethod(method string) bool {
	return method == MethodAgentExec || method == MethodAgentTerminal || method == MethodAgentFile
}

// operationBody strips the ticket and normalizes the JSON field representation
// identically on Hub and Agent before hashing it.
func operationBody(method string, params any) (body any, ticket *OperationTicket, action, id string, err error) {
	raw, e := json.Marshal(params)
	if e != nil {
		err = e
		return
	}
	switch method {
	case MethodAgentExec:
		var p ExecParams
		if err = json.Unmarshal(raw, &p); err != nil {
			return
		}
		ticket = p.Ticket
		p.Ticket = nil
		body = p
		action = "command.exec"
		id = p.TaskID
	case MethodAgentTerminal:
		var p TerminalRequestParams
		if err = json.Unmarshal(raw, &p); err != nil {
			return
		}
		ticket = p.Ticket
		p.Ticket = nil
		body = p
		action = "terminal.open"
		id = p.RequestID
	case MethodAgentFile:
		var p FileOperation
		if err = json.Unmarshal(raw, &p); err != nil {
			return
		}
		ticket = p.Ticket
		p.Ticket = nil
		body = p
		action = FileAction(p.Op)
		id = p.RequestID
	default:
		err = errors.New("method is not a controlled operation")
		return
	}
	if action == "" || id == "" {
		err = errors.New("invalid controlled operation")
	}
	return
}
func signTicket(t OperationTicket, key string) (string, error) {
	t.Signature = ""
	raw, e := json.Marshal(t)
	if e != nil {
		return "", e
	}
	h := hmac.New(sha256.New, []byte(key))
	h.Write(raw)
	return hex.EncodeToString(h.Sum(nil)), nil
}
func digestBody(body any) (string, error) {
	raw, e := json.Marshal(body)
	if e != nil {
		return "", e
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
func SignOperation(node, epoch, actor, key, method string, params any, now time.Time) (any, error) {
	body, _, action, id, e := operationBody(method, params)
	if e != nil {
		return nil, e
	}
	if node == "" || len(epoch) != 32 || actor == "" || len(key) < 12 {
		return nil, errors.New("operation ticket identity or key unavailable")
	}
	if p, ok := body.(FileOperation); ok && p.UUID != node {
		return nil, errors.New("file target differs from ticket node")
	}
	digest, e := digestBody(body)
	if e != nil {
		return nil, e
	}
	var nonce [16]byte
	if _, e = rand.Read(nonce[:]); e != nil {
		return nil, e
	}
	t := &OperationTicket{AgentEpoch: epoch, Version: 1, Node: node, Method: method, Action: action, RequestID: id, Actor: actor, Digest: digest, Nonce: hex.EncodeToString(nonce[:]), IssuedAt: now.UnixMilli(), ExpiresAt: now.Add(TicketLifetime).UnixMilli()}
	t.Signature, e = signTicket(*t, key)
	if e != nil {
		return nil, e
	}
	switch p := body.(type) {
	case ExecParams:
		p.Ticket = t
		return p, nil
	case TerminalRequestParams:
		p.Ticket = t
		return p, nil
	case FileOperation:
		p.Ticket = t
		return p, nil
	}
	return nil, errors.New("unknown operation body")
}
func TicketFromOperation(method string, params any) (*OperationTicket, error) {
	_, t, _, _, e := operationBody(method, params)
	if e != nil {
		return nil, e
	}
	if t == nil {
		return nil, errors.New("operation ticket required")
	}
	return t, nil
}
func VerifyOperation(node, key, method string, params any, now time.Time, minIssued int64) (*OperationTicket, error) {
	body, t, action, id, e := operationBody(method, params)
	if e != nil {
		return nil, e
	}
	if t == nil || len(t.AgentEpoch) != 32 || t.Version != 1 || t.Node != node || t.Method != method || t.Action != action || t.RequestID != id || t.Actor == "" || len(t.Nonce) != 32 || len(key) < 12 {
		return nil, errors.New("invalid operation ticket")
	}
	if p, ok := body.(FileOperation); ok && p.UUID != node {
		return nil, errors.New("file target differs from local node")
	}
	if t.IssuedAt <= minIssued || t.IssuedAt > now.Add(5*time.Second).UnixMilli() || t.ExpiresAt <= now.UnixMilli() || t.ExpiresAt <= t.IssuedAt || t.ExpiresAt-t.IssuedAt > MaxTicketLifetime.Milliseconds() {
		return nil, errors.New("expired or invalid operation ticket time")
	}
	digest, e := digestBody(body)
	if e != nil || digest != t.Digest {
		return nil, errors.New("operation payload does not match ticket")
	}
	signature, e := signTicket(*t, key)
	if e != nil || !hmac.Equal([]byte(signature), []byte(t.Signature)) {
		return nil, errors.New("invalid operation ticket signature")
	}
	return t, nil
}

// TicketVerifier serializes consumption across WS/pull deliveries. Expired
// entries are pruned; unexpired entries are never evicted to make space.
// A random process epoch rejects tickets from previous runs, even if the
// system clock moves backwards. The startup time check is an additional bound.
type TicketVerifier struct {
	mu        sync.Mutex
	seen      map[string]int64
	startedAt int64
	epoch     string
}

func NewTicketVerifier(start time.Time) *TicketVerifier {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		panic("cannot initialize operation epoch")
	}
	return &TicketVerifier{epoch: hex.EncodeToString(random[:]), seen: make(map[string]int64), startedAt: start.UnixMilli()}
}
func (v *TicketVerifier) Epoch() string { return v.epoch }
func (v *TicketVerifier) Consume(node, key, method string, params any, now time.Time) error {
	t, e := VerifyOperation(node, key, method, params, now, v.startedAt)
	if e != nil {
		return e
	}
	if t.AgentEpoch != v.epoch {
		return errors.New("operation ticket belongs to a previous Agent process")
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	for nonce, expires := range v.seen {
		if expires <= now.UnixMilli() {
			delete(v.seen, nonce)
		}
	}
	if _, ok := v.seen[t.Nonce]; ok {
		return ErrReplay
	}
	if len(v.seen) >= 8192 {
		return errors.New("operation replay cache is full")
	}
	v.seen[t.Nonce] = t.ExpiresAt
	return nil
}
