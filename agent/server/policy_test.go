package server

import (
	pkg_flags "github.com/komari-monitor/komari-agent/cmd/flags"
	"os"
	"os/user"
	"testing"
)

// Legacy filesystem/transport tests operate only on disposable temporary data.
// Production still starts with no roots and no enabled operational capabilities.
func TestMain(m *testing.M) {
	if err := ConfigureFileRoots([]string{os.TempDir()}); err != nil {
		panic(err)
	}
	pkg_flags.GlobalConfig.EnableFileRead = true
	pkg_flags.GlobalConfig.EnableFileWrite = true
	if u, e := user.Current(); e == nil {
		pkg_flags.GlobalConfig.ExecutionUser = u.Username
	}
	code := m.Run()
	fileFS.Close()
	os.Exit(code)
}

func TestLocalFileCapabilityClassification(t *testing.T) {
	old := *pkg_flags.GlobalConfig
	defer func() { *pkg_flags.GlobalConfig = old }()
	pkg_flags.GlobalConfig.EnableFileWrite = false
	for _, op := range []string{"create", "mkdir", "delete", "move", "copy", "chmod", "upload_stream", "upload_commit", "upload_cancel", "chown", "unknown"} {
		if pkg_flags.GlobalConfig.Allows(pkg_flags.FileAction(op)) {
			t.Fatalf("write or unknown operation allowed: %s", op)
		}
	}
	if !pkg_flags.GlobalConfig.Allows(pkg_flags.FileAction("download_stream")) {
		t.Fatal("read capability missing")
	}
	pkg_flags.GlobalConfig.DisableWebSsh = true
	if pkg_flags.GlobalConfig.Allows("file.read") {
		t.Fatal("master disable ignored")
	}
}
func TestLocalUploadIdentifierBoundary(t *testing.T) {
	for _, id := range []string{"../escape", "a/b", "a\\b", "", "x.y"} {
		if validUploadID(id) {
			t.Fatalf("unsafe upload ID accepted: %q", id)
		}
	}
}
