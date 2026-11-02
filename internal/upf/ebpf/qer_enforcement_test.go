// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"errors"
	"fmt"
	"net/netip"
	"runtime"
	"sync"
	"testing"

	"github.com/cilium/ebpf"
	"golang.org/x/sys/unix"
)

func loadQERProgram(t *testing.T, useTCX, localSwitch bool) *BpfObjects {
	t.Helper()

	obj := NewBpfObjects(false, false, localSwitch, 0, 1, 0, 0)
	obj.UseTCX = useTCX

	if err := obj.Load(); err != nil {
		t.Fatalf("load QER datapath: %+v", err)
	}

	t.Cleanup(func() { _ = obj.Close() })

	return obj
}

func qerRunOptions(obj *BpfObjects, frame []byte) *ebpf.RunOptions {
	opts := &ebpf.RunOptions{Data: frame, DataOut: make([]byte, len(frame)+256)}
	if obj.UseTCX {
		opts.Context = skbRunContext{IngressIfindex: 1, Ifindex: 1}
	}

	return opts
}

func qerPacketAllowed(obj *BpfObjects, action uint32) bool {
	if obj.UseTCX {
		return action == tcActOK || action == tcActRedirect
	}

	return action == ActionPass || action == ActionTx || action == ActionRedirect
}

func checkQERPacket(t *testing.T, obj *BpfObjects, frame []byte, wantDrop bool) {
	t.Helper()

	action, err := obj.UpfEntryFunc.Run(qerRunOptions(obj, frame))
	if err != nil {
		t.Fatalf("run QER packet: %v", err)
	}

	drop := uint32(ActionDrop)
	if obj.UseTCX {
		drop = tcActShot
	}

	if wantDrop && action != drop || !wantDrop && !qerPacketAllowed(obj, action) {
		logDropReasons(t, obj)
		t.Fatalf("QER packet action = %d, want drop = %t (TC = %t)", action, wantDrop, obj.UseTCX)
	}
}

