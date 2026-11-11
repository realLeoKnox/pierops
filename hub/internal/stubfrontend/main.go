// Command stubfrontend writes a placeholder default theme so that the Hub
// compiles and its tests run without Node.js or the upstream Web UI sources.
//
// It never overwrites an existing archive: a real frontend produced by
// scripts/build-frontend.sh always wins. The placeholder is for tests and
// static analysis only and must not be shipped in a release build.
package main

import (
	"archive/tar"
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/klauspost/compress/zstd"
)

const (
	themeDir  = "web/public/defaultTheme"
	indexHTML = "<!doctype html><html lang=\"en\"><head><meta charset=\"utf-8\"><title>PierOps</title></head>" +
		"<body><p>PierOps placeholder frontend (test build). Run scripts/build-frontend.sh for the real UI.</p></body></html>\n"
	themeJSON = "{\"name\":\"PierOps placeholder\",\"short\":\"default\",\"description\":\"Placeholder theme for tests\",\"version\":\"0.0.0-stub\",\"author\":\"PierOps\"}\n"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "stubfrontend:", err)
		os.Exit(1)
	}
}

func run() error {
	archive := filepath.Join(themeDir, "dist.tar.zst")
	if info, err := os.Stat(archive); err == nil && info.Size() > 0 {
		fmt.Println("stubfrontend: existing frontend archive kept")
		return nil
	}
	if _, err := os.Stat("web/public/public.go"); err != nil {
		return fmt.Errorf("run from the hub module directory: %w", err)
	}
	if err := os.MkdirAll(themeDir, 0o755); err != nil {
		return err
	}

	var tarball bytes.Buffer
	tw := tar.NewWriter(&tarball)
	header := &tar.Header{Name: "index.html", Mode: 0o644, Size: int64(len(indexHTML))}
	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	if _, err := tw.Write([]byte(indexHTML)); err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}

	encoder, err := zstd.NewWriter(nil)
	if err != nil {
		return err
	}
	compressed := encoder.EncodeAll(tarball.Bytes(), nil)
	if err := encoder.Close(); err != nil {
		return err
	}
	if err := os.WriteFile(archive, compressed, 0o644); err != nil {
		return err
	}

	metadata := filepath.Join(themeDir, "komari-theme.json")
	if _, err := os.Stat(metadata); os.IsNotExist(err) {
		if err := os.WriteFile(metadata, []byte(themeJSON), 0o644); err != nil {
			return err
		}
	}
	fmt.Println("stubfrontend: placeholder frontend written to", themeDir)
	return nil
}
