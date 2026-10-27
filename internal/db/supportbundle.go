// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"context"
	"io"
	"maps"
	"slices"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/supportbundle"
	"go.uber.org/zap"
)

func (db *Database) ExportSupportData(ctx context.Context, options supportbundle.ExportOptions) (supportbundle.SanitizedData, error) {
	ctx, span := tracer.Start(ctx, "db/export_support_data")
	defer span.End()

	start := time.Now()
	capturedAt := time.Now().UTC().Format(time.RFC3339)

	out := supportbundle.SanitizedData{}
	sections := map[string]supportbundle.SectionStatus{}
	failure := func(section string, err error) {
		status := sections[section]
		status.Status, status.Reason = "failed", "collection failed"
		sections[section] = status.WithCount(status.Count)
		logger.DBLog.Warn("support section collection failed", zap.String("section", section), zap.Error(err))
	}
	out["bundle_metadata"] = map[string]any{
		"version":     "2.0",
		"captured_at": capturedAt,
		"redaction":   map[string]any{"policy": "secrets-only"},
	}
	db.collectSupportConfiguration(ctx, out, sections, failure)
	db.collectSupportRegistrations(ctx, out, sections, failure)

	if options.IncludeSubscriberUsage {
		db.collectSupportUsage(ctx, out, sections, failure)
	} else {
		for _, section := range []string{"usage_per_day", "usage_per_subscriber"} {
			sections[section] = supportbundle.SectionStatus{Status: "omitted", Reason: "permission restricted"}
		}
	}

	db.collectSupportEvents(ctx, out, options, sections, failure)

	out["bundle_metadata"].(map[string]any)["sections"] = sections

	DBQueryDuration.WithLabelValues("support_bundle", "export").Observe(time.Since(start).Seconds())

	return out, nil
}

