package supportbundle

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"testing"
)

func TestGenerateSupportBundle_IncludesConfigFromProvider(t *testing.T) {
	sampleYAML := `logging:
  system:
    level: info
    output: stdout
  api:
    cert: ella_cert.pem
    key: ella_key.pem
other: value
`

	// Install provider and restore after test.
	old := ConfigProvider
	ConfigProvider = func(ctx context.Context) ([]byte, error) {
		return []byte(sampleYAML), nil
	}

	defer func() { ConfigProvider = old }()

	data := map[string]any{"bundle_metadata": map[string]any{"version": "1.0"}}

	var buf bytes.Buffer
	if err := GenerateSupportBundleFromData(context.Background(), data, &buf); err != nil {
		t.Fatalf("GenerateSupportBundleFromData failed: %v", err)
	}

	// open gzip & tar
	gr, err := gzip.NewReader(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}

	defer func() { _ = gr.Close() }()

	tr := tar.NewReader(gr)

	found := false

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			t.Fatalf("tar.Next: %v", err)
		}

		if hdr.Name != "config.yaml" {
			continue
		}

		found = true

		b, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("read config.yaml: %v", err)
		}

		s := string(b)

		if s != sampleYAML {
			t.Fatalf("expected content to be: %s, got: %s", sampleYAML, s)
		}
	}

	if !found {
		t.Fatalf("config.yaml not found in support bundle")
	}
}
