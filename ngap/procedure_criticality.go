// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

// Criticality assigned to each elementary procedure by the NGAP-ELEMENTARY-
// PROCEDURE definitions of TS 38.413 §9.4.3, and whether the procedure defines
// a message to report an unsuccessful outcome.
type procedureInfo struct {
	crit    Criticality
	outcome bool
}

var procedureInfos = map[ProcedureCode]procedureInfo{
	ProcAMFConfigurationUpdate:                {CriticalityReject, true},
	ProcAMFStatusIndication:                   {CriticalityIgnore, false},
	ProcCellTrafficTrace:                      {CriticalityIgnore, false},
	ProcDeactivateTrace:                       {CriticalityIgnore, false},
	ProcDownlinkNASTransport:                  {CriticalityIgnore, false},
	ProcDownlinkNonUEAssociatedNRPPaTransport: {CriticalityIgnore, false},
	ProcDownlinkRANConfigurationTransfer:      {CriticalityIgnore, false},
	ProcDownlinkRANStatusTransfer:             {CriticalityIgnore, false},
	ProcDownlinkUEAssociatedNRPPaTransport:    {CriticalityIgnore, false},
	ProcErrorIndication:                       {CriticalityIgnore, false},
	ProcHandoverCancel:                        {CriticalityReject, false},
	ProcHandoverNotification:                  {CriticalityIgnore, false},
	ProcHandoverPreparation:                   {CriticalityReject, true},
	ProcHandoverResourceAllocation:            {CriticalityReject, true},
	ProcInitialContextSetup:                   {CriticalityReject, true},
	ProcInitialUEMessage:                      {CriticalityIgnore, false},
	ProcLocationReportingControl:              {CriticalityIgnore, false},
	ProcLocationReportingFailureIndication:    {CriticalityIgnore, false},
	ProcLocationReport:                        {CriticalityIgnore, false},
	ProcNASNonDeliveryIndication:              {CriticalityIgnore, false},
	ProcNGReset:                               {CriticalityReject, false},
	ProcNGSetup:                               {CriticalityReject, true},
	ProcOverloadStart:                         {CriticalityIgnore, false},
	ProcOverloadStop:                          {CriticalityReject, false},
	ProcPaging:                                {CriticalityIgnore, false},
	ProcPathSwitchRequest:                     {CriticalityReject, true},
	ProcPDUSessionResourceModify:              {CriticalityReject, false},
	ProcPDUSessionResourceModifyIndication:    {CriticalityReject, false},
	ProcPDUSessionResourceRelease:             {CriticalityReject, false},
	ProcPDUSessionResourceSetup:               {CriticalityReject, false},
	ProcPDUSessionResourceNotify:              {CriticalityIgnore, false},
	ProcPrivateMessage:                        {CriticalityIgnore, false},
	ProcPWSCancel:                             {CriticalityReject, false},
	ProcPWSFailureIndication:                  {CriticalityIgnore, false},
	ProcPWSRestartIndication:                  {CriticalityIgnore, false},
	ProcRANConfigurationUpdate:                {CriticalityReject, true},
	ProcRerouteNASRequest:                     {CriticalityReject, false},
	ProcRRCInactiveTransitionReport:           {CriticalityIgnore, false},
	ProcTraceFailureIndication:                {CriticalityIgnore, false},
	ProcTraceStart:                            {CriticalityIgnore, false},
	ProcUEContextModification:                 {CriticalityReject, true},
	ProcUEContextRelease:                      {CriticalityReject, false},
	ProcUEContextReleaseRequest:               {CriticalityIgnore, false},
	ProcUERadioCapabilityCheck:                {CriticalityReject, false},
	ProcUERadioCapabilityInfoIndication:       {CriticalityIgnore, false},
	ProcUETNLABindingRelease:                  {CriticalityIgnore, false},
	ProcUplinkNASTransport:                    {CriticalityIgnore, false},
	ProcUplinkNonUEAssociatedNRPPaTransport:   {CriticalityIgnore, false},
	ProcUplinkRANConfigurationTransfer:        {CriticalityIgnore, false},
	ProcUplinkRANStatusTransfer:               {CriticalityIgnore, false},
	ProcUplinkUEAssociatedNRPPaTransport:      {CriticalityIgnore, false},
	ProcWriteReplaceWarning:                   {CriticalityReject, false},
	ProcSecondaryRATDataUsageReport:           {CriticalityIgnore, false},
	ProcUplinkRIMInformationTransfer:          {CriticalityIgnore, false},
	ProcDownlinkRIMInformationTransfer:        {CriticalityIgnore, false},
	ProcRetrieveUEInformation:                 {CriticalityReject, false},
	ProcUEInformationTransfer:                 {CriticalityReject, false},
	ProcRANCPRelocationIndication:             {CriticalityReject, false},
	ProcUEContextResume:                       {CriticalityReject, true},
	ProcUEContextSuspend:                      {CriticalityReject, true},
	ProcUERadioCapabilityIDMapping:            {CriticalityReject, false},
	ProcHandoverSuccess:                       {CriticalityIgnore, false},
	ProcUplinkRANEarlyStatusTransfer:          {CriticalityReject, false},
	ProcDownlinkRANEarlyStatusTransfer:        {CriticalityIgnore, false},
	ProcAMFCPRelocationIndication:             {CriticalityReject, false},
	ProcConnectionEstablishmentIndication:     {CriticalityReject, false},
	ProcBroadcastSessionModification:          {CriticalityReject, true},
	ProcBroadcastSessionRelease:               {CriticalityReject, false},
	ProcBroadcastSessionSetup:                 {CriticalityReject, true},
	ProcDistributionSetup:                     {CriticalityReject, true},
	ProcDistributionRelease:                   {CriticalityReject, false},
	ProcMulticastSessionActivation:            {CriticalityReject, true},
	ProcMulticastSessionDeactivation:          {CriticalityReject, false},
	ProcMulticastSessionUpdate:                {CriticalityReject, true},
	ProcMulticastGroupPaging:                  {CriticalityIgnore, false},
	ProcBroadcastSessionReleaseRequired:       {CriticalityReject, false},
	ProcTimingSynchronisationStatus:           {CriticalityReject, true},
	ProcTimingSynchronisationStatusReport:     {CriticalityIgnore, false},
	ProcMTCommunicationHandling:               {CriticalityReject, true},
	ProcRANPagingRequest:                      {CriticalityIgnore, false},
	ProcBroadcastSessionTransport:             {CriticalityReject, true},
}

// ProcedureCriticality reports the criticality TS 38.413 §9.4.3 assigns the
// procedure, which Criticality Diagnostics reports (§9.3.1.3).
func ProcedureCriticality(p ProcedureCode) Criticality {
	return procedureInfos[p].crit
}

// hasUnsuccessfulOutcome reports whether the procedure defines a message to
// report an unsuccessful outcome. TS 38.413 §10.3.5 rejects with that message
// where it exists, and falls back to Error Indication where it does not.
func hasUnsuccessfulOutcome(p ProcedureCode) bool {
	return procedureInfos[p].outcome
}
