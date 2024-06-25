// Code generated by bpf2go; DO NOT EDIT.

package ebpf

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"

	"github.com/cilium/ebpf"
)

// LoadZeroEntrypoint returns the embedded CollectionSpec for ZeroEntrypoint.
func LoadZeroEntrypoint() (*ebpf.CollectionSpec, error) {
	reader := bytes.NewReader(_ZeroEntrypointBytes)
	spec, err := ebpf.LoadCollectionSpecFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("can't load ZeroEntrypoint: %w", err)
	}

	return spec, err
}

// LoadZeroEntrypointObjects loads ZeroEntrypoint and converts it into a struct.
//
// The following types are suitable as obj argument:
//
//	*ZeroEntrypointObjects
//	*ZeroEntrypointPrograms
//	*ZeroEntrypointMaps
//
// See ebpf.CollectionSpec.LoadAndAssign documentation for details.
func LoadZeroEntrypointObjects(obj interface{}, opts *ebpf.CollectionOptions) error {
	spec, err := LoadZeroEntrypoint()
	if err != nil {
		return err
	}

	return spec.LoadAndAssign(obj, opts)
}

// ZeroEntrypointSpecs contains maps and programs before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type ZeroEntrypointSpecs struct {
	ZeroEntrypointProgramSpecs
	ZeroEntrypointMapSpecs
}

// ZeroEntrypointSpecs contains programs before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type ZeroEntrypointProgramSpecs struct {
	UpfN3EntrypointFunc *ebpf.ProgramSpec `ebpf:"upf_n3_entrypoint_func"`
}

// ZeroEntrypointMapSpecs contains maps before they are loaded into the kernel.
//
// It can be passed ebpf.CollectionSpec.Assign.
type ZeroEntrypointMapSpecs struct {
}

// ZeroEntrypointObjects contains all objects after they have been loaded into the kernel.
//
// It can be passed to LoadZeroEntrypointObjects or ebpf.CollectionSpec.LoadAndAssign.
type ZeroEntrypointObjects struct {
	ZeroEntrypointPrograms
	ZeroEntrypointMaps
}

func (o *ZeroEntrypointObjects) Close() error {
	return _ZeroEntrypointClose(
		&o.ZeroEntrypointPrograms,
		&o.ZeroEntrypointMaps,
	)
}

// ZeroEntrypointMaps contains all maps after they have been loaded into the kernel.
//
// It can be passed to LoadZeroEntrypointObjects or ebpf.CollectionSpec.LoadAndAssign.
type ZeroEntrypointMaps struct {
}

func (m *ZeroEntrypointMaps) Close() error {
	return _ZeroEntrypointClose()
}

// ZeroEntrypointPrograms contains all programs after they have been loaded into the kernel.
//
// It can be passed to LoadZeroEntrypointObjects or ebpf.CollectionSpec.LoadAndAssign.
type ZeroEntrypointPrograms struct {
	UpfN3EntrypointFunc *ebpf.Program `ebpf:"upf_n3_entrypoint_func"`
}

func (p *ZeroEntrypointPrograms) Close() error {
	return _ZeroEntrypointClose(
		p.UpfN3EntrypointFunc,
	)
}

func _ZeroEntrypointClose(closers ...io.Closer) error {
	for _, closer := range closers {
		if err := closer.Close(); err != nil {
			return err
		}
	}
	return nil
}

// Do not access this directly.
//
//go:embed zeroentrypoint_bpf.o
var _ZeroEntrypointBytes []byte
