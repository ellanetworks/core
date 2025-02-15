package ebpf

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/RoaringBitmap/roaring"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/upf/config"

	"github.com/cilium/ebpf"
)

//
// Supported BPF_CFLAGS:
// 	- ENABLE_LOG:
//		- enables debug output to tracepipe (`bpftool prog tracelog`)
// 	- ENABLE_ROUTE_CACHE
//		- enable routing decision cache
//

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -cflags "$BPF_CFLAGS" -target bpf IpEntrypoint 	xdp/n3n6_entrypoint.c -- -I. -O2 -Wall -g

type ProfileInfo struct {
	Count   uint64
	TotalNs uint64
}

type BpfObjects struct {
	IpEntrypointObjects

	FarIdTracker *IdTracker
	QerIdTracker *IdTracker
}

func NewBpfObjects() *BpfObjects {
	return &BpfObjects{
		FarIdTracker: NewIdTracker(config.Conf.FarMapSize),
		QerIdTracker: NewIdTracker(config.Conf.QerMapSize),
	}
}

func (bpfObjects *BpfObjects) Load() error {
	pinPath := "/sys/fs/bpf/upf_pipeline"
	if err := os.MkdirAll(pinPath, 0o750); err != nil {
		logger.UpfLog.Infof("failed to create bpf fs subpath: %+v", err)
		return err
	}

	collectionOptions := ebpf.CollectionOptions{
		Maps: ebpf.MapOptions{
			// Pin the map to the BPF filesystem and configure the
			// library to automatically re-write it in the BPF
			// program, so it can be re-used if it already exists or
			// create it if not
			PinPath: pinPath,
		},
	}

	return LoadAllObjects(&collectionOptions,
		Loader{LoadIpEntrypointObjects, &bpfObjects.IpEntrypointObjects})
}

func (bpfObjects *BpfObjects) Close() error {
	return CloseAllObjects(
		&bpfObjects.IpEntrypointObjects,
	)
}

type (
	LoaderFunc func(obj interface{}, opts *ebpf.CollectionOptions) error
	Loader     struct {
		LoaderFunc
		object interface{}
	}
)

func LoadAllObjects(opts *ebpf.CollectionOptions, loaders ...Loader) error {
	for _, loader := range loaders {
		if err := loader.LoaderFunc(loader.object, opts); err != nil {
			return err
		}
	}
	return nil
}

func CloseAllObjects(closers ...io.Closer) error {
	for _, closer := range closers {
		if err := closer.Close(); err != nil {
			return err
		}
	}
	return nil
}

type IdTracker struct {
	bitmap  *roaring.Bitmap
	maxSize uint32
}

func NewIdTracker(size uint32) *IdTracker {
	newBitmap := roaring.NewBitmap()
	newBitmap.Flip(0, uint64(size))

	return &IdTracker{
		bitmap:  newBitmap,
		maxSize: size,
	}
}

func (t *IdTracker) GetNext() (next uint32, err error) {
	i := t.bitmap.Iterator()
	if i.HasNext() {
		next := i.Next()
		t.bitmap.Remove(next)
		return next, nil
	}

	return 0, errors.New("pool is empty")
}

func (t *IdTracker) Release(id uint32) {
	if id >= t.maxSize {
		return
	}

	t.bitmap.Add(id)
}

// Define the descriptive names for each profile step.
// The order must match the order defined in the C enum.
var stepNames = []string{
	"UPF IP Entrypoint",     // key 0: STEP_UPF_IP_ENTRYPOINT
	"Process Packet",        // key 1: STEP_PROCESS_PACKET
	"Parse Ethernet",        // key 2: STEP_PARSE_ETHERNET
	"Handle IPv4",           // key 3: STEP_HANDLE_IP4
	"Handle IPv6",           // key 4: STEP_HANDLE_IP6
	"Handle GTPU",           // key 5: STEP_HANDLE_GTPU
	"Handle GTP Packet",     // key 6: STEP_HANDLE_GTP_PACKET
	"Handle N6 Packet IPv4", // key 7: STEP_HANDLE_N6_PACKET_IP4
	"Handle N6 Packet IPv6", // key 8: STEP_HANDLE_N6_PACKET_IP6
	"Send to GTP Tunnel",    // key 9: STEP_SEND_TO_GTP_TUNNEL
}

func PrintProfileData(m *ebpf.Map) {
	stepNames := []string{
		"UPF IP Entrypoint",     // key 0: STEP_UPF_IP_ENTRYPOINT
		"Process Packet",        // key 1: STEP_PROCESS_PACKET
		"Parse Ethernet",        // key 2: STEP_PARSE_ETHERNET
		"Handle IPv4",           // key 3: STEP_HANDLE_IP4
		"Handle IPv6",           // key 4: STEP_HANDLE_IP6
		"Handle GTPU",           // key 5: STEP_HANDLE_GTPU
		"Handle GTP Packet",     // key 6: STEP_HANDLE_GTP_PACKET
		"Handle N6 Packet IPv4", // key 7: STEP_HANDLE_N6_PACKET_IP4
		"Handle N6 Packet IPv6", // key 8: STEP_HANDLE_N6_PACKET_IP6
		"Send to GTP Tunnel",    // key 9: STEP_SEND_TO_GTP_TUNNEL
	}
	numSteps := uint32(len(stepNames))

	// Print table header.
	fmt.Println("--------------------------------------------------------------------------------")
	fmt.Printf("| %-24s | %-10s | %-10s | %-10s |\n", "Step", "Count", "TotalNs", "AvgNs")
	fmt.Println("--------------------------------------------------------------------------------")

	// Iterate over each step and aggregate per-CPU values.
	for key := uint32(0); key < numSteps; key++ {
		var perCPU []ProfileInfo
		if err := m.Lookup(key, &perCPU); err != nil {
			fmt.Printf("| %-24s | %-10s | %-10s | %-10s |\n", stepNames[key], "N/A", "N/A", "N/A")
			continue
		}

		var agg ProfileInfo
		for _, v := range perCPU {
			agg.Count += v.Count
			agg.TotalNs += v.TotalNs
		}
		var avg uint64
		if agg.Count > 0 {
			avg = agg.TotalNs / agg.Count
		}
		fmt.Printf("| %-24s | %-10d | %-10d | %-10d |\n", stepNames[key], agg.Count, agg.TotalNs, avg)
	}
	fmt.Println("--------------------------------------------------------------------------------")
}