func TestQERMissingMeterFailsClosed(t *testing.T) {
	requireProgTestRun(t)

	for _, build := range []string{"XDP", "TC"} {
		t.Run(build, func(t *testing.T) {
			for _, path := range []struct {
				name        string
				uplink      bool
				ipv6        bool
				localSwitch bool
			}{
				{name: "uplink IPv4", uplink: true},
				{name: "uplink IPv6", uplink: true, ipv6: true},
				{name: "downlink IPv4"},
				{name: "downlink IPv6", ipv6: true},
				{name: "local switch IPv4", localSwitch: true},
				{name: "local switch IPv6", localSwitch: true, ipv6: true},
			} {
				t.Run(path.name, func(t *testing.T) {
					const teid = 0x51455220

					obj := loadQERProgram(t, build == "TC", path.localSwitch)
					ue := netip.MustParseAddr("10.45.0.7")
					inner := ipv4Packet(canonicalUEv4.As4(), ue.As4(), 17, udpDatagram(4000, 53, nil))
					etherType := uint16(0x0800)

					if path.ipv6 {
						ue = netip.MustParseAddr("2001:db8:1::")
						inner = ipv6Packet(testUEv6Src, ue.As16(), 17, udpDatagram(4000, 53, nil))
						etherType = 0x86DD
					}

					ul := PdrInfo{
						IMSI:         "001010000000001",
						Far:          FarInfo{Action: 0x02},
						UEIPv4:       canonicalUEv4,
						UEIPv6Prefix: canonicalUEv6Prefix,
					}
					pdr := ipv4OuterDownlinkPDR(teid, testUPFN3IP, testGNBIP, 5)
					frame := ethFrame(etherType, inner)
					direction := Downlink

					if path.uplink {
						pdr = ul
					}

					if path.uplink || path.localSwitch {
						frame = uplinkGPDU(teid, inner)
						direction = Uplink
					}

					if path.localSwitch {
						if err := obj.PutPdrUplink(teid, ul); err != nil {
							t.Fatalf("install unlimited sender: %v", err)
						}
					}

					pdr.SEID, pdr.QerID = 0x100000001, 7
					publish := func() {
						t.Helper()

						var err error

						if path.uplink {
							err = obj.PutPdrUplink(teid, pdr)
						} else {
							err = obj.PutPdrDownlink(ue, pdr)
						}

						if err != nil {
							t.Fatalf("publish PDR: %v", err)
						}
					}

					publish()
					checkQERPacket(t, obj, frame, false)

					if path.uplink {
						pdr.Qer.MaxBitrateUL = 1
					} else {
						pdr.Qer.MaxBitrateDL = 1
					}

					publish()

					for range 2 {
						checkQERPacket(t, obj, frame, true)
					}

					key := N3N6EntrypointQerKey{Seid: pdr.SEID, QerId: pdr.QerID}

					var meter N3N6EntrypointQerWindow
					if err := obj.QerWindows.LookupWithFlags(key, &meter, ebpf.LookupLock); !errors.Is(err, ebpf.ErrKeyNotExist) {
						t.Fatalf("PDR publication or packet lazily created a meter: lookup = %v", err)
					}

					if got := DropCount(obj, direction, "qer_meter_missing"); got != 2 {
						t.Fatalf("qer_meter_missing drops = %d, want 2", got)
					}

					if got := DropCount(obj, direction, "qer_rate_limit"); got != 0 {
						t.Fatalf("missing meter reported as %d rate-limit drops", got)
					}

					provisionTestQER(t, obj, pdr)
					checkQERPacket(t, obj, frame, false)
					checkQERPacket(t, obj, frame, true)

					if got := DropCount(obj, direction, "qer_rate_limit"); got != 1 {
						t.Fatalf("provisioned meter: qer_rate_limit drops = %d, want 1", got)
					}

					if err := obj.DeleteQerMeter(pdr.SEID, pdr.QerID); err != nil {
						t.Fatalf("delete meter: %v", err)
					}

					checkQERPacket(t, obj, frame, true)

					if got := DropCount(obj, direction, "qer_meter_missing"); got != 3 {
						t.Fatalf("retired meter: qer_meter_missing drops = %d, want 3", got)
					}
				})
			}
		})
	}
}

