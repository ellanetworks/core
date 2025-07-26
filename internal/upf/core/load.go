package core

import (
	"os"
	"strconv"
	"strings"
	"time"
)

func GetCPUUsagePercent() (uint8, error) {
	stat1, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, err
	}
	time.Sleep(100 * time.Millisecond)
	stat2, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, err
	}

	var idle1, total1, idle2, total2 uint64

	parse := func(stat []byte) (idle, total uint64) {
		fields := strings.Fields(string(stat))
		for i, v := range fields[1:8] {
			val, _ := strconv.ParseUint(v, 10, 64)
			total += val
			if i == 3 { // idle
				idle = val
			}
		}
		return
	}

	idle1, total1 = parse(stat1)
	idle2, total2 = parse(stat2)

	deltaIdle := float64(idle2 - idle1)
	deltaTotal := float64(total2 - total1)

	if deltaTotal == 0 {
		return 0, nil
	}

	usage := (1 - (deltaIdle / deltaTotal)) * 100
	if usage < 0 {
		usage = 0
	} else if usage > 100 {
		usage = 100
	}

	return uint8(usage + 0.5), nil // rounded and clamped
}
