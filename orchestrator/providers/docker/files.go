package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/docker/docker/api/types/container"
)

// CopyTo writes data to the file at filePath inside the container.
// filePath must be absolute (e.g. "/data/whitelist.json"). Parent
// directory is inferred from the path; the existing file is
// overwritten if present.
//
// Docker's CopyToContainer takes a tar stream, so we build a one-entry
// tar in memory before shipping it. Acceptable for config-file sized
// payloads (KB–MB); for large data use volume mounts or direct S3.
func (d *DockerProvider) CopyTo(ctx context.Context, id, filePath string, data []byte) error {
	if !strings.HasPrefix(filePath, "/") {
		return fmt.Errorf("filePath must be absolute, got %q", filePath)
	}
	dir := path.Dir(filePath)
	name := path.Base(filePath)

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{
		Name: name,
		Mode: 0o644,
		Size: int64(len(data)),
	}); err != nil {
		return fmt.Errorf("tar header: %w", err)
	}
	if _, err := tw.Write(data); err != nil {
		return fmt.Errorf("tar body: %w", err)
	}
	if err := tw.Close(); err != nil {
		return fmt.Errorf("tar close: %w", err)
	}

	return d.client.CopyToContainer(ctx, id, dir, &buf, container.CopyToContainerOptions{})
}

// CopyFrom reads the file at filePath inside the container and
// returns its bytes. Works for regular files only — directory reads
// return an error. The caller should enforce a size limit to avoid
// memory blow-up on unexpected paths.
func (d *DockerProvider) CopyFrom(ctx context.Context, id, filePath string) ([]byte, error) {
	if !strings.HasPrefix(filePath, "/") {
		return nil, fmt.Errorf("filePath must be absolute, got %q", filePath)
	}
	rc, stat, err := d.client.CopyFromContainer(ctx, id, filePath)
	if err != nil {
		return nil, fmt.Errorf("copy from container: %w", err)
	}
	defer rc.Close()
	if stat.Mode.IsDir() {
		return nil, fmt.Errorf("%q is a directory; CopyFrom only reads regular files", filePath)
	}

	tr := tar.NewReader(rc)
	hdr, err := tr.Next()
	if err != nil {
		return nil, fmt.Errorf("tar next: %w", err)
	}
	if hdr.Typeflag != tar.TypeReg {
		return nil, fmt.Errorf("unexpected tar entry type %v", hdr.Typeflag)
	}
	data, err := io.ReadAll(tr)
	if err != nil {
		return nil, fmt.Errorf("read tar entry: %w", err)
	}
	return data, nil
}

// DeleteFile removes the file at filePath inside the container.
// Implemented via exec since Docker's SDK has no direct "delete file"
// call. Requires the container to have `rm` on PATH (all standard
// base images do). Idempotent — removing a missing file succeeds.
func (d *DockerProvider) DeleteFile(ctx context.Context, id, filePath string) error {
	if !strings.HasPrefix(filePath, "/") {
		return fmt.Errorf("filePath must be absolute, got %q", filePath)
	}
	_, err := d.Exec(ctx, id, []string{"rm", "-f", filePath})
	return err
}
