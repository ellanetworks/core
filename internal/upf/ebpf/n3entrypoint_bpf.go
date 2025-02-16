// Code generated by bpf2go; DO NOT EDIT.

package ebpf

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"

	"github.com/cilium/ebpf"
)

type N3EntrypointFarInfo struct {
	Action                uint8
	OuterHeaderCreation   uint8
	_                     [2]byte
	Teid                  uint32
	Remoteip              uint32
	Localip               uint32
	TransportLevelMarking uint16
	_                     [2]byte
}

type N3EntrypointIn6Addr struct{ In6U struct{ U6Addr8 [16]uint8 } }

type N3EntrypointN3FarInfo struct {
	Action                uint8
	OuterHeaderCreation   uint8
	_                     [2]byte
	Teid                  uint32
	Remoteip              uint32
	Localip               uint32
	TransportLevelMarking uint16
	_                     [2]byte
}

type N3EntrypointN3PdrInfo struct {
	FarId              uint32
	QerId              uint32
	OuterHeaderRemoval uint8
	SdfMode            uint8
	_                  [6]byte
	N3SdfRules         struct {
		N3SdfFilter struct {
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

type N3EntrypointPdrInfo struct {
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

type N3EntrypointQerInfo struct {
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

type N3EntrypointRouteStat struct {
	FibLookupIp4Cache     uint64
	FibLookupIp4Ok        uint64
	FibLookupIp4ErrorDrop uint64
	FibLookupIp4ErrorPass uint64
	FibLookupIp6Cache     uint64
	FibLookupIp6Ok        uint64
	FibLookupIp6ErrorDrop uint64
	FibLookupIp6ErrorPass uint64
}

type N3EntrypointUpfN3Statistic struct {
	UpfN3Counters struct {
		RxArp      uint64
		RxIcmp     uint64
		RxIcmp6    uint64
		RxIp4      uint64
		RxIp6      uint64
		RxTcp      uint64
		RxUdp      uint64
		RxOther    uint64
		RxGtpEcho  uint64
		RxGtpPdu   uint64
		RxGtpOther uint64
		RxGtpUnexp uint64
		UlBytes    uint64
	}
	UpfN3Counter struct {
		RxN3 uint64
		TxN3 uint64
	}
	XdpActions [8]uint64
}

// LoadN3Entrypoint returns the embedded CollectionSpec for N3Entrypoint.
func LoadN3Entrypoint() (*ebpf.CollectionSpec, error) {
	reader := bytes.NewReader(_N3EntrypointBytes)
	spec, err := ebpf.LoadCollectionSpecFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("can't load N3Entrypoint: %w", err)
	}

	return spec, err
}

// LoadN3EntrypointObjects loads N3Entrypoint and converts it into a struct.
//
// The following types are suitable as obj argument:
//
//	*N3EntrypointObjects
//	*N3EntrypointPrograms
//	*N3EntrypointMaps
//
// See ebpf.CollectionSpec.LoadAndAssign documentation for details.
func LoadN3EntrypointObjects(obj interface{}, opts *ebpf.CollectionOptions) error {
	spec, err := LoadN3Entrypoint()
	if err != nil {
		return err
	}

	return spec.LoadAndAssign(obj, opts)
}

// N3EntrypointSpecs contains maps and programs before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type N3EntrypointSpecs struct {
	N3EntrypointProgramSpecs
	N3EntrypointMapSpecs
	N3EntrypointVariableSpecs
}

// N3EntrypointProgramSpecs contains programs before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type N3EntrypointProgramSpecs struct {
	UpfN3EntrypointFunc *ebpf.ProgramSpec `ebpf:"upf_n3_entrypoint_func"`
}

// N3EntrypointMapSpecs contains maps before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type N3EntrypointMapSpecs struct {
	FarMap              *ebpf.MapSpec `ebpf:"far_map"`
	N3FarMap            *ebpf.MapSpec `ebpf:"n3_far_map"`
	N3PdrMapDownlinkIp4 *ebpf.MapSpec `ebpf:"n3_pdr_map_downlink_ip4"`
	N3PdrMapDownlinkIp6 *ebpf.MapSpec `ebpf:"n3_pdr_map_downlink_ip6"`
	N3PdrMapUplinkIp4   *ebpf.MapSpec `ebpf:"n3_pdr_map_uplink_ip4"`
	PdrMapDownlinkIp4   *ebpf.MapSpec `ebpf:"pdr_map_downlink_ip4"`
	PdrMapDownlinkIp6   *ebpf.MapSpec `ebpf:"pdr_map_downlink_ip6"`
	PdrMapUplinkIp4     *ebpf.MapSpec `ebpf:"pdr_map_uplink_ip4"`
	QerMap              *ebpf.MapSpec `ebpf:"qer_map"`
	UpfExtStat          *ebpf.MapSpec `ebpf:"upf_ext_stat"`
	UpfRouteStat        *ebpf.MapSpec `ebpf:"upf_route_stat"`
}

// N3EntrypointVariableSpecs contains global variables before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type N3EntrypointVariableSpecs struct {
}

