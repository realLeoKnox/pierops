//go:build !windows

package localpolicy

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"syscall"
)

func executionIdentity(name string) (*user.User, uint32, uint32, []uint32, error) {
	if name == "" {
		return nil, 0, 0, nil, errors.New("terminal and commands require a configured non-root execution user")
	}
	u, e := user.Lookup(name)
	if e != nil {
		return nil, 0, 0, nil, errors.New("execution user does not exist")
	}
	uid, e := strconv.ParseUint(u.Uid, 10, 32)
	if e != nil || uid == 0 {
		return nil, 0, 0, nil, errors.New("execution user must have a non-root numeric UID")
	}
	gid, e := strconv.ParseUint(u.Gid, 10, 32)
	if e != nil {
		return nil, 0, 0, nil, errors.New("execution group has invalid GID")
	}
	if os.Geteuid() != 0 && uint64(os.Geteuid()) != uid {
		return nil, 0, 0, nil, errors.New("Agent must run as root or as its configured execution user")
	}
	groupIDs, e := u.GroupIds()
	if e != nil {
		return nil, 0, 0, nil, e
	}
	groups := make([]uint32, 0, len(groupIDs))
	for _, id := range groupIDs {
		g, e := strconv.ParseUint(id, 10, 32)
		if e != nil {
			return nil, 0, 0, nil, e
		}
		groups = append(groups, uint32(g))
	}
	return u, uint32(uid), uint32(gid), groups, nil
}
func ValidateExecutionUser(name string) error { _, _, _, _, e := executionIdentity(name); return e }
func PrepareCommand(cmd *exec.Cmd, name string) error {
	u, uid, gid, groups, e := executionIdentity(name)
	if e != nil {
		return e
	}
	cmd.Dir = u.HomeDir
	// Credentials and Hub settings must not enter a remote shell's environment.
	cmd.Env = []string{"PATH=/usr/local/bin:/usr/bin:/bin", "HOME=" + u.HomeDir, "USER=" + u.Username, "LOGNAME=" + u.Username, "TERM=xterm-256color", "LANG=C.UTF-8", "LC_ALL=C.UTF-8"}
	if os.Geteuid() == 0 {
		if cmd.SysProcAttr == nil {
			cmd.SysProcAttr = &syscall.SysProcAttr{}
		}
		cmd.SysProcAttr.Credential = &syscall.Credential{Uid: uid, Gid: gid, Groups: groups}
	}
	return nil
}
func ExecutionHome(name string) (string, error) {
	u, _, _, _, e := executionIdentity(name)
	if e != nil {
		return "", fmt.Errorf("execution identity: %w", e)
	}
	return u.HomeDir, nil
}

const nonblockingOpen = syscall.O_NONBLOCK
