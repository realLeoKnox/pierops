package agent

import (
	"context"
	"errors"
	"time"

	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/internal/access"
	"github.com/komari-monitor/komari/pkg/rpc"
	v2 "github.com/komari-monitor/komari/protocol/v2"
)

var ErrLocalPolicy = errors.New("Agent must report PierOps M2 support and enable the requested local capability")

type PreparedOperation struct {
	node, method, action string
	params               any
	meta                 *rpc.ContextMeta
}

// CanControl deliberately refuses older Agents even when they accept v2 RPC.
// Agent reporting is an authenticated compatibility declaration, not a remote
// substitute for the independent policy checks performed on the node.
func CanControl(node, action string) bool {
	mu.RLock()
	defer mu.RUnlock()
	report := latestReport[node]
	if report == nil || report.ControlPolicy == nil || report.ControlPolicy.Version != 2 || len(report.ControlPolicy.Epoch) != 32 || report.ControlPolicy.Node != node {
		return false
	}
	online := connectedClients[node] != nil
	if presence, ok := presenceOnly[node]; ok && presence.expire.After(time.Now()) {
		online = true
	}
	if !online {
		return false
	}
	for _, capability := range report.ControlPolicy.Capabilities {
		if capability == action {
			return true
		}
	}
	return false
}
func ClearControlPolicy(node string) {
	mu.Lock()
	defer mu.Unlock()
	if report := latestReport[node]; report != nil {
		copy := *report
		copy.ControlPolicy = nil
		latestReport[node] = &copy
	}
}
func PrepareOperation(ctx context.Context, node, method string, params any) (*PreparedOperation, error) {
	var action string
	switch method {
	case v2.MethodAgentExec:
		action = access.CommandExec
	case v2.MethodAgentTerminal:
		action = access.Terminal
	case v2.MethodAgentFile:
		var p v2.FileOperation
		if err := bindV2EventParams(params, &p); err != nil {
			return nil, err
		}
		action = v2.FileAction(p.Op)
	}
	meta := rpc.MetaFromContext(ctx)
	if action == "" || access.Default().Authorize(meta, action, node) != nil {
		return nil, access.ErrDenied
	}
	if !CanControl(node, action) {
		return nil, ErrLocalPolicy
	}
	var client models.Client
	err := dbcore.GetDBInstance().Select("uuid", "token").Where("uuid = ?", node).First(&client).Error
	if err != nil {
		return nil, errors.New("node unavailable")
	}
	mu.RLock()
	report := latestReport[node]
	epoch := ""
	if report != nil && report.ControlPolicy != nil {
		epoch = report.ControlPolicy.Epoch
	}
	mu.RUnlock()
	if len(epoch) != 32 {
		return nil, ErrLocalPolicy
	}
	signed, err := v2.SignOperation(node, epoch, access.Actor(meta), client.Token, method, params, time.Now())
	if err != nil {
		return nil, err
	}
	return &PreparedOperation{node: node, method: method, action: action, params: signed, meta: meta}, nil
}
func (p *PreparedOperation) Dispatch() bool {
	if p == nil || access.Default().Check(p.meta, p.action, p.node) != nil || !CanControl(p.node, p.action) || !validCurrentTicket(p.node, p.method, p.params) {
		return false
	}
	return dispatchV2Event(p.node, p.method, p.params, p.meta)
}
func (p *PreparedOperation) CancelQueued() {
	if p == nil {
		return
	}
	ticket, err := v2.TicketFromOperation(p.method, p.params)
	if err != nil {
		return
	}
	v2EventMu.Lock()
	defer v2EventMu.Unlock()
	q := v2EventQueues[p.node]
	if q == nil {
		return
	}
	events := q.events[:0]
	for _, event := range q.events {
		t, e := v2.TicketFromOperation(event.Method, event.Params)
		if e == nil && t.Nonce == ticket.Nonce {
			delete(q.authorization, event.ID)
			continue
		}
		events = append(events, event)
	}
	q.events = events
}
func validCurrentTicket(node, method string, params any) bool {
	var client models.Client
	if dbcore.GetDBInstance().Select("token").Where("uuid = ?", node).First(&client).Error != nil {
		return false
	}
	ticket, err := v2.VerifyOperation(node, client.Token, method, params, time.Now(), 0)
	if err != nil {
		return false
	}
	mu.RLock()
	report := latestReport[node]
	compatible := report == nil || report.ControlPolicy == nil || report.ControlPolicy.Epoch == ticket.AgentEpoch
	mu.RUnlock()
	return compatible
}
func queuedOperationState(q *v2EventQueue, event v2.Event) (allowed, discard bool) {
	if !v2.IsControlledMethod(event.Method) {
		return true, false
	}
	ticket, err := v2.TicketFromOperation(event.Method, event.Params)
	if err != nil || ticket.Node != q.node || !validCurrentTicket(q.node, event.Method, event.Params) {
		return false, true
	}
	meta := q.authorization[event.ID]
	if meta == nil {
		return false, true
	}
	if access.Default().Check(meta, ticket.Action, q.node) != nil {
		_ = access.Default().Audit(meta, ticket.Action, q.node, "denied", "queued_permission_revoked")
		return false, true
	}
	if CanControl(q.node, ticket.Action) {
		return true, false
	}
	// Reconnect requires a fresh report; retain tickets briefly while waiting.
	// An actual report disabling this capability cancels them permanently.
	mu.RLock()
	report := latestReport[q.node]
	incompatible := report != nil && report.ControlPolicy != nil
	mu.RUnlock()
	return false, incompatible
}
