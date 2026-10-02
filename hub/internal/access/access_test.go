package access

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/pkg/rpc"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func testService(t *testing.T) *Service {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "access.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	conn, _ := db.DB()
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.AutoMigrate(&models.User{}, &models.Session{}, &models.Client{}, &models.AccessGrant{}, &models.OperationAudit{}); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{Owner, Operator, Viewer} {
		if err := db.Create(&models.User{UUID: role, Username: role, Passwd: "test-only", AccessRole: role}).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&models.Session{UUID: role, Session: "test-session-" + role, Expires: time.Now().Add(time.Hour)}).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, node := range []string{"node-a", "node-b"} {
		if err := db.Create(&models.Client{UUID: node, Token: "test-only-" + node, Name: node}).Error; err != nil {
			t.Fatal(err)
		}
	}
	return &Service{DB: db}
}

func testMeta(role string) *rpc.ContextMeta {
	return &rpc.ContextMeta{Principal: rpc.NewUserPrincipal(role), SessionToken: "test-session-" + role}
}

func TestNodeActionRoleAndCredentialBoundaries(t *testing.T) {
	s := testService(t)
	if err := s.ReplacePolicy(testMeta(Owner), Operator, Operator, []Grant{{"node-a", FileRead}, {"node-a", Terminal}}); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		role, action, node string
		allowed            bool
	}{
		{Owner, Manage, "", true}, {Operator, FileRead, "node-a", true}, {Operator, FileRead, "node-b", false},
		{Operator, FileWrite, "node-a", false}, {Operator, Manage, "", false}, {Viewer, Terminal, "node-a", false},
	} {
		if got := s.Check(testMeta(c.role), c.action, c.node) == nil; got != c.allowed {
			t.Errorf("%s %s %s allowed=%v", c.role, c.action, c.node, got)
		}
	}
	if s.Check(&rpc.ContextMeta{Principal: rpc.NewAgentPrincipal("node-a")}, FileRead, "node-a") == nil {
		t.Fatal("Agent acquired operator permissions")
	}
	s.DB.Model(&models.Session{}).Where("uuid = ?", Operator).Update("expires", time.Now().Add(-time.Minute))
	if s.Check(testMeta(Operator), FileRead, "node-a") == nil {
		t.Fatal("Expired session accepted")
	}
	s.DB.Model(&models.Session{}).Where("uuid = ?", Operator).Update("expires", time.Now().Add(time.Hour))
	if err := s.ReplacePolicy(testMeta(Owner), Operator, Operator, nil); err != nil {
		t.Fatal(err)
	}
	if s.Check(testMeta(Operator), Terminal, "node-a") == nil {
		t.Fatal("Revoked grant accepted on cached identity")
	}
	if err := s.ReplacePolicy(testMeta(Owner), Operator, Disabled, nil); err != nil {
		t.Fatal(err)
	}
	if s.Check(testMeta(Operator), NodeList, "") == nil {
		t.Fatal("Disabled user accepted")
	}
}

func TestAPIKeyRotationInvalidatesCachedIdentity(t *testing.T) {
	s := testService(t)
	key := "test-only-api-key-first"
	s.APIKey = func() string { return key }
	meta := &rpc.ContextMeta{Principal: rpc.NewAPIKeyPrincipal(), APIKeyToken: key}
	if s.Check(meta, Manage, "") != nil {
		t.Fatal("Valid API key rejected")
	}
	key = "test-only-api-key-replaced"
	if s.Check(meta, Manage, "") == nil {
		t.Fatal("Rotated API key accepted")
	}
}