func (db *Database) collectSupportConfiguration(ctx context.Context, out supportbundle.SanitizedData, sections map[string]supportbundle.SectionStatus, failure func(string, error)) {
	perPage := 1000

	op, err := db.GetOperator(ctx)
	if err != nil {
		failure("operator", err)
	} else {
		op.OperatorCode = "*"

		supportedTACs, _ := op.GetSupportedTacs()
		ciphering, _ := op.GetCiphering()
		integrity, _ := op.GetIntegrity()
		operatorMap := map[string]any{
			"ID":            op.ID,
			"Mcc":           op.Mcc,
			"Mnc":           op.Mnc,
			"OperatorCode":  op.OperatorCode,
			"SupportedTACs": supportedTACs,
			"SpnFullName":   op.SpnFullName,
			"SpnShortName":  op.SpnShortName,
			"AmfRegionID":   op.AmfRegionID,
			"AmfSetID":      op.AmfSetID,
			"ClusterID":     op.ClusterID,
			"Ciphering":     ciphering,
			"Integrity":     integrity,
		}
		out["operator"] = operatorMap
		sections["operator"] = sections["operator"].WithCount(1)
	}

	hnKeys, err := db.ListHomeNetworkKeys(ctx)
	if err != nil {
		failure("home_network_keys", err)
	} else {
		redacted := make([]map[string]any, 0, len(hnKeys))
		for _, k := range hnKeys {
			redacted = append(redacted, map[string]any{
				"ID":            k.ID,
				"KeyIdentifier": k.KeyIdentifier,
				"Scheme":        k.Scheme,
				"PrivateKey":    "*",
			})
		}

		out["home_network_keys"] = redacted
		sections["home_network_keys"] = sections["home_network_keys"].WithCount(len(redacted))
	}

	policiesList := []Policy{}

	for page := 1; ; page++ {
		pols, total, err := db.ListPoliciesPage(ctx, page, perPage)
		if err != nil {
			failure("policies", err)
			break
		}

		policiesList = append(policiesList, pols...)

		if len(pols) == 0 || page*perPage >= total {
			break
		}
	}

	out["policies"] = policiesList
	sections["policies"] = sections["policies"].WithCount(len(policiesList))

	dataNetworksList := []DataNetwork{}

	for page := 1; ; page++ {
		dns, total, err := db.ListDataNetworksPage(ctx, page, perPage)
		if err != nil {
			failure("networking", err)
			break
		}

		dataNetworksList = append(dataNetworksList, dns...)

		if len(dns) == 0 || page*perPage >= total {
			break
		}
	}

	out["networking"] = dataNetworksList
	sections["networking"] = sections["networking"].WithCount(len(dataNetworksList))

	subscribersList := []any{}

	for page := 1; ; page++ {
		subs, total, err := db.ListSubscribersPage(ctx, nil, page, perPage)
		if err != nil {
			failure("subscribers", err)
			break
		}

		for _, s := range subs {
			entry := map[string]any{
				"imsi":       s.Imsi,
				"msisdn":     s.Msisdn,
				"profile_id": s.ProfileID,
			}
			subscribersList = append(subscribersList, entry)
		}

		if len(subs) == 0 || page*perPage >= total {
			break
		}
	}

	out["subscribers"] = subscribersList
	sections["subscribers"] = sections["subscribers"].WithCount(len(subscribersList))

	allLeases, err := db.listAllLeases(ctx)
	if err != nil {
		failure("ip_leases", err)
	} else {
		leaseEntries := make([]any, 0, len(allLeases))
		for _, l := range allLeases {
			entry := map[string]any{
				"poolID":  l.PoolID,
				"address": l.Address().String(),
				"imsi":    l.IMSI,
				"type":    l.Type,
			}
			if l.SessionID != nil {
				entry["sessionID"] = *l.SessionID
			}

			leaseEntries = append(leaseEntries, entry)
		}

		out["ip_leases"] = leaseEntries
		sections["ip_leases"] = sections["ip_leases"].WithCount(len(leaseEntries))
	}

	profilesList := []Profile{}

	for page := 1; ; page++ {
		profs, total, err := db.ListProfilesPage(ctx, page, perPage)
		if err != nil {
			failure("profiles", err)
			break
		}

		profilesList = append(profilesList, profs...)

		if len(profs) == 0 || page*perPage >= total {
			break
		}
	}

	out["profiles"] = profilesList
	sections["profiles"] = sections["profiles"].WithCount(len(profilesList))

	db.collectSupportIMSServices(ctx, out, sections, failure, profilesList)

	if slicesList, err := db.ListAllNetworkSlices(ctx); err != nil {
		failure("network_slices", err)
	} else {
		out["network_slices"] = slicesList
		sections["network_slices"] = sections["network_slices"].WithCount(len(slicesList))
	}

	rulesList := []*NetworkRule{}

	for _, p := range policiesList {
		rules, err := db.ListRulesForPolicy(ctx, p.ID)
		if err != nil {
			failure("network_rules", err)
			continue
		}

		rulesList = append(rulesList, rules...)
	}

	out["network_rules"] = rulesList

	sections["network_rules"] = sections["network_rules"].WithCount(len(rulesList))
	if sections["policies"].Status != "complete" {
		status := sections["network_rules"]
		status.Status, status.Reason = "failed", "policy collection incomplete"
		sections["network_rules"] = status.WithCount(len(rulesList))
	}

	if positions, err := db.ListCellPositions(ctx); err != nil {
		failure("cell_positions", err)
	} else {
		out["cell_positions"] = positions
		sections["cell_positions"] = sections["cell_positions"].WithCount(len(positions))
	}

	if members, err := db.ListClusterMembers(ctx); err != nil {
		failure("cluster_members", err)
	} else {
		out["cluster_members"] = members
		sections["cluster_members"] = sections["cluster_members"].WithCount(len(members))
	}

	if routes, err := db.ListAllRoutes(ctx); err != nil {
		failure("routes", err)
	} else {
		out["routes"] = routes
		sections["routes"] = sections["routes"].WithCount(len(routes))
	}

	if natEnabled, err := db.IsNATEnabled(ctx); err != nil {
		failure("nat_settings", err)
	} else {
		out["nat_settings"] = map[string]any{"enabled": natEnabled}
		sections["nat_settings"] = sections["nat_settings"].WithCount(1)
	}

	if flowAccountingEnabled, err := db.IsFlowAccountingEnabled(ctx); err != nil {
		failure("flow_accounting_settings", err)
	} else {
		out["flow_accounting_settings"] = map[string]any{"enabled": flowAccountingEnabled}
		sections["flow_accounting_settings"] = sections["flow_accounting_settings"].WithCount(1)
	}

	if localSwitchEnabled, err := db.IsLocalSwitchEnabled(ctx); err != nil {
		failure("local_switch_settings", err)
	} else {
		out["local_switch_settings"] = map[string]any{"enabled": localSwitchEnabled}
		sections["local_switch_settings"] = sections["local_switch_settings"].WithCount(1)
	}

	if n3Settings, err := db.GetN3Settings(ctx); err != nil {
		failure("n3_settings", err)
	} else {
		out["n3_settings"] = map[string]any{"external_address": n3Settings.ExternalAddress}
		sections["n3_settings"] = sections["n3_settings"].WithCount(1)
	}

	if smsSettings, err := db.GetSMSSettings(ctx); err != nil {
		failure("sms_settings", err)
	} else {
		out["sms_settings"] = map[string]any{"sms_number": smsSettings.SMSNumber}
		sections["sms_settings"] = sections["sms_settings"].WithCount(1)
	}

	if smscPeers, err := db.ListSMSCPeers(ctx); err != nil {
		failure("diameter_peers", err)
	} else {
		out["diameter_peers"] = smscPeers
		sections["diameter_peers"] = sections["diameter_peers"].WithCount(len(smscPeers))
	}

	if pcscfAddrs, err := db.ListPCSCFAddresses(ctx); err != nil {
		failure("pcscf_addresses", err)
	} else {
		addrs := make([]string, 0, len(pcscfAddrs))
		for _, a := range pcscfAddrs {
			addrs = append(addrs, a.String())
		}

		out["pcscf_addresses"] = addrs
		sections["pcscf_addresses"] = sections["pcscf_addresses"].WithCount(len(addrs))
	}

	retention := map[string]any{}

	for _, category := range []RetentionCategory{CategoryAuditLogs, CategoryRadioLogs, CategorySubscriberUsage, CategoryFlowReports} {
		days, err := db.GetRetentionPolicy(ctx, category)
		if err != nil {
			failure("retention_policies", err)
			continue
		}

		retention[string(category)] = days
	}

	out["retention_policies"] = retention
	sections["retention_policies"] = sections["retention_policies"].WithCount(len(retention))

	if bgpSettings, err := db.GetBGPSettings(ctx); err != nil {
		failure("bgp_settings", err)
	} else {
		out["bgp_settings"] = map[string]any{
			"enabled":        bgpSettings.Enabled,
			"local_as":       bgpSettings.LocalAS,
			"router_id":      bgpSettings.RouterID,
			"listen_address": bgpSettings.ListenAddress,
		}
		sections["bgp_settings"] = sections["bgp_settings"].WithCount(1)
	}

	bgpPeers, err := db.ListAllBGPPeers(ctx)
	if err != nil {
		failure("bgp_peers", err)
		failure("bgp_import_prefixes", err)
	} else {
		peerEntries := make([]any, 0, len(bgpPeers))
		prefixEntries := []any{}

		for _, p := range bgpPeers {
			peerEntries = append(peerEntries, map[string]any{
				"id":          p.ID,
				"address":     p.Address,
				"remote_as":   p.RemoteAS,
				"hold_time":   p.HoldTime,
				"password":    "*",
				"description": p.Description,
			})

			prefixes, perr := db.ListImportPrefixesByPeer(ctx, p.ID)
			if perr != nil {
				failure("bgp_import_prefixes", perr)
				continue
			}

			for _, px := range prefixes {
				prefixEntries = append(prefixEntries, map[string]any{
					"peer_id":    px.PeerID,
					"prefix":     px.Prefix,
					"max_length": px.MaxLength,
				})
			}
		}

		out["bgp_peers"] = peerEntries
		out["bgp_import_prefixes"] = prefixEntries
		sections["bgp_peers"] = sections["bgp_peers"].WithCount(len(peerEntries))
		sections["bgp_import_prefixes"] = sections["bgp_import_prefixes"].WithCount(len(prefixEntries))
	}
}

