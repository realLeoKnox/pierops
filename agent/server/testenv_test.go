package server

import (
	"os"
	"testing"
)

// requireNetwork gates tests that contact fixed public hosts and need raw
// ICMP sockets. They are opt-in so the default suite is deterministic.
func requireNetwork(t *testing.T) {
	t.Helper()
	if os.Getenv("PIEROPS_NETWORK_TESTS") != "1" {
		t.Skip("external network test; set PIEROPS_NETWORK_TESTS=1 to run")
	}
}

// requireNonRootRunner skips command execution tests when the suite itself
// runs as root. TestMain uses the current user as the execution user, and the
// local policy refuses UID 0 by design. Switching from root to a dedicated
// user is covered by the real-host acceptance checklist instead.
func requireNonRootRunner(t *testing.T) {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("command execution refuses UID 0; run the suite as a non-root user")
	}
}
