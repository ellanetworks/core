package ebpf

import "github.com/yeastengine/ella/internal/logger"

type UpfXdpActionStatistic struct {
	BpfObjects *BpfObjects
}

type UpfCounters struct {
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
}

type UpfStatistic struct {
	Counters UpfCounters
	XdpStats [5]uint64
}

func (current *UpfCounters) Add(nnew UpfCounters) {
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
}

// Getters for the upf_xdp_statistic (xdp_action)

func (stat *UpfXdpActionStatistic) getUpfXdpStatisticField(field uint32) uint64 {
	var statistics []IpEntrypointUpfStatistic
	err := stat.BpfObjects.UpfExtStat.Lookup(uint32(0), &statistics)
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

func (stat *UpfXdpActionStatistic) GetAborted() uint64 {
	return stat.getUpfXdpStatisticField(uint32(0))
}

func (stat *UpfXdpActionStatistic) GetDrop() uint64 {
	return stat.getUpfXdpStatisticField(uint32(1))
}

func (stat *UpfXdpActionStatistic) GetPass() uint64 {
	return stat.getUpfXdpStatisticField(uint32(2))
}

func (stat *UpfXdpActionStatistic) GetTx() uint64 {
	return stat.getUpfXdpStatisticField(uint32(3))
}

func (stat *UpfXdpActionStatistic) GetRedirect() uint64 {
	return stat.getUpfXdpStatisticField(uint32(4))
}

// Getters for the upf_ext_stat (upf_counters)
func (stat *UpfXdpActionStatistic) GetUpfExtStatField() UpfCounters {
	var statistics []IpEntrypointUpfStatistic
	var counters UpfCounters
	err := stat.BpfObjects.UpfExtStat.Lookup(uint32(0), &statistics)
	if err != nil {
		logger.UpfLog.Infof(err.Error())
		return counters
	}

	for _, statistic := range statistics {
		counters.Add(statistic.UpfCounters)
	}

	return counters
}
