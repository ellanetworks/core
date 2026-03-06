package bpfdump

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"time"

	bpf "github.com/cilium/ebpf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"go.uber.org/zap"
)

type DumpOptions struct {
	Exclude          []string
	MaxEntriesPerMap int // 0 = unlimited
}

type MapMetadata struct {
	Name               string    `json:"name"`
	Type               string    `json:"type"`
	KeySize            int       `json:"keySize"`
	ValueSize          int       `json:"valueSize"`
	NumEntriesReported int       `json:"numEntriesReported"`
	Truncated          bool      `json:"truncated"`
	SnapshotTime       time.Time `json:"snapshotTime"`
	Error              string    `json:"error,omitempty"`
}

// DumpAll inspects the generated N3N6EntrypointMaps structure and dumps each
// map into the provided tar.Writer. For testing, use dumpMapsFromStruct to
// pass a fake struct with the same `ebpf` tags but fields typed as MapHandle.
func DumpAll(ctx context.Context, bpfObjects *ebpf.BpfObjects, opts DumpOptions, tw *tar.Writer) ([]MapMetadata, error) {
	if bpfObjects == nil {
		return nil, nil
	}

	return dumpMapsFromStruct(&bpfObjects.N3N6EntrypointMaps, opts, tw)
}

// MapHandle abstracts an eBPF map for production and tests.
type MapHandle interface {
	Type() bpf.MapType
	KeySize() int
	ValueSize() int
	// Iterate calls fn for each entry; if fn returns (false, nil) iteration stops
	Iterate(fn func(keyBytes, valueBytes []byte) (bool, error)) error
	// LookupRaw returns raw value bytes for a key
	LookupRaw(keyBytes []byte) ([]byte, error)
	// UnderlyingMap returns the underlying *bpf.Map if available for typed operations
	UnderlyingMap() *bpf.Map
}

// realMapAdapter adapts *bpf.Map to MapHandle with typed operations.
type realMapAdapter struct{ m *bpf.Map }

func (r *realMapAdapter) Type() bpf.MapType { return r.m.Type() }
func (r *realMapAdapter) KeySize() int      { return int(r.m.KeySize()) }
func (r *realMapAdapter) ValueSize() int    { return int(r.m.ValueSize()) }
func (r *realMapAdapter) UnderlyingMap() *bpf.Map {
	return r.m
}

func (r *realMapAdapter) Iterate(fn func(keyBytes, valueBytes []byte) (bool, error)) error {
	iter := r.m.Iterate()
	k := make([]byte, r.KeySize())

	v := make([]byte, r.ValueSize())
	for iter.Next(&k, &v) {
		kcpy := make([]byte, len(k))
		copy(kcpy, k)

		vcpy := make([]byte, len(v))
		copy(vcpy, v)

		cont, err := fn(kcpy, vcpy)
		if err != nil {
			return err
		}

		if !cont {
			break
		}

		k = make([]byte, r.KeySize())
		v = make([]byte, r.ValueSize())
	}

	if err := iter.Err(); err != nil {
		return err
	}

	return nil
}

func (r *realMapAdapter) LookupRaw(keyBytes []byte) ([]byte, error) {
	val := make([]byte, r.ValueSize())
	if err := r.m.Lookup(&keyBytes, &val); err != nil {
		return nil, err
	}

	return val, nil
}

// IPv6Addr marshals an In6Addr as a hex string for JSON.
type IPv6Addr string

func marshalIn6Addr(addr *ebpf.N3N6EntrypointIn6Addr) IPv6Addr {
	return IPv6Addr(hex.EncodeToString(addr.In6U.U6Addr8[:]))
}

// decodeFallback decodes raw bytes into a generic JSON value, falling back to hex if decoding fails.
func decodeFallback(rawBytes []byte) any {
	var v any
	if err := json.Unmarshal(rawBytes, &v); err == nil {
		return v
	}
	// If decoding fails, return as hex string
	return hex.EncodeToString(rawBytes)
}

