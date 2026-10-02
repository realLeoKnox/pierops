//go:build !windows

package localpolicy

import (
	"os"
	"os/exec"
	"os/user"
	"strings"
	"testing"
)

func TestLocalExecutionIdentityAndEnvironment(t *testing.T) {
	if e := ValidateExecutionUser(""); e == nil {
		t.Fatal("empty execution user accepted")
	}
	if e := ValidateExecutionUser("root"); e == nil {
		t.Fatal("root execution user accepted")
	}
	u, e := user.Current()
	if e != nil || u.Uid == "0" {
		t.Skip("non-root fixture required")
	}
	t.Setenv("AGENT_TOKEN", "test-only-env-marker")
	cmd := exec.Command("/bin/sh", "-c", "true")
	if e := PrepareCommand(cmd, u.Username); e != nil {
		t.Fatal(e)
	}
	for _, value := range cmd.Env {
		if strings.Contains(value, "AGENT_") || strings.Contains(value, "test-only-env-marker") {
			t.Fatal("Agent environment leaked")
		}
	}
	if cmd.Dir != u.HomeDir {
		t.Fatal("execution home not applied")
	}
	if os.Geteuid() != 0 && cmd.SysProcAttr != nil && cmd.SysProcAttr.Credential != nil {
		t.Fatal("unneeded identity switch")
	}
}
