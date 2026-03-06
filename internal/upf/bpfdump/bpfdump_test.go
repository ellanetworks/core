package bpfdump

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"testing"

	bpf "github.com/cilium/ebpf"
	"github.com/ellanetworks/core/internal/upf/ebpf"
)

// Test nil bpfObjects returns nil, no error
func TestDumpAll_Nil(t *testing.T) {
	metas, err := DumpAll(context.Background(), nil, DumpOptions{}, tar.NewWriter(&bytes.Buffer{}))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if metas != nil {
		t.Fatalf("expected nil metas, got %v", metas)
	}
}

// mockMap implements MapHandle for tests
type mockMap struct {
	mtype   bpf.MapType
	keySize int
	valSize int
	entries [][2][]byte // list of {key, val}
}

func (m *mockMap) Type() bpf.MapType { return m.mtype }
func (m *mockMap) KeySize() int      { return m.keySize }
func (m *mockMap) ValueSize() int    { return m.valSize }
func (m *mockMap) Iterate(fn func(keyBytes, valueBytes []byte) (bool, error)) error {
	for _, e := range m.entries {
		cont, err := fn(e[0], e[1])
		if err != nil {
			return err
		}

		if !cont {
			break
		}
	}

	return nil
}

func (m *mockMap) LookupRaw(keyBytes []byte) ([]byte, error) {
	for _, e := range m.entries {
		if bytes.Equal(e[0], keyBytes) {
			return e[1], nil
		}
	}

	return nil, nil
}

func (m *mockMap) UnderlyingMap() *bpf.Map {
	// mockMap does not have an underlying *bpf.Map; return nil to use fallback
	return nil
}

// fakeMaps mirrors the real struct tags used by the generated code.
type fakeMaps struct {
	DownlinkRouteStats MapHandle `ebpf:"downlink_route_stats"`
	DownlinkStatistics MapHandle `ebpf:"downlink_statistics"`
	FarMap             MapHandle `ebpf:"far_map"`
	FlowStats          MapHandle `ebpf:"flow_stats"`
	NatCt              MapHandle `ebpf:"nat_ct"`
	NoNeighMap         MapHandle `ebpf:"no_neigh_map"`
	NocpMap            MapHandle `ebpf:"nocp_map"`
	PdrsDownlinkIp4    MapHandle `ebpf:"pdrs_downlink_ip4"`
	PdrsDownlinkIp6    MapHandle `ebpf:"pdrs_downlink_ip6"`
	PdrsUplink         MapHandle `ebpf:"pdrs_uplink"`
	QerMap             MapHandle `ebpf:"qer_map"`
	UplinkRouteStats   MapHandle `ebpf:"uplink_route_stats"`
	UplinkStatistics   MapHandle `ebpf:"uplink_statistics"`
	UrrMap             MapHandle `ebpf:"urr_map"`
}

func TestDumpAll_WritesMetaAndNDJSON(t *testing.T) {
	// prepare a pdr info entry
	pdr := ebpf.N3N6EntrypointPdrInfo{LocalSeid: 1, Imsi: 2, PdrId: 3}
	key, _ := json.Marshal(uint32(7))
	val, _ := json.Marshal(pdr)

	fake := &fakeMaps{
		PdrsUplink: &mockMap{mtype: bpf.Hash, keySize: 4, valSize: len(val), entries: [][2][]byte{{key, val}}},
	}

	var buf bytes.Buffer

	tw := tar.NewWriter(&buf)

	opts := DumpOptions{Exclude: []string{"nat_ct"}, MaxEntriesPerMap: 0}

	metas, err := dumpMapsFromStruct(fake, opts, tw)
	if err != nil {
		t.Fatalf("dump failed: %v", err)
	}

	if len(metas) == 0 {
		t.Fatalf("expected some metadata entries, got none")
	}

	// read tar entries and find pdrs_uplink file
	tr := tar.NewReader(bytes.NewReader(buf.Bytes()))
	found := false

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			t.Fatalf("tar read error: %v", err)
		}

		if hdr.Name == "bpf/pdrs_uplink.ndjson.gz" {
			found = true

			gz, err := gzip.NewReader(tr)
			if err != nil {
				t.Fatalf("gzip reader: %v", err)
			}

			dec := json.NewDecoder(gz)

			var v map[string]any
			if err := dec.Decode(&v); err != nil && err != io.EOF {
				t.Fatalf("decode error: %v", err)
			}

			_ = gz.Close()
		}
	}

	if !found {
		t.Fatalf("expected pdrs_uplink dump in tar")
	}
}

func TestExcludeAndRingbufSkip(t *testing.T) {
	// create fake maps with an excluded pdrs_uplink and a ringbuf nocp_map
	key, _ := json.Marshal(uint32(1))
	val, _ := json.Marshal(map[string]int{"x": 1})

	fake := &fakeMaps{
		PdrsUplink: &mockMap{mtype: bpf.Hash, keySize: 4, valSize: len(val), entries: [][2][]byte{{key, val}}},
		NocpMap:    &mockMap{mtype: bpf.RingBuf, keySize: 0, valSize: 0, entries: nil},
	}

	var buf bytes.Buffer

	tw := tar.NewWriter(&buf)
	opts := DumpOptions{Exclude: []string{"pdrs_uplink", "nat_ct"}, MaxEntriesPerMap: 0}

	metas, err := dumpMapsFromStruct(fake, opts, tw)
	if err != nil {
		t.Fatalf("dump failed: %v", err)
	}

	foundExcluded := false
	foundRing := false

	for _, m := range metas {
		if m.Name == "pdrs_uplink" && m.Error == "excluded" {
			foundExcluded = true
		}

		if m.Name == "nocp_map" && m.Error == "ring buffer maps cannot be iterated" {
			foundRing = true
		}
	}

	if !foundExcluded {
		t.Fatalf("excluded map not recorded in metadata")
	}

	if !foundRing {
		t.Fatalf("ringbuf map not recorded in metadata")
	}
}