// typedDecoder provides typed iteration/lookup for eBPF maps.
// Each decoder knows how to decode a specific map's key/value types.
// Returns (count, truncated, error)
type typedDecoder func(m *bpf.Map, mapName string, opts DumpOptions, enc *json.Encoder) (int, bool, error)

// getMapDecoder returns a typed decoder for the given map name.
// For production code (when called with a real *bpf.Map), these decoders
// use typed iteration. For tests, the mockMap falls back to generic byte iteration.
func getMapDecoder(mapName string) typedDecoder {
	switch mapName {
	case "downlink_route_stats":
		return decodeDownlinkRouteStats
	case "downlink_statistics":
		return decodeDownlinkStatistics
	case "far_map":
		return decodeFarMap
	case "flow_stats":
		return decodeFlowStats
	case "nat_ct":
		return decodeNatCt
	case "pdrs_downlink_ip4":
		return decodePdrsDownlinkIp4
	case "pdrs_downlink_ip6":
		return decodePdrsDownlinkIp6
	case "pdrs_uplink":
		return decodePdrsUplink
	case "qer_map":
		return decodeQerMap
	case "uplink_route_stats":
		return decodeUplinkRouteStats
	case "uplink_statistics":
		return decodeUplinkStatistics
	case "urr_map":
		return decodeUrrMap
	case "no_neigh_map", "nocp_map":
		// These are not typically inspected; provide a fallback
		return decodeGenericHash
	default:
		return decodeGenericHash
	}
}

// decodeDownlinkRouteStats decodes downlink_route_stats (PerCPUArray).
func decodeDownlinkRouteStats(m *bpf.Map, mapName string, opts DumpOptions, enc *json.Encoder) (int, bool, error) {
	key := uint32(0)

	var vals []ebpf.N3N6EntrypointRouteStat

	if err := m.Lookup(&key, &vals); err != nil {
		return 0, false, fmt.Errorf("lookup failed: %w", err)
	}

	// Convert per-CPU slice into JSON-friendly format
	cpusData := make([]any, len(vals))
	for i, v := range vals {
		cpusData[i] = v
	}

	if err := enc.Encode(map[string]any{"key": 0, "cpus": cpusData}); err != nil {
		return 0, false, fmt.Errorf("encode failed: %w", err)
	}

	return 1, false, nil
}

// decodeDownlinkStatistics decodes downlink_statistics (PerCPUArray).
func decodeDownlinkStatistics(m *bpf.Map, mapName string, opts DumpOptions, enc *json.Encoder) (int, bool, error) {
	key := uint32(0)

	var vals []ebpf.N3N6EntrypointUpfStatistic

	if err := m.Lookup(&key, &vals); err != nil {
		return 0, false, fmt.Errorf("lookup failed: %w", err)
	}

	cpusData := make([]any, len(vals))
	for i, v := range vals {
		cpusData[i] = v
	}

	if err := enc.Encode(map[string]any{"key": 0, "cpus": cpusData}); err != nil {
		return 0, false, fmt.Errorf("encode failed: %w", err)
	}

	return 1, false, nil
}

// decodeFarMap decodes far_map (Array of N3N6EntrypointFarInfo).
func decodeFarMap(m *bpf.Map, mapName string, opts DumpOptions, enc *json.Encoder) (int, bool, error) {
	iter := m.Iterate()

	var (
		key   uint32
		val   ebpf.N3N6EntrypointFarInfo
		count int
	)

	for iter.Next(&key, &val) {
		if err := enc.Encode(map[string]any{"key": key, "value": val}); err != nil {
			return count, false, fmt.Errorf("encode failed: %w", err)
		}

		count++
		if opts.MaxEntriesPerMap > 0 && count >= opts.MaxEntriesPerMap {
			return count, true, nil
		}
	}

	if err := iter.Err(); err != nil {
		return count, false, fmt.Errorf("iterate error: %w", err)
	}

	return count, false, nil
}