func (db *Database) collectSupportRegistrations(ctx context.Context, out supportbundle.SanitizedData, sections map[string]supportbundle.SectionStatus, failure func(string, error)) {
	if regs, err := db.ListAllUERegistrations(ctx); err != nil {
		failure("ue_registrations", err)
	} else {
		entries := make([]any, 0, len(regs))
		for _, r := range regs {
			entries = append(entries, map[string]any{
				"imsi":    r.Imsi,
				"type":    r.Type,
				"node_id": r.NodeID,
				"purged":  r.Purged,
				"version": r.Version,
			})
		}

		out["ue_registrations"] = entries
		sections["ue_registrations"] = sections["ue_registrations"].WithCount(len(entries))
	}

	if imsRegs, err := db.ListAllIMSRegistrations(ctx); err != nil {
		failure("ims_registrations", err)
	} else {
		entries := make([]any, 0, len(imsRegs))
		for _, r := range imsRegs {
			sets := make([]any, 0, len(r.Sets))

			for _, set := range slices.Sorted(maps.Keys(r.Sets)) {
				sets = append(sets, map[string]any{
					"registration_set": set,
					"state":            r.Sets[set].State,
					"auth_pending":     r.Sets[set].AuthPending,
					"emergency":        r.Sets[set].Emergency,
				})
			}

			entries = append(entries, map[string]any{
				"imsi":              r.IMSI,
				"server_name":       r.ServerName,
				"origin_host":       r.OriginHost,
				"origin_realm":      r.OriginRealm,
				"registration_sets": sets,
			})
		}

		out["ims_registrations"] = entries
		sections["ims_registrations"] = sections["ims_registrations"].WithCount(len(entries))
	}

	if ipsmgw, err := db.ListAllIPSMGWRegistrations(ctx); err != nil {
		failure("ip_sm_gw_registrations", err)
	} else {
		entries := make([]any, 0, len(ipsmgw))
		for _, r := range ipsmgw {
			entries = append(entries, map[string]any{
				"imsi":           r.IMSI,
				"number":         r.Number,
				"diameter_host":  r.DiameterHost,
				"diameter_realm": r.DiameterRealm,
			})
		}

		out["ip_sm_gw_registrations"] = entries
		sections["ip_sm_gw_registrations"] = sections["ip_sm_gw_registrations"].WithCount(len(entries))
	}

	if waiting, err := db.ListAllSMSWaiting(ctx); err != nil {
		failure("sms_message_waiting", err)
	} else {
		entries := make([]any, 0, len(waiting))
		for _, w := range waiting {
			entries = append(entries, map[string]any{
				"imsi":            w.IMSI,
				"memory_full":     w.MemoryFull,
				"service_centres": w.ServiceCentres,
				"updated_at":      w.UpdatedAt,
			})
		}

		out["sms_message_waiting"] = entries
		sections["sms_message_waiting"] = sections["sms_message_waiting"].WithCount(len(entries))
	}

	if framedRoutes, err := db.ListAllFramedRoutes(ctx); err != nil {
		failure("subscriber_framed_routes", err)
	} else {
		entries := make([]any, 0, len(framedRoutes))
		for _, fr := range framedRoutes {
			entries = append(entries, map[string]any{
				"imsi":            fr.Imsi,
				"data_network_id": fr.DataNetworkID,
				"prefix":          fr.Prefix,
			})
		}

		out["subscriber_framed_routes"] = entries
		sections["subscriber_framed_routes"] = sections["subscriber_framed_routes"].WithCount(len(entries))
	}
}