// N3EntrypointObjects contains all objects after they have been loaded into the kernel.
//
// It can be passed to LoadN3EntrypointObjects or ebpf.CollectionSpec.LoadAndAssign.
type N3EntrypointObjects struct {
	N3EntrypointPrograms
	N3EntrypointMaps
	N3EntrypointVariables
}

func (o *N3EntrypointObjects) Close() error {
	return _N3EntrypointClose(
		&o.N3EntrypointPrograms,
		&o.N3EntrypointMaps,
	)
}

// N3EntrypointMaps contains all maps after they have been loaded into the kernel.
//
// It can be passed to LoadN3EntrypointObjects or ebpf.CollectionSpec.LoadAndAssign.
type N3EntrypointMaps struct {
	FarMap              *ebpf.Map `ebpf:"far_map"`
	N3FarMap            *ebpf.Map `ebpf:"n3_far_map"`
	N3PdrMapDownlinkIp4 *ebpf.Map `ebpf:"n3_pdr_map_downlink_ip4"`
	N3PdrMapDownlinkIp6 *ebpf.Map `ebpf:"n3_pdr_map_downlink_ip6"`
	N3PdrMapUplinkIp4   *ebpf.Map `ebpf:"n3_pdr_map_uplink_ip4"`
	PdrMapDownlinkIp4   *ebpf.Map `ebpf:"pdr_map_downlink_ip4"`
	PdrMapDownlinkIp6   *ebpf.Map `ebpf:"pdr_map_downlink_ip6"`
	PdrMapUplinkIp4     *ebpf.Map `ebpf:"pdr_map_uplink_ip4"`
	QerMap              *ebpf.Map `ebpf:"qer_map"`
	UpfExtStat          *ebpf.Map `ebpf:"upf_ext_stat"`
	UpfRouteStat        *ebpf.Map `ebpf:"upf_route_stat"`
}

func (m *N3EntrypointMaps) Close() error {
	return _N3EntrypointClose(
		m.FarMap,
		m.N3FarMap,
		m.N3PdrMapDownlinkIp4,
		m.N3PdrMapDownlinkIp6,
		m.N3PdrMapUplinkIp4,
		m.PdrMapDownlinkIp4,
		m.PdrMapDownlinkIp6,
		m.PdrMapUplinkIp4,
		m.QerMap,
		m.UpfExtStat,
		m.UpfRouteStat,
	)
}

// N3EntrypointVariables contains all global variables after they have been loaded into the kernel.
//
// It can be passed to LoadN3EntrypointObjects or ebpf.CollectionSpec.LoadAndAssign.
type N3EntrypointVariables struct {
}

// N3EntrypointPrograms contains all programs after they have been loaded into the kernel.
//
// It can be passed to LoadN3EntrypointObjects or ebpf.CollectionSpec.LoadAndAssign.
type N3EntrypointPrograms struct {
	UpfN3EntrypointFunc *ebpf.Program `ebpf:"upf_n3_entrypoint_func"`
}

func (p *N3EntrypointPrograms) Close() error {
	return _N3EntrypointClose(
		p.UpfN3EntrypointFunc,
	)
}

func _N3EntrypointClose(closers ...io.Closer) error {
	for _, closer := range closers {
		if err := closer.Close(); err != nil {
			return err
		}
	}
	return nil
}

// Do not access this directly.
//
//go:embed n3entrypoint_bpf.o
var _N3EntrypointBytes []byte