// decodeFlowStats decodes flow_stats (LRU_HASH with Flow key and FlowStats value).
func decodeFlowStats(m *bpf.Map, mapName string, opts DumpOptions, enc *json.Encoder) (int, bool, error) {
	iter := m.Iterate()

	var (
		key   ebpf.N3N6EntrypointFlow
		val   ebpf.N3N6EntrypointFlowStats
		count int
	)

	for iter.Next(&key, &val) {
		if err := enc.Encode(map[string]any{"key": key, "value": val}); err != nil {
			return count, false, fmt.Errorf("encode failed: %w", err)
		}

		count++
		if opts.MaxEntriesPerMap > 0 && count >= opts.MaxEntriesPerMap {
			return count, true, nil
		}
	}

	if err := iter.Err(); err != nil {
		return count, false, fmt.Errorf("iterate error: %w", err)
	}

	return count, false, nil
}

// decodeNatCt decodes nat_ct (LRU_HASH with FiveTuple key and NatEntry value).
func decodeNatCt(m *bpf.Map, mapName string, opts DumpOptions, enc *json.Encoder) (int, bool, error) {
	iter := m.Iterate()

	var (
		key   ebpf.N3N6EntrypointFiveTuple
		val   ebpf.N3N6EntrypointNatEntry
		count int
	)

	for iter.Next(&key, &val) {
		if err := enc.Encode(map[string]any{"key": key, "value": val}); err != nil {
			return count, false, fmt.Errorf("encode failed: %w", err)
		}

		count++
		if opts.MaxEntriesPerMap > 0 && count >= opts.MaxEntriesPerMap {
			return count, true, nil
		}
	}

	if err := iter.Err(); err != nil {
		return count, false, fmt.Errorf("iterate error: %w", err)
	}

	return count, false, nil
}

// decodePdrsDownlinkIp4 decodes pdrs_downlink_ip4 (Hash with uint32 key and PdrInfo value).
func decodePdrsDownlinkIp4(m *bpf.Map, mapName string, opts DumpOptions, enc *json.Encoder) (int, bool, error) {
	iter := m.Iterate()

	var (
		key   uint32
		val   ebpf.N3N6EntrypointPdrInfo
		count int
	)

	for iter.Next(&key, &val) {
		if err := enc.Encode(map[string]any{"key": key, "value": val}); err != nil {
			return count, false, fmt.Errorf("encode failed: %w", err)
		}

		count++
		if opts.MaxEntriesPerMap > 0 && count >= opts.MaxEntriesPerMap {
			return count, true, nil
		}
	}

	if err := iter.Err(); err != nil {
		return count, false, fmt.Errorf("iterate error: %w", err)
	}

	return count, false, nil
}

// decodePdrsDownlinkIp6 decodes pdrs_downlink_ip6 (Hash with In6Addr key and PdrInfo value).
func decodePdrsDownlinkIp6(m *bpf.Map, mapName string, opts DumpOptions, enc *json.Encoder) (int, bool, error) {
	iter := m.Iterate()

	var (
		key   ebpf.N3N6EntrypointIn6Addr
		val   ebpf.N3N6EntrypointPdrInfo
		count int
	)

	for iter.Next(&key, &val) {
		// Marshal IPv6 address as hex string for JSON
		keyStr := marshalIn6Addr(&key)
		if err := enc.Encode(map[string]any{"key": keyStr, "value": val}); err != nil {
			return count, false, fmt.Errorf("encode failed: %w", err)
		}

		count++
		if opts.MaxEntriesPerMap > 0 && count >= opts.MaxEntriesPerMap {
			return count, true, nil
		}
	}

	if err := iter.Err(); err != nil {
		return count, false, fmt.Errorf("iterate error: %w", err)
	}

	return count, false, nil
}