func (db *Database) collectSupportUsage(ctx context.Context, out supportbundle.SanitizedData, sections map[string]supportbundle.SectionStatus, failure func(string, error)) {
	now := time.Now().UTC()
	usageDays := NewDayRange(now.AddDate(0, 0, -29), now)

	if usagePerDay, err := db.GetUsagePerDay(ctx, "", usageDays); err != nil {
		failure("usage_per_day", err)
	} else {
		entries := make([]any, 0, len(usagePerDay))
		for _, u := range usagePerDay {
			entries = append(entries, map[string]any{
				"day":            u.EpochDay,
				"bytes_uplink":   u.BytesUplink,
				"bytes_downlink": u.BytesDownlink,
			})
		}

		out["usage_per_day"] = entries
		sections["usage_per_day"] = sections["usage_per_day"].WithCount(len(entries))
	}

	if usagePerSub, err := db.GetUsagePerSubscriber(ctx, "", usageDays, NoUsageLimit); err != nil {
		failure("usage_per_subscriber", err)
	} else {
		entries := make([]any, 0, len(usagePerSub))
		for _, u := range usagePerSub {
			entries = append(entries, map[string]any{
				"imsi":           u.IMSI,
				"bytes_uplink":   u.BytesUplink,
				"bytes_downlink": u.BytesDownlink,
			})
		}

		out["usage_per_subscriber"] = entries
		sections["usage_per_subscriber"] = sections["usage_per_subscriber"].WithCount(len(entries))
	}
}

