package ebpf

import (
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

type UpfXdpActionStatistic struct {
	BpfObjects *BpfObjects
}

type UpfN3Counters struct {
	RxArp      uint64
	RxIcmp     uint64
	RxIcmp6    uint64
	RxIP4      uint64
	RxIP6      uint64
	RxTCP      uint64
	RxUDP      uint64
	RxOther    uint64
	RxGtpEcho  uint64
	RxGtpPdu   uint64
	RxGtpOther uint64
	RxGtpUnexp uint64
	UlBytes    uint64
}

type UpfStatistic struct {
	N3Counters UpfN3Counters
	XdpStats   [5]uint64
}

// Getters for the upf_xdp_statistic (xdp_action)

func (stat *UpfXdpActionStatistic) getUpfN3XdpStatisticField(field uint32) uint64 {
	var statistics []N3EntrypointUpfStatistic
	err := stat.BpfObjects.N3EntrypointObjects.UpfStat.Lookup(uint32(0), &statistics)
	if err != nil {
		logger.UpfLog.Info("Failed to fetch UPF N3 stats", zap.Error(err))
		return 0
	}

	var totalValue uint64 = 0
	for _, statistic := range statistics {
		totalValue += statistic.XdpActions[field]
	}

	return totalValue
}

func (stat *UpfXdpActionStatistic) getUpfN6XdpStatisticField(field uint32) uint64 {
	var statistics []N6EntrypointUpfStatistic
	err := stat.BpfObjects.N6EntrypointObjects.UpfStat.Lookup(uint32(0), &statistics)
	if err != nil {
		logger.UpfLog.Info("Failed to fetch UPF N6 stats", zap.Error(err))
		return 0
	}

	var totalValue uint64 = 0
	for _, statistic := range statistics {
		totalValue += statistic.XdpActions[field]
	}

	return totalValue
}

func (stat *UpfXdpActionStatistic) GetN3Aborted() uint64 {
	return stat.getUpfN3XdpStatisticField(uint32(0))
}

func (stat *UpfXdpActionStatistic) GetN3Drop() uint64 {
	return stat.getUpfN3XdpStatisticField(uint32(1))
}

func (stat *UpfXdpActionStatistic) GetN3Pass() uint64 {
	return stat.getUpfN3XdpStatisticField(uint32(2))
}

func (stat *UpfXdpActionStatistic) GetN3Tx() uint64 {
	return stat.getUpfN3XdpStatisticField(uint32(3))
}

func (stat *UpfXdpActionStatistic) GetN3Redirect() uint64 {
	return stat.getUpfN3XdpStatisticField(uint32(4))
}

func (stat *UpfXdpActionStatistic) GetN6Aborted() uint64 {
	return stat.getUpfN6XdpStatisticField(uint32(0))
}

func (stat *UpfXdpActionStatistic) GetN6Drop() uint64 {
	return stat.getUpfN6XdpStatisticField(uint32(1))
}

func (stat *UpfXdpActionStatistic) GetN6Tx() uint64 {
	return stat.getUpfN3XdpStatisticField(uint32(3))
}

func (stat *UpfXdpActionStatistic) GetN6Redirect() uint64 {
	return stat.getUpfN6XdpStatisticField(uint32(4))
}

func (stat *UpfXdpActionStatistic) GetN3UplinkThroughputStats() uint64 {
	var n3Statistics []N3EntrypointUpfStatistic
	err := stat.BpfObjects.N3EntrypointMaps.UpfStat.Lookup(uint32(0), &n3Statistics)
	if err != nil {
		logger.UpfLog.Info("Failed to fetch UPF N3 stats", zap.Error(err))
		return 0
	}

	var totalValue uint64 = 0
	for _, statistic := range n3Statistics {
		totalValue += statistic.UpfCounters.Bytes
	}

	return totalValue
}

func (stat *UpfXdpActionStatistic) GetN6DownlinkThroughputStats() uint64 {
	var n6Statistics []N6EntrypointUpfStatistic
	err := stat.BpfObjects.N6EntrypointMaps.UpfStat.Lookup(uint32(0), &n6Statistics)
	if err != nil {
		logger.UpfLog.Info("Failed to fetch UPF N6 stats", zap.Error(err))
		return 0
	}

	var totalValue uint64 = 0
	for _, statistic := range n6Statistics {
		totalValue += statistic.UpfCounters.Bytes
	}

	return totalValue
}