// decodePdrsUplink decodes pdrs_uplink (Hash with uint32 key and PdrInfo value).
func decodePdrsUplink(m *bpf.Map, mapName string, opts DumpOptions, enc *json.Encoder) (int, bool, error) {
	iter := m.Iterate()

	var (
		key   uint32
		val   ebpf.N3N6EntrypointPdrInfo
		count int
	)

	for iter.Next(&key, &val) {
		if err := enc.Encode(map[string]any{"key": key, "value": val}); err != nil {
			return count, false, fmt.Errorf("encode failed: %w", err)
		}

		count++
		if opts.MaxEntriesPerMap > 0 && count >= opts.MaxEntriesPerMap {
			return count, true, nil
		}
	}

	if err := iter.Err(); err != nil {
		return count, false, fmt.Errorf("iterate error: %w", err)
	}

	return count, false, nil
}

// decodeQerMap decodes qer_map (Array of N3N6EntrypointQerInfo).
func decodeQerMap(m *bpf.Map, mapName string, opts DumpOptions, enc *json.Encoder) (int, bool, error) {
	iter := m.Iterate()

	var (
		key   uint32
		val   ebpf.N3N6EntrypointQerInfo
		count int
	)

	for iter.Next(&key, &val) {
		if err := enc.Encode(map[string]any{"key": key, "value": val}); err != nil {
			return count, false, fmt.Errorf("encode failed: %w", err)
		}

		count++
		if opts.MaxEntriesPerMap > 0 && count >= opts.MaxEntriesPerMap {
			return count, true, nil
		}
	}

	if err := iter.Err(); err != nil {
		return count, false, fmt.Errorf("iterate error: %w", err)
	}

	return count, false, nil
}

// decodeUplinkRouteStats decodes uplink_route_stats (PerCPUArray).
func decodeUplinkRouteStats(m *bpf.Map, mapName string, opts DumpOptions, enc *json.Encoder) (int, bool, error) {
	key := uint32(0)

	var vals []ebpf.N3N6EntrypointRouteStat

	if err := m.Lookup(&key, &vals); err != nil {
		return 0, false, fmt.Errorf("lookup failed: %w", err)
	}

	cpusData := make([]any, len(vals))
	for i, v := range vals {
		cpusData[i] = v
	}

	if err := enc.Encode(map[string]any{"key": 0, "cpus": cpusData}); err != nil {
		return 0, false, fmt.Errorf("encode failed: %w", err)
	}

	return 1, false, nil
}

// decodeUplinkStatistics decodes uplink_statistics (PerCPUArray).
func decodeUplinkStatistics(m *bpf.Map, mapName string, opts DumpOptions, enc *json.Encoder) (int, bool, error) {
	key := uint32(0)

	var vals []ebpf.N3N6EntrypointUpfStatistic

	if err := m.Lookup(&key, &vals); err != nil {
		return 0, false, fmt.Errorf("lookup failed: %w", err)
	}

	cpusData := make([]any, len(vals))
	for i, v := range vals {
		cpusData[i] = v
	}

	if err := enc.Encode(map[string]any{"key": 0, "cpus": cpusData}); err != nil {
		return 0, false, fmt.Errorf("encode failed: %w", err)
	}

	return 1, false, nil
}

// decodeUrrMap decodes urr_map (PerCPUHash with uint32 key and uint64 values).
func decodeUrrMap(m *bpf.Map, mapName string, opts DumpOptions, enc *json.Encoder) (int, bool, error) {
	iter := m.Iterate()

	var (
		key   uint32
		vals  []uint64
		count int
	)

	for iter.Next(&key, &vals) {
		cpusData := make([]any, len(vals))
		for i, v := range vals {
			cpusData[i] = v
		}

		if err := enc.Encode(map[string]any{"key": key, "cpus": cpusData}); err != nil {
			return count, false, fmt.Errorf("encode failed: %w", err)
		}

		count++
		if opts.MaxEntriesPerMap > 0 && count >= opts.MaxEntriesPerMap {
			return count, true, nil
		}
	}

	if err := iter.Err(); err != nil {
		return count, false, fmt.Errorf("iterate error: %w", err)
	}

	return count, false, nil
}