func (db *Database) collectSupportEvents(ctx context.Context, out supportbundle.SanitizedData, options supportbundle.ExportOptions, sections map[string]supportbundle.SectionStatus, failure func(string, error)) {
	if options.IncludeAuditLogs {
		if auditLogs, _, err := db.ListAuditLogsPage(ctx, nil, 1, 100); err != nil {
			failure("audit_logs", err)
		} else {
			entries := make([]any, 0, len(auditLogs))
			for _, a := range auditLogs {
				entries = append(entries, map[string]any{
					"id":        a.ID,
					"timestamp": a.Timestamp,
					"level":     a.Level,
					"actor":     a.Actor,
					"action":    a.Action,
					"ip":        a.IP,
					"details":   a.Details,
				})
			}

			out["audit_logs"] = entries
			sections["audit_logs"] = sections["audit_logs"].WithCount(len(entries))
		}
	} else {
		sections["audit_logs"] = supportbundle.SectionStatus{Status: "omitted", Reason: "permission restricted"}
	}

	if logs, _, err := db.ListRadioEvents(ctx, 1, 100, nil); err != nil {
		failure("radio_logs", err)
	} else {
		entries := make([]any, 0, len(logs))
		for _, l := range logs {
			entries = append(entries, map[string]any{
				"id":             l.ID,
				"timestamp":      l.Timestamp,
				"protocol":       l.Protocol,
				"message_type":   l.MessageType,
				"direction":      l.Direction,
				"local_address":  l.LocalAddress,
				"remote_address": l.RemoteAddress,
				"radio_name":     l.RadioName,
				"details":        l.Details,
			})
		}

		out["radio_logs"] = entries
		sections["radio_logs"] = sections["radio_logs"].WithCount(len(entries))
	}
}

func (db *Database) WriteSupportBundle(ctx context.Context, w io.Writer, options supportbundle.ExportOptions) error {
	data, err := db.ExportSupportData(ctx, options)
	if err != nil {
		return err
	}

	return supportbundle.GenerateSupportBundleFromData(ctx, data, w)
}

func (db *Database) collectSupportIMSServices(ctx context.Context, out supportbundle.SanitizedData, sections map[string]supportbundle.SectionStatus, failure func(string, error), profiles []Profile) {
	servers, err := db.ListIMSApplicationServers(ctx)
	if err != nil {
		failure("ims_application_servers", err)
		failure("ims_filter_criteria", err)

		return
	}

	serverEntries := make([]any, 0, len(servers))
	serverNames := make(map[string]string, len(servers))

	for _, as := range servers {
		serverNames[as.ID] = as.Name
		serverEntries = append(serverEntries, map[string]any{
			"name":                      as.Name,
			"server_name":               as.ServerName,
			"public_service_identities": as.PublicServiceIdentities,
		})
	}

	out["ims_application_servers"] = serverEntries
	sections["ims_application_servers"] = sections["ims_application_servers"].WithCount(len(serverEntries))

	criteriaEntries := []any{}

	for _, p := range profiles {
		for _, set := range IMSRegistrationSets {
			ifcs, err := db.ListProfileIMSFilterCriteria(ctx, p.ID, set)
			if err != nil {
				failure("ims_filter_criteria", err)
				continue
			}

			for i, ifc := range ifcs {
				criteriaEntries = append(criteriaEntries, map[string]any{
					"profile":                   p.Name,
					"registration_set":          set,
					"priority":                  i,
					"application_server_name":   serverNames[ifc.ApplicationServerID],
					"trigger":                   ifc.Trigger,
					"profile_part":              ifc.ProfilePart,
					"default_handling":          ifc.DefaultHandling,
					"service_info":              ifc.ServiceInfo,
					"include_register_request":  ifc.IncludeRegisterRequest,
					"include_register_response": ifc.IncludeRegisterResponse,
				})
			}
		}
	}

	out["ims_filter_criteria"] = criteriaEntries
	sections["ims_filter_criteria"] = sections["ims_filter_criteria"].WithCount(len(criteriaEntries))
}
