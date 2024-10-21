package context

// func SetupSMFContext(config *factory.Config) error {
// 	return nil
// }

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

// func (c *SMFContext) updateSmfNssaiInfo(modSliceInfo *factory.SnssaiInfoItem) error {
// 	// identify slices to be updated
// 	logger.InitLog.Infof("Network Slices to be modified [%v] ", factory.PrettyPrintNetworkSlices([]factory.SnssaiInfoItem{*modSliceInfo}))
// 	if err := c.deleteSmfNssaiInfo(modSliceInfo); err != nil {
// 		logger.InitLog.Errorf("network slice delete error %v", err)
// 		return err
// 	}

// 	if err := c.insertSmfNssaiInfo(modSliceInfo); err != nil {
// 		logger.InitLog.Errorf("network slice insert error %v", err)
// 		return err
// 	}
// 	return nil
// }

// func (c *SMFContext) deleteSmfNssaiInfo(delSliceInfo *factory.SnssaiInfoItem) error {
// 	logger.InitLog.Infof("Network Slices to be deleted [%v] ", factory.PrettyPrintNetworkSlices([]factory.SnssaiInfoItem{*delSliceInfo}))
// 	snssaiInfos := nf_helpers.GetSnssaiInfos()
// 	for index, slice := range snssaiInfos {
// 		if slice.Snssai.Sd == delSliceInfo.SNssai.Sd && slice.Snssai.Sst == delSliceInfo.SNssai.Sst {
// 			// Remove the desired slice
// 			logger.InitLog.Infof("network slices deleted [%v] ", factory.PrettyPrintNetworkSlices([]factory.SnssaiInfoItem{*delSliceInfo}))
// 			c.SnssaiInfos = append(c.SnssaiInfos[:index], c.SnssaiInfos[index+1:]...)
// 			return nil
// 		}
// 	}

// 	err := fmt.Errorf("network slice [%v] to be deleted not found", factory.PrettyPrintNetworkSlices([]factory.SnssaiInfoItem{*delSliceInfo}))
// 	logger.InitLog.Errorf("%v", err.Error())
// 	return err
// }

// func (c *SMFContext) getSmfNssaiInfo(sst int32, sd string) *SnssaiSmfInfo {
// 	for _, slice := range c.SnssaiInfos {
// 		if slice.Snssai.Sd == sd && slice.Snssai.Sst == sst {
// 			return &slice
// 		}
// 	}
// 	return nil
// }
