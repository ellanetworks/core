package context

import (
	"context"
	"database/sql"
	"net"

	"github.com/omec-project/openapi/models"
	db "github.com/yeastengine/ella/internal/db/sql"
)

type DNS struct {
	IPv4Addr net.IP
	IPv6Addr net.IP
}

type SnssaiSmfDnnInfo struct {
	UeIPAllocator *IPAllocator
	DNS           DNS
	MTU           uint16
}

type SnssaiSmfInfo struct {
	DnnInfos map[string]*SnssaiSmfDnnInfo
	PlmnId   models.PlmnId
	Snssai   SNssai
}

func GetSnssaiInfos(queries *db.Queries) ([]SnssaiSmfInfo, error) {
	networkSlices, err := queries.ListNetworkSlices(context.Background())
	if err != nil {
		return nil, err
	}
	snssaiInfos := make([]SnssaiSmfInfo, 0, len(networkSlices))
	for _, networkSlice := range networkSlices {
		networkSliceID := sql.NullInt64{
			Int64: networkSlice.ID,
			Valid: true,
		}
		networkSliceDeviceGroups, err := queries.ListDeviceGroupsByNetworkSliceId(context.Background(), networkSliceID)
		if err != nil {
			return nil, err
		}
		dnnInfos := make(map[string]*SnssaiSmfDnnInfo)
		for _, networkSliceDeviceGroup := range networkSliceDeviceGroups {
			dnnInfo := SnssaiSmfDnnInfo{}
			dnnInfo.DNS.IPv4Addr = net.IP(networkSliceDeviceGroup.DnsPrimary).To4()
			if allocator, err := NewIPAllocator(networkSliceDeviceGroup.UeIpPool); err != nil {
				return nil, err
			} else {
				dnnInfo.UeIPAllocator = allocator
			}
			dnnInfo.MTU = uint16(networkSliceDeviceGroup.Mtu)
			dnnInfos[networkSliceDeviceGroup.Dnn] = &dnnInfo
		}

		snssaiInfo := SnssaiSmfInfo{
			DnnInfos: dnnInfos,
			PlmnId:   models.PlmnId{Mnc: networkSlice.Mnc, Mcc: networkSlice.Mcc},
			Snssai:   SNssai{Sst: int32(networkSlice.Sst), Sd: networkSlice.Sd},
		}
		snssaiInfos = append(snssaiInfos, snssaiInfo)
	}
	return snssaiInfos, nil
}

// we're replacing the function below
// func (c *SMFContext) insertSmfNssaiInfo(snssaiInfoConfig *factory.SnssaiInfoItem) error {
// 	logger.InitLog.Infof("Network Slices to be inserted [%v] ", factory.PrettyPrintNetworkSlices([]factory.SnssaiInfoItem{*snssaiInfoConfig}))

// 	if smfContext.SnssaiInfos == nil {
// 		c.SnssaiInfos = make([]SnssaiSmfInfo, 0)
// 	}

// 	// Check if prev slice with same sst+sd exist
// 	if slice := c.getSmfNssaiInfo(snssaiInfoConfig.SNssai.Sst, snssaiInfoConfig.SNssai.Sd); slice != nil {
// 		logger.InitLog.Errorf("network slice [%v] already exist, deleting", factory.PrettyPrintNetworkSlices([]factory.SnssaiInfoItem{*snssaiInfoConfig}))
// 		c.deleteSmfNssaiInfo(snssaiInfoConfig)
// 	}

// 	snssaiInfo := SnssaiSmfInfo{}
// 	snssaiInfo.Snssai = SNssai{
// 		Sst: snssaiInfoConfig.SNssai.Sst,
// 		Sd:  snssaiInfoConfig.SNssai.Sd,
// 	}

// 	// PLMN ID
// 	snssaiInfo.PlmnId = snssaiInfoConfig.PlmnId

// 	// DNN Info
// 	snssaiInfo.DnnInfos = make(map[string]*SnssaiSmfDnnInfo)

// 	for _, dnnInfoConfig := range snssaiInfoConfig.DnnInfos {
// 		dnnInfo := SnssaiSmfDnnInfo{}
// 		dnnInfo.DNS.IPv4Addr = net.ParseIP(dnnInfoConfig.DNS.IPv4Addr).To4()
// 		dnnInfo.DNS.IPv6Addr = net.ParseIP(dnnInfoConfig.DNS.IPv6Addr).To4()
// 		if allocator, err := NewIPAllocator(dnnInfoConfig.UESubnet); err != nil {
// 			logger.InitLog.Errorf("create ip allocator[%s] failed: %s", dnnInfoConfig.UESubnet, err)
// 			continue
// 		} else {
// 			dnnInfo.UeIPAllocator = allocator
// 		}

// 		if dnnInfoConfig.MTU != 0 {
// 			dnnInfo.MTU = dnnInfoConfig.MTU
// 		} else {
// 			// Adding default MTU value, if nothing is set in config file.
// 			dnnInfo.MTU = 1400
// 		}

// 		// block static IPs for this DNN if any
// 		if staticIpsCfg := c.GetDnnStaticIpInfo(dnnInfoConfig.Dnn); staticIpsCfg != nil {
// 			logger.InitLog.Infof("initialising slice [sst:%v, sd:%v], dnn [%s] with static IP info [%v]", snssaiInfo.Snssai.Sst, snssaiInfo.Snssai.Sd, dnnInfoConfig.Dnn, staticIpsCfg)
// 			dnnInfo.UeIPAllocator.ReserveStaticIps(&staticIpsCfg.ImsiIpInfo)
// 		}

// 		snssaiInfo.DnnInfos[dnnInfoConfig.Dnn] = &dnnInfo
// 	}
// 	c.SnssaiInfos = append(c.SnssaiInfos, snssaiInfo)

// 	return nil
// }