func TestQERMeterBudgetSurvivesUpdates(t *testing.T) {
	requireProgTestRun(t)

	for _, build := range []string{"XDP", "TC"} {
		t.Run(build, func(t *testing.T) {
			for _, source := range []string{"PDR", "classifier"} {
				t.Run(source, func(t *testing.T) {
					const teid = 0x51455221

					obj := loadQERProgram(t, build == "TC", false)
					pdr := ipv4OuterDownlinkPDR(teid, testUPFN3IP, testGNBIP, 5)
					pdr.SEID, pdr.QerID, pdr.PdrID = 0x100000002, 8, 2
					pdr.Qer.MaxBitrateUL, pdr.Qer.MaxBitrateDL = 1, 1
					pdr.Qer.AveragingWindowMs = 5

					if source == "classifier" {
						pdr.Flags = PdrFlagSDF
					}

					provisionTestQER(t, obj, pdr)

					publish := func() {
						t.Helper()

						ul := pdr
						ul.Flags = 0
						ul.Far = FarInfo{Action: 0x02}
						ul.UEIPv4, ul.UEIPv6Prefix = canonicalUEv4, canonicalUEv6Prefix

						if err := obj.PutPdrUplink(teid, ul); err != nil {
							t.Fatalf("publish uplink: %v", err)
						}

						dl := pdr

						if source == "classifier" {
							dl.QerID++
							dl.Qer = QerInfo{Qfi: pdr.Qer.Qfi}
						}

						for _, ue := range []netip.Addr{netip.AddrFrom4(classifierUE), canonicalUEv6Prefix} {
							if err := obj.PutPdrDownlink(ue, dl); err != nil {
								t.Fatalf("publish downlink: %v", err)
							}
						}

						if source == "classifier" {
							c := Classifier{
								Rules:   []ClassifierRule{{Direction: ClassifierDownlink, Protocol: protoUDP}},
								Targets: []ClassifierTarget{{PdrID: pdr.PdrID, QerID: pdr.QerID, Far: pdr.Far, Qer: pdr.Qer}},
							}
							if err := obj.PutClassifier(pdr.SEID, c); err != nil {
								t.Fatalf("publish classifier: %v", err)
							}
						}
					}
					publish()

					ulFrame := uplinkGPDU(teid, innerIPv4UDP([4]byte{8, 8, 8, 8}, 53))
					dl4 := downlinkUDP(remotePort, localPort)
					dl6 := ethFrame(0x86DD, ipv6Packet(testUEv6Src, testUEv6, 17, udpDatagram(53, 4000, nil)))

					checkQERPacket(t, obj, ulFrame, false)
					checkQERPacket(t, obj, dl4, false)
					checkQERPacket(t, obj, dl6, true)

					key := N3N6EntrypointQerKey{Seid: pdr.SEID, QerId: pdr.QerID}

					var charged N3N6EntrypointQerWindow
					if err := obj.QerWindows.LookupWithFlags(key, &charged, ebpf.LookupLock); err != nil {
						t.Fatalf("read charged meter: %v", err)
					}

					if charged.UlStart == 0 || charged.DlStart == 0 {
						t.Fatalf("directions did not independently charge their shared meter: %+v", charged)
					}

					for _, update := range []string{"reput", "rate and window", "gate closed", "unlimited", "limited again", "reload"} {
						t.Run(update, func(t *testing.T) {
							reason := "qer_rate_limit"

							switch update {
							case "reput":
								provisionTestQER(t, obj, pdr)
							case "rate and window":
								pdr.Qer.MaxBitrateUL, pdr.Qer.MaxBitrateDL = 2, 2
								pdr.Qer.AveragingWindowMs = 200
								pdr.Far.TransportLevelMarking = 0xB800
							case "gate closed":
								pdr.Qer.GateStatusUL, pdr.Qer.GateStatusDL = 1, 1
								reason = "qer_gate_closed"
							case "unlimited":
								pdr.Qer.GateStatusUL, pdr.Qer.GateStatusDL = 0, 0
								pdr.Qer.MaxBitrateUL, pdr.Qer.MaxBitrateDL = 0, 0
								reason = ""
							case "limited again":
								pdr.Qer.MaxBitrateUL, pdr.Qer.MaxBitrateDL = 2, 2
							case "reload":
								if err := obj.LoadWithMapReplacements(); err != nil {
									t.Fatalf("reload datapath: %v", err)
								}
							}

							publish()

							for _, packet := range []struct {
								frame []byte
								dir   Direction
							}{{ulFrame, Uplink}, {dl4, Downlink}, {dl6, Downlink}} {
								before := DropCount(obj, packet.dir, reason)
								checkQERPacket(t, obj, packet.frame, reason != "")

								if reason != "" && DropCount(obj, packet.dir, reason) != before+1 {
									t.Fatalf("packet was not dropped for %s", reason)
								}
							}

							var after N3N6EntrypointQerWindow
							if err := obj.QerWindows.LookupWithFlags(key, &after, ebpf.LookupLock); err != nil {
								t.Fatalf("read preserved meter: %v", err)
							}

							if after.UlStart != charged.UlStart || after.DlStart != charged.DlStart {
								t.Fatalf("update reset or spent budget: before %+v, after %+v", charged, after)
							}
						})
					}
				})
			}
		})
	}
}

