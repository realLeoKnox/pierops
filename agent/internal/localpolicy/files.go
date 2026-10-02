// Package localpolicy enforces settings chosen on the node, independently of Hub permissions.
package localpolicy

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Files pins directory handles for the lifetime of the Agent. All remote file
// access must use these handles, including recursive traversal and part files.
type Files struct{ roots []fileRoot }
type fileRoot struct {
	name   string
	handle *os.Root
}

func NewFiles(names []string) (*Files, error) {
	f := &Files{}
	for _, name := range names {
		if !filepath.IsAbs(name) {
			f.Close()
			return nil, errors.New("allowed file roots must be absolute existing directories")
		}
		name = filepath.Clean(name)
		if name == filepath.VolumeName(name)+string(filepath.Separator) {
			f.Close()
			return nil, errors.New("a filesystem root cannot be an allowed file root")
		}
		handle, err := os.OpenRoot(name)
		if err != nil {
			f.Close()
			return nil, errors.New("cannot open an allowed file root")
		}
		f.roots = append(f.roots, fileRoot{name, handle})
	}
	return f, nil
}
func (f *Files) Close() {
	for _, r := range f.roots {
		_ = r.handle.Close()
	}
}
func (f *Files) Roots() []string {
	var out []string
	for _, r := range f.roots {
		out = append(out, r.name)
	}
	return out
}
func (f *Files) scope(name string) (*os.Root, string, error) {
	if f == nil || !filepath.IsAbs(name) {
		return nil, "", errors.New("file path must be absolute and inside an allowed root")
	}
	name = filepath.Clean(name)
	// Most specific root first; an explicitly nested root must not fall back to
	// a broader root when its own handle rejects a symlink.
	best := -1
	relative := ""
	for i, r := range f.roots {
		rel, err := filepath.Rel(r.name, name)
		if err == nil && filepath.IsLocal(rel) && (best < 0 || len(r.name) > len(f.roots[best].name)) {
			best = i
			relative = rel
		}
	}
	if best < 0 {
		return nil, "", errors.New("file path is outside allowed roots")
	}
	return f.roots[best].handle, relative, nil
}
func (f *Files) Open(name string) (*os.File, error) {
	r, n, e := f.scope(name)
	if e != nil {
		return nil, e
	}
	h, e := r.OpenFile(n, os.O_RDONLY|nonblockingOpen, 0)
	if e != nil {
		return nil, e
	}
	info, e := h.Stat()
	if e != nil || (!info.Mode().IsRegular() && !info.IsDir()) {
		h.Close()
		return nil, errors.New("only regular files and directories are accessible")
	}
	return h, nil
}
func (f *Files) OpenFile(name string, flag int, mode fs.FileMode) (*os.File, error) {
	r, n, e := f.scope(name)
	if e != nil {
		return nil, e
	}
	// Inspect the opened file before truncating; never truncate a special file.
	h, e := r.OpenFile(n, (flag&^os.O_TRUNC)|nonblockingOpen, mode)
	if e != nil {
		return nil, e
	}
	info, e := h.Stat()
	if e != nil || !info.Mode().IsRegular() {
		h.Close()
		return nil, errors.New("remote writes require a regular file")
	}
	if flag&os.O_TRUNC != 0 {
		if e = h.Truncate(0); e != nil {
			h.Close()
			return nil, e
		}
	}
	return h, nil
}
func (f *Files) Stat(name string) (os.FileInfo, error) {
	r, n, e := f.scope(name)
	if e != nil {
		return nil, e
	}
	return r.Stat(n)
}
func (f *Files) Lstat(name string) (os.FileInfo, error) {
	r, n, e := f.scope(name)
	if e != nil {
		return nil, e
	}
	return r.Lstat(n)
}
func (f *Files) ReadDir(name string) ([]os.DirEntry, error) {
	h, e := f.Open(name)
	if e != nil {
		return nil, e
	}
	defer h.Close()
	return h.ReadDir(-1)
}
func (f *Files) MkdirAll(name string, mode fs.FileMode) error {
	r, n, e := f.scope(name)
	if e != nil {
		return e
	}
	return r.MkdirAll(n, mode.Perm())
}
func (f *Files) Remove(name string) error {
	r, n, e := f.scope(name)
	if e != nil {
		return e
	}
	if n == "." {
		return errors.New("cannot remove an allowed root")
	}
	return r.Remove(n)
}
func (f *Files) RemoveAll(name string) error {
	r, n, e := f.scope(name)
	if e != nil {
		return e
	}
	if n == "." {
		return errors.New("cannot remove an allowed root")
	}
	return r.RemoveAll(n)
}
func (f *Files) Rename(source, destination string) error {
	r, s, e := f.scope(source)
	if e != nil {
		return e
	}
	d, n, e := f.scope(destination)
	if e != nil {
		return e
	}
	if r != d {
		return errors.New("moving between allowed roots is not supported; copy and delete separately")
	}
	if s == "." || n == "." {
		return errors.New("cannot replace or move an allowed root")
	}
	return r.Rename(s, n)
}
func (f *Files) Readlink(name string) (string, error) {
	r, n, e := f.scope(name)
	if e != nil {
		return "", e
	}
	return r.Readlink(n)
}
func (f *Files) Symlink(target, name string) error {
	r, n, e := f.scope(name)
	if e != nil {
		return e
	}
	if filepath.IsAbs(target) || !filepath.IsLocal(filepath.Join(filepath.Dir(n), target)) {
		return errors.New("symlink must stay within its allowed root")
	}
	return r.Symlink(target, n)
}
func (f *Files) CreateTemp(dir, pattern string) (*os.File, error) {
	for i := 0; i < 10; i++ {
		var b [12]byte
		if _, e := rand.Read(b[:]); e != nil {
			return nil, e
		}
		name := filepath.Join(dir, strings.Replace(pattern, "*", hex.EncodeToString(b[:]), 1))
		h, e := f.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
		if errors.Is(e, fs.ErrExist) {
			continue
		}
		return h, e
	}
	return nil, errors.New("cannot allocate temporary file")
}
func (f *Files) Chmod(name string, mode fs.FileMode) error {
	h, e := f.Open(name)
	if e != nil {
		return e
	}
	defer h.Close()
	return h.Chmod(mode.Perm())
}
func (f *Files) Chown(name string, uid, gid int) error {
	h, e := f.Open(name)
	if e != nil {
		return e
	}
	defer h.Close()
	return h.Chown(uid, gid)
}
func (f *Files) Truncate(name string, size int64) error {
	h, e := f.OpenFile(name, os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	defer h.Close()
	return h.Truncate(size)
}

// Preserve file mode, but not timestamps: path-based Root.Chtimes has a Unix
// symlink race. A later descriptor-based implementation may restore timestamps.
func (f *Files) Chtimes(name string, atime, mtime time.Time) error { _, e := f.Stat(name); return e }
func (f *Files) WalkDir(name string, fn fs.WalkDirFunc) error {
	r, n, e := f.scope(name)
	if e != nil {
		return e
	}
	return fs.WalkDir(r.FS(), filepath.ToSlash(n), func(p string, d fs.DirEntry, e error) error {
		return fn(filepath.Join(r.Name(), filepath.FromSlash(p)), d, e)
	})
}
