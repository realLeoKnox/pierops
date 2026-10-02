package jsonrpc

import (
	"context"

	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/internal/access"
	"github.com/komari-monitor/komari/pkg/rpc"
)

func init() {
	reg("accessCreateUser", adminAccessCreateUser, "Create a restricted user with explicit node grants")
	reg("accessSetPolicy", adminAccessSetPolicy, "Replace a restricted user's role and node grants")
	reg("accessGetPolicy", adminAccessGetPolicy, "Read a user's policy")
	reg("accessListUsers", adminAccessListUsers, "List users without credentials")
	reg("accessGetSelf", adminAccessGetSelf, "Read the caller's policy")
	reg("accessGetAudit", adminAccessGetAudit, "Read operation authorization and policy audit events")
	rpc.MarkSensitive("admin:accessCreateUser")
	rpc.MarkSensitive("admin:accessSetPolicy")
}

func adminAccessCreateUser(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var p struct {
		Username string         `json:"username"`
		Password string         `json:"password"`
		Role     string         `json:"role"`
		Grants   []access.Grant `json:"grants"`
	}
	if req.BindParams(&p) != nil {
		return nil, rpc.MakeError(rpc.InvalidParams, "Invalid params", nil)
	}
	id, err := access.Default().CreateUser(rpc.MetaFromContext(ctx), p.Username, p.Password, p.Role, p.Grants)
	if err != nil {
		return nil, rpc.MakeError(rpc.InvalidParams, err.Error(), nil)
	}
	return map[string]any{"uuid": id}, nil
}

func adminAccessSetPolicy(ctx context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var p struct {
		UUID   string         `json:"uuid"`
		Role   string         `json:"role"`
		Grants []access.Grant `json:"grants"`
	}
	if req.BindParams(&p) != nil || p.UUID == "" {
		return nil, rpc.MakeError(rpc.InvalidParams, "uuid is required", nil)
	}
	if err := access.Default().ReplacePolicy(rpc.MetaFromContext(ctx), p.UUID, p.Role, p.Grants); err != nil {
		return nil, rpc.MakeError(rpc.InvalidParams, err.Error(), nil)
	}
	return nil, nil
}

func adminAccessGetPolicy(_ context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var p struct {
		UUID string `json:"uuid"`
	}
	if req.BindParams(&p) != nil || p.UUID == "" {
		return nil, rpc.MakeError(rpc.InvalidParams, "uuid is required", nil)
	}
	policy, err := access.Default().GetPolicy(p.UUID)
	if err != nil {
		return nil, rpc.MakeError(rpc.NotFound, "user not found", nil)
	}
	return policy, nil
}

func adminAccessListUsers(_ context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	users := []access.UserSummary{}
	if access.Default().DB.Model(&models.User{}).Select("uuid", "username", "access_role").Order("created_at").Scan(&users).Error != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Cannot read users", nil)
	}
	return users, nil
}

func adminAccessGetSelf(ctx context.Context, _ *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	subject, err := access.Default().Resolve(rpc.MetaFromContext(ctx))
	if err != nil {
		return nil, rpc.MakeError(rpc.PermissionDenied, "Operation not permitted", nil)
	}
	if subject.ID == "api-key" || subject.ID == "internal" {
		return map[string]any{"role": subject.Role}, nil
	}
	policy, err := access.Default().GetPolicy(subject.ID)
	if err != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Cannot read policy", nil)
	}
	return policy, nil
}

func adminAccessGetAudit(_ context.Context, req *rpc.JsonRpcRequest) (any, *rpc.JsonRpcError) {
	var p struct {
		Before uint64 `json:"before"`
		Limit  int    `json:"limit"`
	}
	if req.BindParams(&p) != nil {
		return nil, rpc.MakeError(rpc.InvalidParams, "Invalid params", nil)
	}
	if p.Limit <= 0 {
		p.Limit = 100
	}
	if p.Limit > 200 {
		p.Limit = 200
	}
	events := []models.OperationAudit{}
	query := access.Default().DB.Order("id desc").Limit(p.Limit)
	if p.Before > 0 {
		query = query.Where("id < ?", p.Before)
	}
	if query.Find(&events).Error != nil {
		return nil, rpc.MakeError(rpc.InternalError, "Cannot read audit", nil)
	}
	return events, nil
}
