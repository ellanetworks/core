// Copyright 2024 Ella Networks

package db

import (
	"context"
	"fmt"
	"net"
	"os"
)

func (db *Database) GetSize() (int64, error) {
	fileInfo, err := os.Stat(db.filepath)
	if err != nil {
		return 0, err
	}
	return fileInfo.Size(), nil
}

func (db *Database) GetIPAddressesTotal() (int, error) {
	dataNetworks, err := db.ListDataNetworks(context.Background())
	if err != nil {
		return 0, err
	}

	var total int
	for _, dn := range dataNetworks {
		ipPool := dn.IPPool
		_, ipNet, err := net.ParseCIDR(ipPool)
		if err != nil {
			return 0, fmt.Errorf("invalid IP pool format '%s': %v", ipPool, err)
		}
		total += countIPsInCIDR(ipNet)
	}
	return total, nil
}

func countIPsInCIDR(ipNet *net.IPNet) int {
	ones, bits := ipNet.Mask.Size()
	if bits-ones > 30 {
		return int(^uint32(0))
	}
	return 1 << (bits - ones)
}

func (db *Database) GetIPAddressesAllocated(ctx context.Context) (int, error) {
	subscribers, err := db.ListSubscribers(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to list subscribers: %v", err)
	}

	var allocatedCount int
	for _, subscriber := range subscribers {
		if subscriber.IPAddress != "" {
			allocatedCount++
		}
	}

	return allocatedCount, nil
}
