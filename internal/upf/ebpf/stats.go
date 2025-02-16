package ebpf

import "github.com/ellanetworks/core/internal/logger"

type UpfXdpActionStatistic struct {
	BpfObjects *BpfObjects
}

type UpfN3Counters struct {
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

type UpfStatistic struct {
	N3Counters UpfN3Counters
	XdpStats   [5]uint64
}

func (current *UpfN3Counters) Add(nnew UpfN3Counters) {
	current.RxArp += nnew.RxArp
	current.RxIcmp += nnew.RxIcmp
	current.RxIcmp6 += nnew.RxIcmp6
	current.RxIp4 += nnew.RxIp4
	current.RxIp6 += nnew.RxIp6
	current.RxTcp += nnew.RxTcp
	current.RxUdp += nnew.RxUdp
	current.RxOther += nnew.RxOther
	current.RxGtpEcho += nnew.RxGtpEcho
	current.RxGtpPdu += nnew.RxGtpPdu
	current.RxGtpOther += nnew.RxGtpOther
	current.UlBytes += nnew.UlBytes
}

// Getters for the upf_xdp_statistic (xdp_action)

func (stat *UpfXdpActionStatistic) getUpfN3XdpStatisticField(field uint32) uint64 {
	var statistics []N3EntrypointUpfN3Statistic
	err := stat.BpfObjects.N3EntrypointObjects.UpfN3Stat.Lookup(uint32(0), &statistics)
	if err != nil {
		logger.UpfLog.Infof(err.Error())
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

func (stat *UpfXdpActionStatistic) GetN3UplinkThroughputStats() uint64 {
	var n3Statistics []N3EntrypointUpfN3Statistic
	err := stat.BpfObjects.N3EntrypointMaps.UpfN3Stat.Lookup(uint32(0), &n3Statistics)
	if err != nil {
		logger.UpfLog.Infof("Failed to fetch UPF stats: %v", err)
		return 0
	}

	var totalValue uint64 = 0
	for _, statistic := range n3Statistics {
		totalValue += statistic.UpfN3Counters.UlBytes
	}

	return totalValue
}

func (stat *UpfXdpActionStatistic) GetN6DownlinkThroughputStats() uint64 {
	var n6Statistics []N6EntrypointUpfN6Statistic
	err := stat.BpfObjects.N6EntrypointMaps.UpfN6Stat.Lookup(uint32(0), &n6Statistics)
	if err != nil {
		logger.UpfLog.Infof("Failed to fetch UPF stats: %v", err)
		return 0
	}

	var totalValue uint64 = 0
	for _, statistic := range n6Statistics {
		totalValue += statistic.UpfN6Counters.DlBytes
	}

	return totalValue
}
