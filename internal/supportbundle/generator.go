package supportbundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/ellanetworks/core/version"
)

func getVersionInfo() *struct{ Version, Revision string } {
	v := version.GetVersion()
	if v == nil {
		return nil
	}

	return &struct{ Version, Revision string }{Version: v.Version, Revision: v.Revision}
}

// ConfigProvider can be set by main to provide the runtime configuration YAML
// bytes to include in support bundles. If nil, the generator falls back to
// repository-local config paths.
var ConfigProvider func(ctx context.Context) ([]byte, error)

// GenerateSupportBundleFromData writes a minimal support bundle containing the
// provided redacted data as db.json into the writer as a gzipped tar archive.
// This avoids import cycles: callers (e.g. internal/db) should fetch and redact
// the data and then call this helper to write the archive.
func GenerateSupportBundleFromData(ctx context.Context, data map[string]any, w io.Writer) error {
	// No test hooks; generate normally.
	gz := gzip.NewWriter(w)

	tw := tar.NewWriter(gz)

	defer func() {
		_ = tw.Close()
		_ = gz.Close()
	}()

	// Marshal pretty JSON
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal db.json: %w", err)
	}

	// write db.json into tar
	hdr := &tar.Header{
		Name:    "db.json",
		Size:    int64(len(b)),
		Mode:    0o600,
		ModTime: time.Now(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("failed to write tar header: %w", err)
	}

	if _, err := tw.Write(b); err != nil {
		return fmt.Errorf("failed to write db.json to tar: %w", err)
	}

	if ConfigProvider != nil {
		if bs, err := ConfigProvider(ctx); err == nil {
			hdr := &tar.Header{Name: "config.yaml", Size: int64(len(bs)), Mode: 0o600, ModTime: time.Now()}
			_ = tw.WriteHeader(hdr)
			_, _ = tw.Write(bs)
		} else {
			hdr := &tar.Header{Name: "system/error-config.txt", Size: int64(len(err.Error())), Mode: 0o600, ModTime: time.Now()}
			_ = tw.WriteHeader(hdr)
			_, _ = tw.Write([]byte(err.Error()))
		}
	}

	writeFile := func(name string, b []byte) error {
		hdr := &tar.Header{Name: name, Size: int64(len(b)), Mode: 0o600, ModTime: time.Now()}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}

		_, err := tw.Write(b)

		return err
	}

	if ver := getVersionInfo(); ver != nil {
		buf := &bytes.Buffer{}
		fmt.Fprintf(buf, "Version: %s\nRevision: %s\n", ver.Version, ver.Revision)
		_ = writeFile("system/version.txt", buf.Bytes())
	}

	if bs, err := os.ReadFile("/etc/os-release"); err == nil {
		_ = writeFile("system/os-release.txt", bs)
	} else {
		_ = writeFile("system/error-etc-os-release.txt", []byte(err.Error()))
	}

	if out, err := runCommand(ctx, "uname", "-r"); err == nil {
		_ = writeFile("system/uname.txt", out)
	} else {
		_ = writeFile("system/error-uname.txt", []byte(err.Error()))
	}

	for _, f := range []struct{ path, name string }{{"/proc/meminfo", "system/meminfo.txt"}, {"/proc/cpuinfo", "system/cpuinfo.txt"}} {
		if bs, err := os.ReadFile(f.path); err == nil {
			_ = writeFile(f.name, bs)
		} else {
			_ = writeFile("system/error-"+f.name+".txt", []byte(err.Error()))
		}
	}

	if out, err := runCommand(ctx, "df", "-h"); err == nil {
		_ = writeFile("system/df.txt", out)
	} else {
		_ = writeFile("system/error-df.txt", []byte(err.Error()))
	}

	if out, err := runCommand(ctx, "ip", "a"); err == nil {
		_ = writeFile("system/ip_a.txt", out)
	} else {
		_ = writeFile("system/error-ip_a.txt", []byte(err.Error()))
	}

	if out, err := runCommand(ctx, "ip", "route"); err == nil {
		_ = writeFile("system/ip_route.txt", out)
	} else {
		_ = writeFile("system/error-ip_route.txt", []byte(err.Error()))
	}

	if out, err := runCommand(ctx, "ip", "neigh"); err == nil {
		_ = writeFile("system/ip_neigh.txt", out)
	} else {
		_ = writeFile("system/error-ip_neigh.txt", []byte(err.Error()))
	}

	if out, err := runCommand(ctx, "ss", "-tunap"); err == nil {
		_ = writeFile("system/netstat.txt", out)
	} else if out2, err2 := runCommand(ctx, "netstat", "-tunap"); err2 == nil {
		_ = writeFile("system/netstat.txt", out2)
	} else {
		_ = writeFile("system/error-netstat.txt", []byte(err.Error()))
	}

	return nil
}

// runCommand executes a command with a fixed timeout and returns combined output.
func runCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	timeout := 5 * time.Second
	cctx, cancel := context.WithTimeout(ctx, timeout)

	defer cancel()

	cmd := exec.CommandContext(cctx, name, args...)
	out, err := cmd.CombinedOutput()

	if cctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("command timed out: %s %v", name, args)
	}

	if err != nil {
		return out, fmt.Errorf("command failed: %w; output: %s", err, string(out))
	}

	return out, nil
}