func TestPolicyValidationAtomicityAndCredentialRedaction(t *testing.T) {
	s := testService(t)
	owner := testMeta(Owner)
	grants := []Grant{{"node-a", FileRead}}
	if err := s.ReplacePolicy(owner, Viewer, Viewer, grants); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []struct {
		role   string
		grants []Grant
	}{
		{Viewer, []Grant{{"node-a", FileWrite}}}, {Operator, []Grant{{"*", Terminal}}},
		{Operator, []Grant{{"node-a", "*"}}}, {Operator, []Grant{{"node-a", FileRead}, {"node-a", FileRead}}},
		{Disabled, grants}, {Owner, nil},
	} {
		if s.ReplacePolicy(owner, Viewer, bad.role, bad.grants) == nil {
			t.Fatal("Invalid policy accepted")
		}
	}
	policy, err := s.GetPolicy(Viewer)
	if err != nil || policy.User.AccessRole != Viewer || len(policy.Grants) != 1 {
		t.Fatalf("Existing policy lost after rejection: %v", err)
	}
	if s.ReplacePolicy(owner, Owner, Disabled, nil) == nil {
		t.Fatal("Installation owner disabled")
	}
	if s.ReplacePolicy(testMeta(Operator), Viewer, Operator, nil) == nil {
		t.Fatal("Operator edited another user's permissions")
	}
	password := "test-only-long-password"
	id, err := s.CreateUser(owner, "limited", password, Viewer, grants)
	if err != nil {
		t.Fatal(err)
	}
	var user models.User
	s.DB.Where("uuid = ?", id).First(&user)
	if bcrypt.CompareHashAndPassword([]byte(user.Passwd), []byte(password)) != nil {
		t.Fatal("Password was not bcrypt hashed")
	}
	var audit models.OperationAudit
	s.DB.Order("id desc").First(&audit)
	if audit.Action != "access.policy" || audit.Details == "" {
		t.Fatal("Policy change audit missing")
	}
}

func TestRPCScopePreflightAndPositionalParameters(t *testing.T) {
	s := testService(t)
	if err := s.ReplacePolicy(testMeta(Owner), Operator, Operator, []Grant{{"node-a", CommandExec}, {"node-a", CommandRead}}); err != nil {
		t.Fatal(err)
	}
	meta := testMeta(Operator)
	for _, params := range []any{
		map[string]any{"command": "test-only-command", "clients": []string{"node-a", "node-b"}},
		[]any{"test-only-command", []string{"node-a", "node-b"}},
	} {
		if s.AuthorizeRPC(meta, &rpc.JsonRpcRequest{Method: "admin:exec", Params: params}) == nil {
			t.Fatal("Mixed-scope batch allowed")
		}
	}
	var count int64
	s.DB.Model(&models.OperationAudit{}).Where("outcome = ?", "allowed").Count(&count)
	if count != 0 {
		t.Fatal("Partial execution authorization recorded")
	}
	// First positional field is task_id; authorizing it as a node would bypass scope.
	if s.AuthorizeRPC(meta, &rpc.JsonRpcRequest{Method: "admin:getSpecificTaskResult", Params: []any{"node-a", "node-b"}}) == nil {
		t.Fatal("Task ID confused with target node")
	}
	if s.AuthorizeRPC(meta, &rpc.JsonRpcRequest{Method: "admin:exec", Params: []any{"test-only-command", []string{"node-a"}}}) != nil {
		t.Fatal("Valid positional exec denied")
	}
	for _, method := range []string{"admin:getClientToken", "admin:getTasks", "admin:editSettings", "plugin:privileged"} {
		if s.AuthorizeRPC(meta, &rpc.JsonRpcRequest{Method: method}) == nil {
			t.Fatal("Unlisted privileged method allowed")
		}
	}
}

func TestMissingAuditStoreFailsClosed(t *testing.T) {
	s := testService(t)
	if err := s.DB.Migrator().DropTable(&models.OperationAudit{}); err != nil {
		t.Fatal(err)
	}
	if s.Authorize(testMeta(Owner), Manage, "") == nil {
		t.Fatal("Dispatch allowed without durable audit")
	}
}

func TestLegacyOwnerMigration(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "legacy.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	conn, _ := db.DB()
	defer conn.Close()
	if db.Exec("CREATE TABLE users (uuid varchar(36) PRIMARY KEY, username varchar(50) NOT NULL UNIQUE, passwd varchar(255) NOT NULL)").Error != nil {
		t.Fatal("Cannot create legacy schema")
	}
	if db.Exec("INSERT INTO users (uuid, username, passwd) VALUES (?, ?, ?)", "legacy-owner", "legacy-owner", "test-only").Error != nil {
		t.Fatal("Cannot create legacy user")
	}
	if err := db.AutoMigrate(&models.User{}); err != nil {
		t.Fatal(err)
	}
	var user models.User
	if db.First(&user, "uuid = ?", "legacy-owner").Error != nil || user.AccessRole != Owner {
		t.Fatal("Existing installation owner lost access after migration")
	}
}
