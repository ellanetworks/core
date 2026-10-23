// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package supportbundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"reflect"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/redact"
	"github.com/ellanetworks/core/version"
	"go.uber.org/zap"
)

func getVersionInfo() *struct{ Version, Revision string } {
	v := version.GetVersion()
	if v == nil {
		return nil
	}

	return &struct{ Version, Revision string }{Version: v.Version, Revision: v.Revision}
}

var ConfigProvider func(ctx context.Context) ([]byte, error)

var BpfDumper func(ctx context.Context, tw *tar.Writer) (SectionStatus, error)

var AMFDumper func(ctx context.Context) (any, error)

var MMEDumper func(ctx context.Context) (any, error)

func GenerateSupportBundleFromData(ctx context.Context, data SanitizedData, w io.Writer) (result error) {
	output := &errorWriter{Writer: w}
	gz := gzip.NewWriter(output)

	tw := tar.NewWriter(gz)

	defer func() {
		result = errors.Join(result, tw.Close(), gz.Close(), output.err)
	}()

	collectors := map[string]SectionStatus{}

	for name, present := range map[string]bool{"config.yaml": ConfigProvider != nil, "bpf": BpfDumper != nil, "amf_ues.json": AMFDumper != nil, "mme_ues.json": MMEDumper != nil} {
		status := SectionStatus{Status: "complete"}
		if !present {
			status = SectionStatus{Status: "omitted", Reason: "provider unavailable"}
		}

		collectors[name] = status
	}

	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal db.json: %w", err)
	}

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
			masked, maskErr := redact.ApplyYAML(bs)
			if maskErr != nil {
				collectors["config.yaml"] = SectionStatus{Status: "failed", Reason: "sanitization failed"}
				errBs := []byte(maskErr.Error())
				hdr := &tar.Header{Name: "system/error-config.txt", Size: int64(len(errBs)), Mode: 0o600, ModTime: time.Now()}
				_ = tw.WriteHeader(hdr)
				_, _ = tw.Write(errBs)
			} else {
				collectors["config.yaml"] = SectionStatus{Status: "complete", Count: 1}
				hdr := &tar.Header{Name: "config.yaml", Size: int64(len(masked)), Mode: 0o600, ModTime: time.Now()}
				_ = tw.WriteHeader(hdr)
				_, _ = tw.Write(masked)
			}
		} else {
			collectors["config.yaml"] = SectionStatus{Status: "failed", Reason: "collection failed"}
			errText := "Configuration collection failed"
			hdr := &tar.Header{Name: "system/error-config.txt", Size: int64(len(errText)), Mode: 0o600, ModTime: time.Now()}
			_ = tw.WriteHeader(hdr)
			_, _ = tw.Write([]byte(errText))
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

	_ = writeFile("logs/system.log", logger.RecentLogs())

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

	if out, err := runCommand(ctx, "uname"); err == nil {
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

	if out, err := runCommand(ctx, "df"); err == nil {
		_ = writeFile("system/df.txt", out)
	} else {
		_ = writeFile("system/error-df.txt", []byte(err.Error()))
	}

	if out, err := runCommand(ctx, "ip a"); err == nil {
		_ = writeFile("system/ip_a.txt", out)
	} else {
		_ = writeFile("system/error-ip_a.txt", []byte(err.Error()))
	}

	if out, err := runCommand(ctx, "ip route"); err == nil {
		_ = writeFile("system/ip_route.txt", out)
	} else {
		_ = writeFile("system/error-ip_route.txt", []byte(err.Error()))
	}

	if out, err := runCommand(ctx, "ip neigh"); err == nil {
		_ = writeFile("system/ip_neigh.txt", out)
	} else {
		_ = writeFile("system/error-ip_neigh.txt", []byte(err.Error()))
	}

	if out, err := runCommand(ctx, "ss"); err == nil {
		_ = writeFile("system/netstat.txt", out)
	} else if out2, err2 := runCommand(ctx, "netstat"); err2 == nil {
		_ = writeFile("system/netstat.txt", out2)
	} else {
		_ = writeFile("system/error-netstat.txt", []byte(err.Error()))
	}

	if BpfDumper != nil {
		status, err := BpfDumper(ctx, tw)
		if err != nil {
			status.Status, status.Reason = "failed", "collection failed"
			status = status.WithCount(status.Count)

			logger.APILog.Error("bpf dump failed", zap.Error(err))

			errBs := []byte("BPF collection failed")

			hdr := &tar.Header{Name: "bpf/error.txt", Size: int64(len(errBs)), Mode: 0o600, ModTime: time.Now()}
			if werr := tw.WriteHeader(hdr); werr != nil {
				logger.APILog.Warn("failed to write tar header for bpf error file", zap.Error(werr))
			} else {
				if _, werr2 := tw.Write(errBs); werr2 != nil {
					logger.APILog.Warn("failed to write bpf error file to tar", zap.Error(werr2))
				}
			}
		}

		collectors["bpf"] = status
	}

	if AMFDumper != nil {
		amfData, err := AMFDumper(ctx)
		if err != nil {
			collectors["amf_ues.json"] = SectionStatus{Status: "failed", Reason: "collection failed"}

			logger.APILog.Warn("amf ue dump failed", zap.Error(err))

			errBs := []byte("AMF collection failed")
			_ = writeFile("amf_ues_error.txt", errBs)
		} else if amfData != nil {
			amfJSON, err := json.MarshalIndent(amfData, "", "  ")
			if err != nil {
				collectors["amf_ues.json"] = SectionStatus{Status: "failed", Reason: "encoding failed"}

				logger.APILog.Warn("failed to marshal amf_ues.json", zap.Error(err))
			} else {
				count := 1
				if records := reflect.ValueOf(amfData); records.Kind() == reflect.Slice || records.Kind() == reflect.Array {
					count = records.Len()
				}

				collectors["amf_ues.json"] = SectionStatus{Status: "complete", Count: count}

				if err := writeFile("amf_ues.json", amfJSON); err != nil {
					return err
				}
			}
		} else {
			collectors["amf_ues.json"] = SectionStatus{Status: "omitted", Reason: "state unavailable"}
		}
	}

	if MMEDumper != nil {
		mmeData, err := MMEDumper(ctx)
		if err != nil {
			collectors["mme_ues.json"] = SectionStatus{Status: "failed", Reason: "collection failed"}

			logger.APILog.Warn("mme ue dump failed", zap.Error(err))

			errBs := []byte("MME collection failed")
			_ = writeFile("mme_ues_error.txt", errBs)
		} else if mmeData != nil {
			mmeJSON, err := json.MarshalIndent(mmeData, "", "  ")
			if err != nil {
				collectors["mme_ues.json"] = SectionStatus{Status: "failed", Reason: "encoding failed"}

				logger.APILog.Warn("failed to marshal mme_ues.json", zap.Error(err))
			} else {
				count := 1
				if records := reflect.ValueOf(mmeData); records.Kind() == reflect.Slice || records.Kind() == reflect.Array {
					count = records.Len()
				}

				collectors["mme_ues.json"] = SectionStatus{Status: "complete", Count: count}

				if err := writeFile("mme_ues.json", mmeJSON); err != nil {
					return err
				}
			}
		} else {
			collectors["mme_ues.json"] = SectionStatus{Status: "omitted", Reason: "state unavailable"}
		}
	}

	manifest, err := json.MarshalIndent(collectors, "", "  ")
	if err != nil {
		return err
	}

	return writeFile("collection_status.json", manifest)
}

type errorWriter struct {
	io.Writer
	err error
}

func (w *errorWriter) Write(b []byte) (int, error) {
	n, err := w.Writer.Write(b)
	if err == nil && n < len(b) {
		err = io.ErrShortWrite
	}

	if err != nil && w.err == nil {
		w.err = err
	}

	return n, err
}

func runCommand(ctx context.Context, name string) ([]byte, error) {
	timeout := 5 * time.Second
	cctx, cancel := context.WithTimeout(ctx, timeout)

	defer cancel()

	var cmd *exec.Cmd

	switch name {
	case "uname":
		cmd = exec.CommandContext(cctx, "uname", "-r")
	case "df":
		cmd = exec.CommandContext(cctx, "df", "-h")
	case "ip a":
		cmd = exec.CommandContext(cctx, "ip", "a")
	case "ip route":
		cmd = exec.CommandContext(cctx, "ip", "route")
	case "ip neigh":
		cmd = exec.CommandContext(cctx, "ip", "neigh")
	case "ss":
		cmd = exec.CommandContext(cctx, "ss", "-tunap")
	case "netstat":
		cmd = exec.CommandContext(cctx, "netstat", "-tunap")
	default:
		return nil, fmt.Errorf("unsupported command: %s", name)
	}

	out, err := cmd.CombinedOutput()

	if cctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("command timed out: %s", name)
	}

	if err != nil {
		return out, fmt.Errorf("command failed: %w; output: %s", err, string(out))
	}

	return out, nil
}
