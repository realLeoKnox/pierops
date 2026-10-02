package access

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/pkg/rpc"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type Grant struct {
	ClientUUID string `json:"client_uuid"`
	Action     string `json:"action"`
}
type UserSummary struct {
	UUID       string `json:"uuid"`
	Username   string `json:"username"`
	AccessRole string `json:"access_role"`
}
type Policy struct {
	User   UserSummary `json:"user"`
	Grants []Grant     `json:"grants"`
}

func validateGrants(db *gorm.DB, role string, grants []Grant) error {
	if role != Operator && role != Viewer && role != Disabled {
		return fmt.Errorf("role must be operator, viewer or disabled")
	}
	if len(grants) > 500 {
		return fmt.Errorf("too many grants")
	}
	seen := map[Grant]bool{}
	for _, grant := range grants {
		if !Grantable(role, grant.Action) || grant.ClientUUID == "" || seen[grant] {
			return fmt.Errorf("invalid or duplicate grant")
		}
		seen[grant] = true
		var count int64
		if db.Model(&models.Client{}).Where("uuid = ?", grant.ClientUUID).Count(&count).Error != nil || count != 1 {
			return fmt.Errorf("grant node does not exist")
		}
	}
	return nil
}

func savePolicy(tx *gorm.DB, meta *rpc.ContextMeta, id, role string, grants []Grant) error {
	if err := tx.Where("user_uuid = ?", id).Delete(&models.AccessGrant{}).Error; err != nil {
		return err
	}
	for _, grant := range grants {
		if err := tx.Create(&models.AccessGrant{UserUUID: id, ClientUUID: grant.ClientUUID, Action: grant.Action}).Error; err != nil {
			return err
		}
	}
	if err := tx.Model(&models.User{}).Where("uuid = ?", id).Update("access_role", role).Error; err != nil {
		return err
	}
	if role == Disabled {
		if err := tx.Where("uuid = ?", id).Delete(&models.Session{}).Error; err != nil {
			return err
		}
	}
	details, _ := json.Marshal(map[string]any{"user_uuid": id, "role": role, "grants": grants})
	return tx.Create(&models.OperationAudit{Actor: Actor(meta), Action: "access.policy", Outcome: "changed", Reason: "policy_replaced", Details: string(details), Time: time.Now().UTC()}).Error
}

// Owner accounts are deliberately immutable through this API, preserving the
// installation owner and avoiding accidental owner lockout or privilege escalation.
func (s *Service) ReplacePolicy(meta *rpc.ContextMeta, id, role string, grants []Grant) error {
	if s.Check(meta, Manage, "") != nil {
		return ErrDenied
	}
	return s.DB.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if tx.Where("uuid = ?", id).First(&user).Error != nil {
			return fmt.Errorf("user not found")
		}
		if user.AccessRole == Owner {
			return fmt.Errorf("owner accounts cannot be changed through this API")
		}
		if err := validateGrants(tx, role, grants); err != nil {
			return err
		}
		return savePolicy(tx, meta, id, role, grants)
	})
}

func (s *Service) CreateUser(meta *rpc.ContextMeta, name, password, role string, grants []Grant) (string, error) {
	if s.Check(meta, Manage, "") != nil {
		return "", ErrDenied
	}
	name = strings.TrimSpace(name)
	if len(name) == 0 || len(name) > 50 || len(password) < 12 || len(password) > 72 {
		return "", fmt.Errorf("username must be 1-50 bytes; password must be 12-72 bytes")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	id := uuid.NewString()
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if err := validateGrants(tx, role, grants); err != nil {
			return err
		}
		if err := tx.Create(&models.User{UUID: id, Username: name, Passwd: string(hash), AccessRole: role}).Error; err != nil {
			return fmt.Errorf("failed to create user")
		}
		return savePolicy(tx, meta, id, role, grants)
	})
	return id, err
}

func (s *Service) GetPolicy(id string) (Policy, error) {
	policy := Policy{Grants: []Grant{}}
	var user models.User
	if err := s.DB.Select("uuid", "username", "access_role").Where("uuid = ?", id).First(&user).Error; err != nil {
		return policy, err
	}
	policy.User = UserSummary{user.UUID, user.Username, user.AccessRole}
	var rows []models.AccessGrant
	if err := s.DB.Where("user_uuid = ?", id).Order("client_uuid, action").Find(&rows).Error; err != nil {
		return policy, err
	}
	for _, row := range rows {
		policy.Grants = append(policy.Grants, Grant{row.ClientUUID, row.Action})
	}
	return policy, nil
}
