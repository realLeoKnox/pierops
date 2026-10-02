package localpolicy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalRootConfinement(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	secret := filepath.Join(outside, "outside.txt")
	if e := os.WriteFile(secret, []byte("untouched"), 0600); e != nil {
		t.Fatal(e)
	}
	f, e := NewFiles([]string{root})
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	if _, e = f.Open(secret); e == nil {
		t.Fatal("outside read accepted")
	}
	if _, e = f.OpenFile(secret, os.O_WRONLY|os.O_TRUNC, 0600); e == nil {
		t.Fatal("outside write accepted")
	}
	if e = os.Symlink(outside, filepath.Join(root, "escape")); e != nil {
		t.Skip(e)
	}
	path := filepath.Join(root, "escape", "outside.txt")
	if _, e = f.Open(path); e == nil {
		t.Fatal("symlink escape read accepted")
	}
	if _, e = f.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0600); e == nil {
		t.Fatal("symlink escape write accepted")
	}
	if e = f.MkdirAll(filepath.Join(root, "escape", "new"), 0700); e == nil {
		t.Fatal("symlink escape mkdir accepted")
	}
	if e = f.RemoveAll(filepath.Join(root, "escape", "outside.txt")); e == nil {
		t.Fatal("symlink escape deletion accepted")
	}
	if e = f.Rename(filepath.Join(root, "escape", "outside.txt"), filepath.Join(root, "stolen")); e == nil {
		t.Fatal("symlink escape rename accepted")
	}
	if data, e := os.ReadFile(secret); e != nil || string(data) != "untouched" {
		t.Fatalf("outside data changed: %q %v", data, e)
	}
	if e = f.RemoveAll(root); e == nil {
		t.Fatal("allowed root deletion accepted")
	}
	if e = f.Symlink("../escape", filepath.Join(root, "bad")); e == nil {
		t.Fatal("escaping symlink creation accepted")
	}
}
func TestLocalRootSafeFilesAndPinnedHandle(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "allowed")
	if e := os.Mkdir(root, 0700); e != nil {
		t.Fatal(e)
	}
	f, e := NewFiles([]string{root})
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	h, e := f.CreateTemp(root, "part-*")
	if e != nil {
		t.Fatal(e)
	}
	name := h.Name()
	h.WriteString("content")
	h.Close()
	if e = f.Rename(name, filepath.Join(root, "final")); e != nil {
		t.Fatal(e)
	}
	if e = os.Symlink("final", filepath.Join(root, "link")); e != nil {
		t.Skip(e)
	}
	h, e = f.Open(filepath.Join(root, "link"))
	if e != nil {
		t.Fatal(e)
	}
	h.Close()
	if e = os.Rename(root, root+"-moved"); e != nil {
		t.Skip(e)
	}
	if e = os.Mkdir(root, 0700); e != nil {
		t.Fatal(e)
	}
	// The lexical path still resolves against the original directory handle.
	h, e = f.Open(filepath.Join(root, "final"))
	if e != nil {
		t.Fatal(e)
	}
	h.Close()
	if _, e := os.Stat(filepath.Join(root, "final")); !os.IsNotExist(e) {
		t.Fatal("test replacement directory was not empty")
	}
}
func TestLocalRootsRequireExplicitDirectory(t *testing.T) {
	for _, name := range []string{"relative", string(filepath.Separator)} {
		if f, e := NewFiles([]string{name}); e == nil {
			f.Close()
			t.Fatalf("unsafe root accepted %q", name)
		}
	}
}
