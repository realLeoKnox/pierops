// Package access implements transport-independent user/node/action authorization.
package access

import (
	"crypto/subtle"
	"errors"
	"time"

	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/internal/config"
	"github.com/komari-monitor/komari/pkg/rpc"
	"gorm.io/gorm"
)

const (
	Owner       = "owner"
	Operator    = "operator"
	Viewer      = "viewer"
	Disabled    = "disabled"
	Manage      = "hub.manage"
	NodeRead    = "node.read"
	Terminal    = "terminal.open"
	FileRead    = "file.read"
	FileWrite   = "file.write"
	CommandExec = "command.exec"
	CommandRead = "command.read"
	DockerRead  = "docker.read"
	NodeList    = "node.list"
	Self        = "access.self"
)

var ErrDenied = errors.New("operation not permitted")

type Subject struct{ ID, Role string }
type Service struct {
	DB     *gorm.DB
	APIKey func() string
}

func Default() *Service {
	return &Service{DB: dbcore.GetDBInstance(), APIKey: func() string {
		key, _ := config.GetAs[string](config.ApiKeyKey, "")
		return key
	}}
}

// Resolve rereads credentials and role on every operation, including WS messages.
func (s *Service) Resolve(meta *rpc.ContextMeta) (Subject, error) {
	if meta == nil || meta.Principal == nil {
		return Subject{}, ErrDenied
	}
	p := meta.Principal
	switch p.Type {
	case rpc.PrincipalInternal:
		return Subject{"internal", Owner}, nil
	case rpc.PrincipalAPIKey:
		key := ""
		if s.APIKey != nil {
			key = s.APIKey()
		}
		if len(key) < 12 || subtle.ConstantTimeCompare([]byte(key), []byte(meta.APIKeyToken)) != 1 {
			return Subject{}, ErrDenied
		}
		return Subject{"api-key", Owner}, nil
	case rpc.PrincipalUser:
		if p.UserUUID == "" || meta.SessionToken == "" {
			return Subject{}, ErrDenied
		}
		var session models.Session
		if err := s.DB.Where("session = ? AND uuid = ?", meta.SessionToken, p.UserUUID).First(&session).Error; err != nil {
			return Subject{}, ErrDenied
		}
		if !session.Expires.After(time.Now()) {
			return Subject{}, ErrDenied
		}
		var user models.User
		if err := s.DB.Select("uuid", "access_role").Where("uuid = ?", p.UserUUID).First(&user).Error; err != nil {
			return Subject{}, ErrDenied
		}
		if user.AccessRole != Owner && user.AccessRole != Operator && user.AccessRole != Viewer {
			return Subject{p.UserUUID, Disabled}, ErrDenied
		}
		return Subject{user.UUID, user.AccessRole}, nil
	default:
		return Subject{}, ErrDenied
	}
}

func Grantable(role, action string) bool {
	switch action {
	case NodeRead, FileRead, CommandRead, DockerRead:
		return role == Operator || role == Viewer
	case Terminal, FileWrite, CommandExec:
		return role == Operator
	default:
		return false
	}
}

func (s *Service) Check(meta *rpc.ContextMeta, action, node string) error {
	subject, err := s.Resolve(meta)
	if err != nil {
		return err
	}
	if subject.Role == Owner {
		return nil
	}
	if action == NodeList || action == Self {
		return nil
	}
	if node == "" || !Grantable(subject.Role, action) {
		return ErrDenied
	}
	var count int64
	if err := s.DB.Model(&models.AccessGrant{}).Where("user_uuid = ? AND client_uuid = ? AND action = ?", subject.ID, node, action).Count(&count).Error; err != nil {
		return ErrDenied
	}
	if count == 0 {
		return ErrDenied
	}
	return nil
}

func Actor(meta *rpc.ContextMeta) string {
	if meta == nil || meta.Principal == nil {
		return "anonymous"
	}
	switch meta.Principal.Type {
	case rpc.PrincipalUser:
		return meta.Principal.UserUUID
	case rpc.PrincipalAPIKey:
		return "api-key"
	case rpc.PrincipalInternal:
		return "internal"
	case rpc.PrincipalAgent:
		return "agent:" + meta.Principal.ClientUUID
	default:
		return "anonymous"
	}
}

func (s *Service) Audit(meta *rpc.ContextMeta, action, node, outcome, reason string) error {
	// Never retain passwords, tokens, paths, commands, payloads, or raw errors.
	return s.DB.Create(&models.OperationAudit{Actor: Actor(meta), Action: action, ClientUUID: node, Outcome: outcome, Reason: reason, Time: time.Now().UTC()}).Error
}

func (s *Service) Authorize(meta *rpc.ContextMeta, action, node string) error {
	if err := s.Check(meta, action, node); err != nil {
		_ = s.Audit(meta, action, node, "denied", "permission_denied")
		return ErrDenied
	}
	// Persisting an authorization attempt is a precondition for dispatch.
	if err := s.Audit(meta, action, node, "allowed", "authorized"); err != nil {
		return ErrDenied
	}
	return nil
}
