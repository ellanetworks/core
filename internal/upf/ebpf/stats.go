package ebpf

import (
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

const (
	XDP_ABORTED  = 0
	XDP_DROP     = 1
	XDP_PASS     = 2
	XDP_TX       = 3
	XDP_REDIRECT = 4
)

func getUpfN3XdpStatisticField(bpfObjects *BpfObjects, field uint32) uint64 {
	var statistics []N3N6EntrypointUpfStatistic

	err := bpfObjects.UplinkStatistics.Lookup(uint32(0), &statistics)
	if err != nil {
		logger.UpfLog.Info("Failed to fetch UPF N3 stats", zap.Error(err))
		return 0
	}

	var totalValue uint64
	for _, statistic := range statistics {
		totalValue += statistic.XdpActions[field]
	}

	return totalValue
}

func getUpfN6XdpStatisticField(bpfObjects *BpfObjects, field uint32) uint64 {
	var statistics []N3N6EntrypointUpfStatistic

	err := bpfObjects.DownlinkStatistics.Lookup(uint32(0), &statistics)
	if err != nil {
		logger.UpfLog.Info("Failed to fetch UPF N6 stats", zap.Error(err))
		return 0
	}

	var totalValue uint64
	for _, statistic := range statistics {
		totalValue += statistic.XdpActions[field]
	}

	return totalValue
}

func GetN3Aborted(bpfObjects *BpfObjects) uint64 {
	return getUpfN3XdpStatisticField(bpfObjects, XDP_ABORTED)
}

func GetN3Drop(bpfObjects *BpfObjects) uint64 {
	return getUpfN3XdpStatisticField(bpfObjects, XDP_DROP)
}

func GetN3Pass(bpfObjects *BpfObjects) uint64 {
	return getUpfN3XdpStatisticField(bpfObjects, XDP_PASS)
}

func GetN3Tx(bpfObjects *BpfObjects) uint64 {
	return getUpfN3XdpStatisticField(bpfObjects, XDP_TX)
}

func GetN3Redirect(bpfObjects *BpfObjects) uint64 {
	return getUpfN3XdpStatisticField(bpfObjects, XDP_REDIRECT)
}

func GetN6Aborted(bpfObjects *BpfObjects) uint64 {
	return getUpfN6XdpStatisticField(bpfObjects, XDP_ABORTED)
}

func GetN6Drop(bpfObjects *BpfObjects) uint64 {
	return getUpfN6XdpStatisticField(bpfObjects, XDP_DROP)
}

func GetN6Pass(bpfObjects *BpfObjects) uint64 {
	return getUpfN6XdpStatisticField(bpfObjects, XDP_PASS)
}

func GetN6Tx(bpfObjects *BpfObjects) uint64 {
	return getUpfN6XdpStatisticField(bpfObjects, XDP_TX)
}

func GetN6Redirect(bpfObjects *BpfObjects) uint64 {
	return getUpfN6XdpStatisticField(bpfObjects, XDP_REDIRECT)
}

func GetN3UplinkThroughputStats(bpfObjects *BpfObjects) uint64 {
	var n3Statistics []N3N6EntrypointUpfStatistic

	err := bpfObjects.UplinkStatistics.Lookup(uint32(0), &n3Statistics)
	if err != nil {
		logger.UpfLog.Info("Failed to fetch UPF N3 stats", zap.Error(err))
		return 0
	}

	var totalValue uint64
	for _, statistic := range n3Statistics {
		totalValue += statistic.ByteCounter.Bytes
	}

	return totalValue
}

func GetN6DownlinkThroughputStats(bpfObjects *BpfObjects) uint64 {
	var n6Statistics []N3N6EntrypointUpfStatistic

	err := bpfObjects.DownlinkStatistics.Lookup(uint32(0), &n6Statistics)
	if err != nil {
		logger.UpfLog.Info("Failed to fetch UPF N6 stats", zap.Error(err))
		return 0
	}

	var totalValue uint64
	for _, statistic := range n6Statistics {
		totalValue += statistic.ByteCounter.Bytes
	}

	return totalValue
}