// decodeGenericHash is a fallback for unknown Hash/LRUHash maps.
func decodeGenericHash(m *bpf.Map, mapName string, opts DumpOptions, enc *json.Encoder) (int, bool, error) {
	iter := m.Iterate()

	var (
		key   uint32
		val   any
		count int
	)

	for iter.Next(&key, &val) {
		if err := enc.Encode(map[string]any{"key": key, "value": decodeFallback(fmt.Append(nil, val))}); err != nil {
			return count, false, fmt.Errorf("encode failed: %w", err)
		}

		count++
		if opts.MaxEntriesPerMap > 0 && count >= opts.MaxEntriesPerMap {
			return count, true, nil
		}
	}

	if err := iter.Err(); err != nil {
		return count, false, fmt.Errorf("iterate error: %w", err)
	}

	return count, false, nil
}

// dumpMapsFromStruct inspects a struct value with `ebpf` tags and dumps maps.
// The struct may be the real ebpf.N3N6EntrypointMaps or a test fake where
// fields are of type MapHandle.
func dumpMapsFromStruct(mapsStruct any, opts DumpOptions, tw *tar.Writer) ([]MapMetadata, error) {
	var metas []MapMetadata

	v := reflect.ValueOf(mapsStruct).Elem()
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		ft := t.Field(i)
		mapName := ft.Tag.Get("ebpf")

		meta := MapMetadata{
			Name:         mapName,
			SnapshotTime: time.Now().UTC(),
		}

		if slices.Contains(opts.Exclude, mapName) {
			meta.Error = "excluded"
			metas = append(metas, meta)

			logger.UpfLog.Info("Skipping excluded map", zap.String("map", mapName))

			continue
		}

		if field.IsNil() {
			meta.Error = "map not present"
			metas = append(metas, meta)

			logger.UpfLog.Warn("Map is not present (nil)", zap.String("map", mapName))

			continue
		}

		var mh MapHandle

		if field.CanInterface() {
			// first, try if the concrete value itself implements MapHandle (tests)
			if val, ok := field.Interface().(MapHandle); ok {
				mh = val
			}
			// otherwise, if it's a *bpf.Map, wrap it
			if mh == nil {
				if pm, ok := field.Interface().(*bpf.Map); ok && pm != nil {
					mh = &realMapAdapter{m: pm}
				}
			}
		}

		if mh == nil {
			meta.Error = "map not present"
			metas = append(metas, meta)

			logger.UpfLog.Warn("Map handle not available", zap.String("map", mapName))

			continue
		}

		mtype := mh.Type()
		meta.Type = mtype.String()
		meta.KeySize = mh.KeySize()
		meta.ValueSize = mh.ValueSize()

		if mtype == bpf.RingBuf {
			meta.Error = "ring buffer maps cannot be iterated"
			metas = append(metas, meta)

			logger.UpfLog.Warn("Skipping ring buffer map", zap.String("map", mapName))

			continue
		}

		logger.UpfLog.Info("Starting dump for map", zap.String("map", mapName))

		// Prepare an in-memory buffer to hold the gzipped NDJSON for this map so
		// we can compute its size for the tar header before writing.
		var mapBuf bytes.Buffer

		gz := gzip.NewWriter(&mapBuf)
		enc := json.NewEncoder(gz)

		var (
			count     int
			err       error
			truncated bool
		)

		// Try typed decoding for production maps (when we have a real *bpf.Map).
		// For tests with mockMap, this will fall back to generic byte iteration.

		if underlyingMap := mh.UnderlyingMap(); underlyingMap != nil {
			decoder := getMapDecoder(mapName)
			count, truncated, err = decoder(underlyingMap, mapName, opts, enc)
		} else {
			// Fallback for test mocks: use generic byte iteration
			err = mh.Iterate(func(keyBytes, valueBytes []byte) (bool, error) {
				switch mtype {
				case bpf.Array:
					var key uint32
					if err := binary.Read(bytes.NewReader(keyBytes), binary.LittleEndian, &key); err == nil {
						_ = enc.Encode(map[string]any{"key": key})
					} else {
						_ = enc.Encode(map[string]any{"key_raw": keyBytes})
					}
				case bpf.Hash, bpf.LRUHash:
					var key uint32
					if err := binary.Read(bytes.NewReader(keyBytes), binary.LittleEndian, &key); err == nil {
						val := decodeFallback(valueBytes)
						_ = enc.Encode(map[string]any{"key": key, "value": val})
					} else {
						_ = enc.Encode(map[string]any{"key_raw": keyBytes, "value": decodeFallback(valueBytes)})
					}
				case bpf.PerCPUArray, bpf.PerCPUHash:
					val := decodeFallback(valueBytes)
					if mtype == bpf.PerCPUArray {
						_ = enc.Encode(map[string]any{"key": 0, "cpus": val})
					} else {
						var key uint32
						if err := binary.Read(bytes.NewReader(keyBytes), binary.LittleEndian, &key); err == nil {
							_ = enc.Encode(map[string]any{"key": key, "cpus": val})
						} else {
							_ = enc.Encode(map[string]any{"key_raw": keyBytes, "cpus": val})
						}
					}

					count = 1

					return false, nil
				default:
					_ = enc.Encode(map[string]any{"unsupported_map_type": mtype.String()})
				}

				count++
				if opts.MaxEntriesPerMap > 0 && count >= opts.MaxEntriesPerMap {
					truncated = true
					return false, nil
				}

				return true, nil
			})
		}

		if err != nil {
			meta.Error = fmt.Sprintf("iterate error: %v", err)
			logger.UpfLog.Error("iterate error", zap.String("map", mapName), zap.Error(err))
		}

		if truncated {
			meta.Truncated = true

			logger.UpfLog.Warn("map truncated due to MaxEntriesPerMap", zap.String("map", mapName))
		}

		if err := gz.Close(); err != nil {
			logger.UpfLog.Error("failed to close gzip writer", zap.String("map", mapName), zap.Error(err))
		}

		// Now write the tar header with the exact size and then the gzipped bytes.
		hdr := &tar.Header{
			Name:    fmt.Sprintf("bpf/%s.ndjson.gz", mapName),
			Mode:    0o644,
			ModTime: time.Now(),
			Size:    int64(mapBuf.Len()),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			meta.Error = fmt.Sprintf("failed to write tar header: %v", err)
			metas = append(metas, meta)

			logger.UpfLog.Error("failed to write tar header", zap.String("map", mapName), zap.Error(err))

			continue
		}

		if _, err := tw.Write(mapBuf.Bytes()); err != nil {
			meta.Error = fmt.Sprintf("failed to write tar content: %v", err)
			metas = append(metas, meta)

			logger.UpfLog.Error("failed to write tar content", zap.String("map", mapName), zap.Error(err))

			continue
		}

		logger.UpfLog.Info("Finished dump for map", zap.String("map", mapName), zap.Int("entries", count))

		meta.NumEntriesReported = count
		metas = append(metas, meta)
	}

	// After all per-map entries, write final manifest into the tar archive
	// as bpf/_metadata.json containing the JSON array of MapMetadata.
	var metaBuf bytes.Buffer

	enc := json.NewEncoder(&metaBuf)
	if err := enc.Encode(metas); err != nil {
		return metas, fmt.Errorf("failed to encode metadata: %w", err)
	}

	hdr := &tar.Header{
		Name:    "bpf/_metadata.json",
		Mode:    0o644,
		ModTime: time.Now(),
		Size:    int64(metaBuf.Len()),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return metas, fmt.Errorf("failed to write metadata tar header: %w", err)
	}

	if _, err := tw.Write(metaBuf.Bytes()); err != nil {
		return metas, fmt.Errorf("failed to write metadata to tar: %w", err)
	}

	return metas, nil
}
