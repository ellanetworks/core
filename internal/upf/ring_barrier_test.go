// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"os"
	"testing"

	cebpf "github.com/cilium/ebpf"
	"github.com/cilium/ebpf/asm"
	"github.com/cilium/ebpf/ringbuf"
)

func newRingbufWriter(t *testing.T) (*cebpf.Map, *cebpf.Program) {
	t.Helper()

	rb, err := cebpf.NewMap(&cebpf.MapSpec{
		Type:       cebpf.RingBuf,
		MaxEntries: uint32(os.Getpagesize()),
	})
	if err != nil {
		t.Fatalf("create ring buffer: %v", err)
	}

	t.Cleanup(func() { _ = rb.Close() })

	prog, err := cebpf.NewProgram(&cebpf.ProgramSpec{
		Type:    cebpf.SocketFilter,
		License: "GPL",
		Instructions: asm.Instructions{
			asm.StoreImm(asm.R10, -8, 0, asm.DWord),
			asm.LoadMapPtr(asm.R1, rb.FD()),
			asm.Mov.Reg(asm.R2, asm.R10),
			asm.Add.Imm(asm.R2, -8),
			asm.Mov.Imm(asm.R3, 8),
			asm.Mov.Imm(asm.R4, 0),
			asm.FnRingbufOutput.Call(),
			asm.Mov.Imm(asm.R0, 0),
			asm.Return(),
		},
	})
	if err != nil {
		t.Fatalf("load ring buffer writer: %v", err)
	}

	t.Cleanup(func() { _ = prog.Close() })

	return rb, prog
}

func TestRingCursorTracksKernelPositions(t *testing.T) {
	requireRoot(t)

	rb, prog := newRingbufWriter(t)

	cursor, err := openRingCursor(rb)
	if err != nil {
		t.Fatalf("open ring cursor: %v", err)
	}

	t.Cleanup(func() { _ = cursor.Close() })

	start := cursor.producer()

	if got := cursor.consumer(); got != start {
		t.Fatalf("consumer = %d, want %d on an empty ring", got, start)
	}

	if _, err := prog.Run(&cebpf.RunOptions{Data: make([]byte, 14)}); err != nil {
		t.Fatalf("run ring buffer writer: %v", err)
	}

	want := start + 16

	if got := cursor.producer(); got != want {
		t.Fatalf("producer = %d, want %d after one 8-byte record", got, want)
	}

	if !positionBefore(cursor.consumer(), cursor.producer()) {
		t.Fatal("unread record not reported behind the producer")
	}

	reader, err := ringbuf.NewReader(rb)
	if err != nil {
		t.Fatalf("open reader: %v", err)
	}

	t.Cleanup(func() { _ = reader.Close() })

	if _, err := reader.Read(); err != nil {
		t.Fatalf("read record: %v", err)
	}

	if got := cursor.consumer(); got != want {
		t.Errorf("consumer = %d, want %d after reading the record", got, want)
	}
}

func TestProgramBarrierWaits(t *testing.T) {
	requireRoot(t)

	barrier, err := newProgramBarrier()
	if err != nil {
		t.Fatalf("create program barrier: %v", err)
	}

	t.Cleanup(func() { _ = barrier.Close() })

	for range 3 {
		if err := barrier.wait(); err != nil {
			t.Fatalf("wait: %v", err)
		}
	}
}
