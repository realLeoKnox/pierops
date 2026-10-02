package agent

import (
	"context"
	"testing"
	"time"

	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/internal/access"
	"github.com/komari-monitor/komari/pkg/rpc"
	v2 "github.com/komari-monitor/komari/protocol/v2"
)

func TestControlPolicyRefusesOlderAndWrongNode(t *testing.T) {
	node := "policy-test-node"
	KeepAlivePresence(node, 1, time.Minute)
	defer DeleteLatestReport(node)
	RecordReport(v2.Report{UUID: node, UpdatedAt: time.Now()})
	if CanControl(node, access.FileRead) {
		t.Fatal("old Agent allowed")
	}
	RecordReport(v2.Report{UUID: node, UpdatedAt: time.Now(), ControlPolicy: &v2.ControlPolicy{Epoch: "00112233445566778899aabbccddeeff", Version: 2, Node: "wrong-node", Capabilities: []string{access.FileRead}}})
	if CanControl(node, access.FileRead) {
		t.Fatal("wrong node allowed")
	}
	RecordReport(v2.Report{UUID: node, UpdatedAt: time.Now(), ControlPolicy: &v2.ControlPolicy{Epoch: "00112233445566778899aabbccddeeff", Version: 2, Node: node, Capabilities: []string{access.FileRead}}})
	if !CanControl(node, access.FileRead) || CanControl(node, access.FileWrite) {
		t.Fatal("local read/write capability not enforced")
	}
	ClearControlPolicy(node)
	if CanControl(node, access.FileRead) {
		t.Fatal("reconnect retained old policy")
	}
	if DispatchV2Event(node, v2.MethodAgentExec, v2.ExecParams{TaskID: "unsigned", Command: "printf ok"}) {
		t.Fatal("generic unsigned dispatch allowed")
	}
}
func TestControlQueueRechecksRevocationAndCancellation(t *testing.T) {
	flags.DatabaseType = flags.DatabaseTypeSQLite
	flags.DatabaseFile = "file:control-ticket-test?mode=memory&cache=shared"
	db := dbcore.GetDBInstance()

	node := "queued-test-node"
	key := "test-only-node-key-0000000000"
	db.Create(&models.Client{UUID: node, Token: key})
	db.Create(&models.User{UUID: "queued-user", Username: "queue-operator", AccessRole: access.Operator, Passwd: "unused"})
	db.Create(&models.Session{UUID: "queued-user", Session: "queue-test-session", Expires: time.Now().Add(time.Hour)})
	grant := models.AccessGrant{UserUUID: "queued-user", ClientUUID: node, Action: access.FileRead}
	db.Create(&grant)
	MarkV2Client(node)
	KeepAlivePresence(node, 1, time.Minute)
	RecordReport(v2.Report{UUID: node, UpdatedAt: time.Now(), ControlPolicy: &v2.ControlPolicy{Epoch: "00112233445566778899aabbccddeeff", Version: 2, Node: node, Capabilities: []string{access.FileRead}}})
	defer func() { DeleteLatestReport(node); v2EventMu.Lock(); delete(v2EventQueues, node); v2EventMu.Unlock() }()
	ctx := rpc.NewContextWithMeta(context.Background(), &rpc.ContextMeta{Principal: rpc.NewUserPrincipal("queued-user"), SessionToken: "queue-test-session"})
	prepare := func(id string) *PreparedOperation {
		p, e := PrepareOperation(ctx, node, v2.MethodAgentFile, v2.FileOperation{UUID: node, RequestID: id, Op: "stat", Args: map[string]any{"path": "/srv/work/file"}})
		if e != nil {
			t.Fatal(e)
		}
		if !p.Dispatch() {
			t.Fatal("queue dispatch failed")
		}
		return p
	}
	first := prepare("file-a")
	events := TakeV2Events(node, nil, 0)
	if len(events) != 1 {
		t.Fatalf("queued events %d", len(events))
	}
	if _, err := v2.VerifyOperation(node, key, events[0].Method, events[0].Params, time.Now(), 0); err != nil {
		t.Fatal(err)
	}
	first.CancelQueued()
	if len(TakeV2Events(node, nil, 0)) != 0 {
		t.Fatal("canceled queue entry delivered")
	}
	prepare("file-b")
	db.Delete(&grant)
	if len(TakeV2Events(node, nil, 0)) != 0 {
		t.Fatal("revoked queued operation delivered")
	}
	db.Create(&grant)
	if len(TakeV2Events(node, nil, 0)) != 0 {
		t.Fatal("revoked queue entry returned after regrant")
	}
	prepare("file-c")
	db.Model(&models.Client{}).Where("uuid = ?", node).Update("token", key+"rotated")
	if len(TakeV2Events(node, nil, 0)) != 0 {
		t.Fatal("old-key ticket delivered after rotation")
	}

}
