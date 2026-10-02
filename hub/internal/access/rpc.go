package access

import "github.com/komari-monitor/komari/pkg/rpc"

// RPCScope is the complete allowlist for restricted users. Unlisted privileged
// methods require hub.manage, even when plugins introduce new namespaces.
func RPCScope(req *rpc.JsonRpcRequest) (string, []string, error) {
	if req.Method == "admin:listClients" {
		return NodeList, nil, nil
	}
	if req.Method == "admin:accessGetSelf" {
		return Self, nil, nil
	}
	if req.Method == "admin:exec" {
		var p struct {
			Command string   `json:"command"`
			Clients []string `json:"clients"`
		}
		if req.BindParams(&p) != nil || len(p.Clients) == 0 || len(p.Clients) > 100 {
			return "", nil, ErrDenied
		}
		for _, node := range p.Clients {
			if node == "" {
				return "", nil, ErrDenied
			}
		}
		return CommandExec, p.Clients, nil
	}
	if req.Method == "admin:getSpecificTaskResult" {
		var p struct {
			TaskID string `json:"task_id"`
			UUID   string `json:"uuid"`
		}
		if req.BindParams(&p) != nil || p.UUID == "" {
			return "", nil, ErrDenied
		}
		return CommandRead, []string{p.UUID}, nil
	}
	action := ""
	switch req.Method {
	case "admin:getClient":
		action = NodeRead
	case "admin:fileList", "admin:fileListRoots", "admin:fileStat", "admin:fileSearch":
		action = FileRead
	case "admin:fileMkdir", "admin:fileDelete", "admin:fileMove", "admin:fileCopy", "admin:fileChmod", "admin:fileChown":
		action = FileWrite
	default:
		return Manage, nil, nil
	}
	var p struct {
		UUID string `json:"uuid"`
	}
	if req.BindParams(&p) != nil || p.UUID == "" {
		return "", nil, ErrDenied
	}
	return action, []string{p.UUID}, nil
}

func (s *Service) AuthorizeRPC(meta *rpc.ContextMeta, req *rpc.JsonRpcRequest) error {
	action, nodes, err := RPCScope(req)
	if err != nil {
		_ = s.Audit(meta, "rpc.invalid_scope", "", "denied", "invalid_scope")
		return ErrDenied
	}
	// Preflight every target before recording or dispatching the first one.
	for _, node := range nodes {
		if err := s.Check(meta, action, node); err != nil {
			_ = s.Audit(meta, action, node, "denied", "permission_denied")
			return ErrDenied
		}
	}
	if len(nodes) == 0 {
		return s.Authorize(meta, action, "")
	}
	for _, node := range nodes {
		if s.Audit(meta, action, node, "allowed", "authorized") != nil {
			return ErrDenied
		}
	}
	return nil
}