// TestTruncation ensures DumpAll truncates map iteration when MaxEntriesPerMap is set
func TestTruncation(t *testing.T) {
	// prepare 5 entries, but set MaxEntriesPerMap to 2
	var entries [][2][]byte

	for i := range 5 {
		k, _ := json.Marshal(uint32(i))
		v, _ := json.Marshal(map[string]int{"v": i})
		entries = append(entries, [2][]byte{k, v})
	}

	// local fake struct with ebpf tag matching desired map name
	type truncStruct struct {
		TestTrunc MapHandle `ebpf:"test_trunc_map"`
	}

	fake := &truncStruct{
		TestTrunc: &mockMap{mtype: bpf.Hash, keySize: 4, valSize: len(entries[0][1]), entries: entries},
	}

	var buf bytes.Buffer

	tw := tar.NewWriter(&buf)
	opts := DumpOptions{MaxEntriesPerMap: 2}

	metas, err := dumpMapsFromStruct(fake, opts, tw)
	if err != nil {
		t.Fatalf("dump failed: %v", err)
	}

	var metaFound *MapMetadata

	for i := range metas {
		if metas[i].Name == "test_trunc_map" {
			metaFound = &metas[i]
			break
		}
	}

	if metaFound == nil {
		t.Fatalf("metadata for test_trunc_map not found")
	}

	if !metaFound.Truncated {
		t.Fatalf("expected Truncated=true, got false")
	}

	if metaFound.NumEntriesReported != opts.MaxEntriesPerMap {
		t.Fatalf("expected NumEntriesReported=%d, got %d", opts.MaxEntriesPerMap, metaFound.NumEntriesReported)
	}

	// inspect tar and count lines in ndjson.gz for the map
	tr := tar.NewReader(bytes.NewReader(buf.Bytes()))
	found := false

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			t.Fatalf("tar read error: %v", err)
		}

		if hdr.Name == "bpf/test_trunc_map.ndjson.gz" {
			found = true

			gz, err := gzip.NewReader(tr)
			if err != nil {
				t.Fatalf("gzip reader: %v", err)
			}

			scanner := bufio.NewScanner(gz)

			var lines int

			for scanner.Scan() {
				if len(scanner.Bytes()) > 0 {
					lines++
				}
			}

			if err := scanner.Err(); err != nil {
				t.Fatalf("scanner error: %v", err)
			}

			_ = gz.Close()

			if lines != opts.MaxEntriesPerMap {
				t.Fatalf("expected %d ndjson lines, got %d", opts.MaxEntriesPerMap, lines)
			}
		}
	}

	if !found {
		t.Fatalf("expected test_trunc_map dump in tar")
	}
}

// TestPerCPUOutput ensures per-CPU maps are decoded into a 'cpus' array in NDJSON
func TestPerCPUOutput(t *testing.T) {
	// per-CPU sample values
	cpuVals := []uint64{10, 20, 30}
	raw, _ := json.Marshal(cpuVals)

	type pcpuStruct struct {
		TestPerCPU MapHandle `ebpf:"test_percpu"`
	}

	fake := &pcpuStruct{
		TestPerCPU: &mockMap{mtype: bpf.PerCPUArray, keySize: 4, valSize: len(raw), entries: [][2][]byte{{[]byte{0, 0, 0, 0}, raw}}},
	}

	var buf bytes.Buffer

	tw := tar.NewWriter(&buf)
	opts := DumpOptions{MaxEntriesPerMap: 0}

	metas, err := dumpMapsFromStruct(fake, opts, tw)
	if err != nil {
		t.Fatalf("dump failed: %v", err)
	}

	_ = metas

	// read tar and find test_percpu.ndjson.gz
	tr := tar.NewReader(bytes.NewReader(buf.Bytes()))
	found := false

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			t.Fatalf("tar read error: %v", err)
		}

		if hdr.Name == "bpf/test_percpu.ndjson.gz" {
			found = true

			gz, err := gzip.NewReader(tr)
			if err != nil {
				t.Fatalf("gzip reader: %v", err)
			}
			// read first non-empty line
			dec := json.NewDecoder(gz)

			var v map[string]any
			if err := dec.Decode(&v); err != nil {
				t.Fatalf("json decode error: %v", err)
			}

			_ = gz.Close()

			cpusRaw, ok := v["cpus"]
			if !ok {
				t.Fatalf("expected 'cpus' field in ndjson entry")
			}

			arr, ok := cpusRaw.([]any)
			if !ok {
				t.Fatalf("cpus field not array, got %T", cpusRaw)
			}

			if len(arr) != len(cpuVals) {
				t.Fatalf("expected %d cpus, got %d", len(cpuVals), len(arr))
			}

			for i := range arr {
				// JSON numbers are float64
				if float64(cpuVals[i]) != arr[i].(float64) {
					t.Fatalf("unexpected cpu value at %d: expected %v got %v", i, cpuVals[i], arr[i])
				}
			}
		}
	}

	if !found {
		t.Fatalf("expected test_percpu dump in tar")
	}
}
