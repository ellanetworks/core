package db

import (
	"context"
	db "database/sql"
	"fmt"
	"net"

	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/ella/internal/db/sql"
	"github.com/yeastengine/ella/internal/smf/factory"
	"github.com/yeastengine/ella/internal/smf/logger"
)

type SnssaiSmfInfo struct {
	DnnInfos map[string]*SnssaiSmfDnnInfo
	PlmnId   models.PlmnId
	Snssai   models.Snssai
}

type DNS struct {
	IPv4Addr net.IP
	IPv6Addr net.IP
}

func GetSnssaiInfos() ([]SnssaiSmfInfo, error) {
	queries := factory.SmfConfig.Configuration.DBQueries
	networkSlices, err := queries.ListNetworkSlices(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to list network slices: %v", err)
	}

	snssaiInfos := make([]SnssaiSmfInfo, 0, len(networkSlices))

	for _, networkSlice := range networkSlices {
		plmnID := models.PlmnId{
			Mcc: networkSlice.Mcc,
			Mnc: networkSlice.Mnc,
		}
		snssaiInfo := SnssaiSmfInfo{
			PlmnId: plmnID,
			Snssai: models.Snssai{
				Sst: int32(networkSlice.Sst),
				Sd:  networkSlice.Sd,
			},
			DnnInfos: make(map[string]*SnssaiSmfDnnInfo),
		}

		deviceGroups, err := queries.ListDeviceGroupsByNetworkSliceId(context.Background(), networkSlice.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to list device groups for network slice %d: %v", networkSlice.ID, err)
		}

		for _, deviceGroup := range deviceGroups {
			dnn := deviceGroup.Dnn
			snssaiInfo.DnnInfos[dnn] = &SnssaiSmfDnnInfo{
				DNS: DNS{
					IPv4Addr: net.ParseIP(deviceGroup.DnsPrimary).To4(),
				},
				MTU: uint16(deviceGroup.Mtu),
			}
		}
		snssaiInfos = append(snssaiInfos, snssaiInfo)
	}

	return snssaiInfos, nil
}

func RetrieveDnnInformation(Snssai models.Snssai, dnn string) *SnssaiSmfDnnInfo {
	snssaiInfo, err := GetSnssaiInfos()
	if err != nil {
		logger.CtxLog.Errorf("get snssai info failed: %v", err)
		return nil
	}
	for _, snssaiInfo := range snssaiInfo {
		if snssaiInfo.Snssai.Sst == Snssai.Sst && snssaiInfo.Snssai.Sd == Snssai.Sd {
			return snssaiInfo.DnnInfos[dnn]
		}
	}
	return nil
}

func GetDnnInfo() (*SnssaiSmfDnnInfo, error) {
	return nil, nil
}

type SnssaiSmfDnnInfo struct {
	DNS DNS
	MTU uint16
}

func AllocateIP(imsi string) (net.IP, error) {
	queries := factory.SmfConfig.Configuration.DBQueries
	subscriber, err := queries.GetSubscriberByImsi(context.Background(), imsi)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscriber by IMSI %s: %v", imsi, err)
	}
	deviceGroup, err := queries.GetDeviceGroup(context.Background(), subscriber.DeviceGroupID.Int64)
	if err != nil {
		return nil, fmt.Errorf("failed to get device group for subscriber IMSI %s: %v", imsi, err)
	}
	cidr, err := queries.GetIPPoolCIDR(context.Background(), deviceGroup.UeIpPoolID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve CIDR for pool ID %d: %v", deviceGroup.UeIpPoolID, err)
	}

	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR format: %v", err)
	}

	for ip := ipNet.IP.Mask(ipNet.Mask); ipNet.Contains(ip); incrementIP(ip) {
		findAvailableIPParams := sql.FindAvailableIPParams{
			IpAddress: ip.String(),
			PoolID:    deviceGroup.UeIpPoolID,
		}
		ip, err := queries.FindAvailableIP(context.Background(), findAvailableIPParams)

		if err == db.ErrNoRows {
			allocateIPParams := sql.AllocateIPParams{
				Imsi:      imsi,
				IpAddress: ip,
				PoolID:    deviceGroup.UeIpPoolID,
			}

			err = queries.AllocateIP(context.Background(), allocateIPParams)
			if err != nil {
				return nil, fmt.Errorf("failed to allocate IP: %v", err)
			}
			return net.ParseIP(ip), nil
		}
	}

	return nil, fmt.Errorf("no available IP addresses in pool %d", deviceGroup.UeIpPoolID)
}

// incrementIP increments an IP address by 1.
func incrementIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func ReleaseIP(imsi string) error {
	queries := factory.SmfConfig.Configuration.DBQueries

	// Check if the IMSI has an allocated IP address
	_, err := queries.GetAllocatedIPByIMSI(context.Background(), imsi)
	if err != nil {
		if err == db.ErrNoRows {
			// No IP allocated for this IMSI, nothing to release
			return fmt.Errorf("no IP allocated for IMSI %s", imsi)
		}
		return fmt.Errorf("failed to retrieve allocated IP for IMSI %s: %v", imsi, err)
	}

	err = queries.ReleaseIP(context.Background(), imsi)
	if err != nil {
		return fmt.Errorf("failed to release IP for IMSI %s: %v", imsi, err)
	}

	return nil
}