func TestQERConcurrentCPUsShareOneBudget(t *testing.T) {
	requireProgTestRun(t)

	var affinity unix.CPUSet
	if err := unix.SchedGetaffinity(0, &affinity); err != nil {
		t.Fatalf("get CPU affinity: %v", err)
	}

	possible, err := ebpf.PossibleCPU()
	if err != nil {
		t.Fatalf("possible CPUs: %v", err)
	}

	var cpus []int

	for cpu := 0; cpu < possible && len(cpus) < 8; cpu++ {
		if affinity.IsSet(cpu) {
			cpus = append(cpus, cpu)
		}
	}

	if len(cpus) < 2 {
		t.Skip("shared-meter concurrency requires two allowed CPUs")
	}

	for _, build := range []string{"XDP", "TC"} {
		t.Run(build, func(t *testing.T) {
			obj := loadQERProgram(t, build == "TC", false)
			pdr := ipv4OuterDownlinkPDR(0x51455222, testUPFN3IP, testGNBIP, 5)
			pdr.SEID, pdr.QerID = 0x100000003, 9
			pdr.Qer.MaxBitrateDL = 1

			for _, ue := range []netip.Addr{netip.AddrFrom4(classifierUE), canonicalUEv6Prefix} {
				if err := obj.PutPdrDownlink(ue, pdr); err != nil {
					t.Fatalf("publish shared-QER PDR: %v", err)
				}
			}

			frames := [][]byte{
				downlinkUDP(remotePort, localPort),
				ethFrame(0x86DD, ipv6Packet(testUEv6Src, testUEv6, 17, udpDatagram(53, 4000, nil))),
			}

			probeFrame := ethFrame(0x0800, innerIPv4UDP([4]byte{8, 8, 8, 8}, 53))
			checkQERPacket(t, obj, probeFrame, false)

			probe := qerRunOptions(obj, probeFrame)
			probe.Flags, probe.CPU = unix.BPF_F_TEST_RUN_ON_CPU, uint32(cpus[0])
			_, err := obj.UpfEntryFunc.Run(probe)
			explicitCPU := err == nil

			if err != nil && !errors.Is(err, unix.EINVAL) && !errors.Is(err, ebpf.ErrNotSupported) {
				t.Fatalf("probe test-run CPU selection: %v", err)
			}

			t.Logf("CPUs %v; explicit test-run CPU selection = %t (workers always pinned)", cpus, explicitCPU)

			const rounds = 128

			for round := range rounds {
				provisionTestQER(t, obj, pdr)

				results := make([]struct {
					action uint32
					err    error
				}, len(cpus))

				var ready, done sync.WaitGroup

				start := make(chan struct{})

				ready.Add(len(cpus))

				for i, cpu := range cpus {
					done.Go(func() {
						runtime.LockOSThread()

						defer runtime.UnlockOSThread()

						var saved, pinned unix.CPUSet
						if err := unix.SchedGetaffinity(0, &saved); err != nil {
							results[i].err = fmt.Errorf("get worker affinity: %w", err)

							ready.Done()

							return
						}

						pinned.Set(cpu)

						if err := unix.SchedSetaffinity(0, &pinned); err != nil {
							results[i].err = fmt.Errorf("pin worker: %w", err)

							ready.Done()

							return
						}

						defer func() {
							if err := unix.SchedSetaffinity(0, &saved); err != nil {
								results[i].err = fmt.Errorf("restore worker affinity: %w", err)
							}
						}()

						opts := qerRunOptions(obj, frames[i%len(frames)])
						if explicitCPU {
							opts.Flags, opts.CPU = unix.BPF_F_TEST_RUN_ON_CPU, uint32(cpu)
						}

						ready.Done()
						<-start

						results[i].action, results[i].err = obj.UpfEntryFunc.Run(opts)
					})
				}

				ready.Wait()
				close(start)
				done.Wait()

				admitted := 0

				for i, result := range results {
					if result.err != nil {
						t.Fatalf("round %d CPU %d: %v", round, cpus[i], result.err)
					}

					if qerPacketAllowed(obj, result.action) {
						admitted++
					}
				}

				if admitted != 1 {
					t.Fatalf("round %d: admitted %d of %d simultaneous IPv4/IPv6 packets sharing one QER, want 1", round, admitted, len(cpus))
				}

				if got, want := DropCount(obj, Downlink, "qer_rate_limit"), uint64((round+1)*(len(cpus)-1)); got != want {
					t.Fatalf("round %d: rate-limit drops = %d, want %d", round, got, want)
				}

				if err := obj.DeleteQerMeter(pdr.SEID, pdr.QerID); err != nil {
					t.Fatalf("retire round's meter: %v", err)
				}
			}
		})
	}
}
