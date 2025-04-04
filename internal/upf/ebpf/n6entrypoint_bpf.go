// Code generated by bpf2go; DO NOT EDIT.

package ebpf

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"

	"github.com/cilium/ebpf"
)

type N6EntrypointFarInfo struct {
	Action                uint8
	OuterHeaderCreation   uint8
	_                     [2]byte
	TeID                  uint32
	Remoteip              uint32
	Localip               uint32
	TransportLevelMarking uint16
	_                     [2]byte
}

type N6EntrypointIn6Addr struct{ In6U struct{ U6Addr8 [16]uint8 } }

type N6EntrypointPdrInfo struct {
	FarId              uint32
	QerId              uint32
	OuterHeaderRemoval uint8
	SdfMode            uint8
	_                  [6]byte
	SdfRules           struct {
		SdfFilter struct {
			Protocol uint8
			_        [15]byte
			SrcAddr  struct {
				Type uint8
				_    [15]byte
				Ip   [16]byte /* uint128 */
				Mask [16]byte /* uint128 */
			}
			SrcPort struct {
				LowerBound uint16
				UpperBound uint16
			}
			_       [12]byte
			DstAddr struct {
				Type uint8
				_    [15]byte
				Ip   [16]byte /* uint128 */
				Mask [16]byte /* uint128 */
			}
			DstPort struct {
				LowerBound uint16
				UpperBound uint16
			}
			_ [12]byte
		}
		OuterHeaderRemoval uint8
		_                  [3]byte
		FarId              uint32
		QerId              uint32
		_                  [4]byte
	}
}

type N6EntrypointQerInfo struct {
	UlGateStatus     uint8
	DlGateStatus     uint8
	Qfi              uint8
	_                [1]byte
	UlMaximumBitrate uint32
	DlMaximumBitrate uint32
	_                [4]byte
	UlStart          uint64
	DlStart          uint64
}

type N6EntrypointRouteStat struct {
	FibLookupIp4Cache     uint64
	FibLookupIp4Ok        uint64
	FibLookupIp4ErrorDrop uint64
	FibLookupIp4ErrorPass uint64
	FibLookupIp6Cache     uint64
	FibLookupIp6Ok        uint64
	FibLookupIp6ErrorDrop uint64
	FibLookupIp6ErrorPass uint64
}

type N6EntrypointUpfN6Statistic struct {
	UpfN6Counters struct{ DlBytes uint64 }
	UpfN6Counter  struct {
		RxN6 uint64
		TxN6 uint64
	}
	XdpActions [8]uint64
}

// LoadN6Entrypoint returns the embedded CollectionSpec for N6Entrypoint.
func LoadN6Entrypoint() (*ebpf.CollectionSpec, error) {
	reader := bytes.NewReader(_N6EntrypointBytes)
	spec, err := ebpf.LoadCollectionSpecFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("can't load N6Entrypoint: %w", err)
	}

	return spec, err
}

// LoadN6EntrypointObjects loads N6Entrypoint and converts it into a struct.
//
// The following types are suitable as obj argument:
//
//	*N6EntrypointObjects
//	*N6EntrypointPrograms
//	*N6EntrypointMaps
//
// See ebpf.CollectionSpec.LoadAndAssign documentation for details.
func LoadN6EntrypointObjects(obj interface{}, opts *ebpf.CollectionOptions) error {
	spec, err := LoadN6Entrypoint()
	if err != nil {
		return err
	}

	return spec.LoadAndAssign(obj, opts)
}

// N6EntrypointSpecs contains maps and programs before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type N6EntrypointSpecs struct {
	N6EntrypointProgramSpecs
	N6EntrypointMapSpecs
	N6EntrypointVariableSpecs
}

// N6EntrypointProgramSpecs contains programs before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type N6EntrypointProgramSpecs struct {
	UpfN6EntrypointFunc *ebpf.ProgramSpec `ebpf:"upf_n6_entrypoint_func"`
}

// N6EntrypointMapSpecs contains maps before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type N6EntrypointMapSpecs struct {
	FarMap            *ebpf.MapSpec `ebpf:"far_map"`
	PdrMapDownlinkIp4 *ebpf.MapSpec `ebpf:"pdr_map_downlink_ip4"`
	PdrMapDownlinkIp6 *ebpf.MapSpec `ebpf:"pdr_map_downlink_ip6"`
	PdrMapUplinkIp4   *ebpf.MapSpec `ebpf:"pdr_map_uplink_ip4"`
	QerMap            *ebpf.MapSpec `ebpf:"qer_map"`
	UpfN6Stat         *ebpf.MapSpec `ebpf:"upf_n6_stat"`
	UpfRouteStat      *ebpf.MapSpec `ebpf:"upf_route_stat"`
}

// N6EntrypointVariableSpecs contains global variables before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type N6EntrypointVariableSpecs struct {
}

// N6EntrypointObjects contains all objects after they have been loaded into the kernel.
//
// It can be passed to LoadN6EntrypointObjects or ebpf.CollectionSpec.LoadAndAssign.
type N6EntrypointObjects struct {
	N6EntrypointPrograms
	N6EntrypointMaps
	N6EntrypointVariables
}

func (o *N6EntrypointObjects) Close() error {
	return _N6EntrypointClose(
		&o.N6EntrypointPrograms,
		&o.N6EntrypointMaps,
	)
}

// N6EntrypointMaps contains all maps after they have been loaded into the kernel.
//
// It can be passed to LoadN6EntrypointObjects or ebpf.CollectionSpec.LoadAndAssign.
type N6EntrypointMaps struct {
	FarMap            *ebpf.Map `ebpf:"far_map"`
	PdrMapDownlinkIp4 *ebpf.Map `ebpf:"pdr_map_downlink_ip4"`
	PdrMapDownlinkIp6 *ebpf.Map `ebpf:"pdr_map_downlink_ip6"`
	PdrMapUplinkIp4   *ebpf.Map `ebpf:"pdr_map_uplink_ip4"`
	QerMap            *ebpf.Map `ebpf:"qer_map"`
	UpfN6Stat         *ebpf.Map `ebpf:"upf_n6_stat"`
	UpfRouteStat      *ebpf.Map `ebpf:"upf_route_stat"`
}

func (m *N6EntrypointMaps) Close() error {
	return _N6EntrypointClose(
		m.FarMap,
		m.PdrMapDownlinkIp4,
		m.PdrMapDownlinkIp6,
		m.PdrMapUplinkIp4,
		m.QerMap,
		m.UpfN6Stat,
		m.UpfRouteStat,
	)
}

// N6EntrypointVariables contains all global variables after they have been loaded into the kernel.
//
// It can be passed to LoadN6EntrypointObjects or ebpf.CollectionSpec.LoadAndAssign.
type N6EntrypointVariables struct {
}

// N6EntrypointPrograms contains all programs after they have been loaded into the kernel.
//
// It can be passed to LoadN6EntrypointObjects or ebpf.CollectionSpec.LoadAndAssign.
type N6EntrypointPrograms struct {
	UpfN6EntrypointFunc *ebpf.Program `ebpf:"upf_n6_entrypoint_func"`
}

func (p *N6EntrypointPrograms) Close() error {
	return _N6EntrypointClose(
		p.UpfN6EntrypointFunc,
	)
}

func _N6EntrypointClose(closers ...io.Closer) error {
	for _, closer := range closers {
		if err := closer.Close(); err != nil {
			return err
		}
	}
	return nil
}

// Do not access this directly.
//
//go:embed n6entrypoint_bpf.o
var _N6EntrypointBytes []byte
