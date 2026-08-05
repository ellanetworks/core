// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

// Criticality assigned to each elementary procedure by the S1AP-ELEMENTARY-
// PROCEDURE definitions of TS 36.413 §9.3.2, and whether the procedure
// defines a message to report an unsuccessful outcome.
type procedureInfo struct {
	crit    Criticality
	outcome bool
}

var procedureInfos = map[ProcedureCode]procedureInfo{
	ProcHandoverPreparation:                  {CriticalityReject, true},
	ProcHandoverResourceAllocation:           {CriticalityReject, true},
	ProcHandoverNotification:                 {CriticalityIgnore, false},
	ProcPathSwitchRequest:                    {CriticalityReject, true},
	ProcHandoverCancel:                       {CriticalityReject, false},
	ProcERABSetup:                            {CriticalityReject, false},
	ProcERABModify:                           {CriticalityReject, false},
	ProcERABRelease:                          {CriticalityReject, false},
	ProcERABReleaseIndication:                {CriticalityIgnore, false},
	ProcInitialContextSetup:                  {CriticalityReject, true},
	ProcPaging:                               {CriticalityIgnore, false},
	ProcDownlinkNASTransport:                 {CriticalityIgnore, false},
	ProcInitialUEMessage:                     {CriticalityIgnore, false},
	ProcUplinkNASTransport:                   {CriticalityIgnore, false},
	ProcReset:                                {CriticalityReject, false},
	ProcErrorIndication:                      {CriticalityIgnore, false},
	ProcNASNonDeliveryIndication:             {CriticalityIgnore, false},
	ProcS1Setup:                              {CriticalityReject, true},
	ProcUEContextReleaseRequest:              {CriticalityIgnore, false},
	ProcDownlinkS1cdma2000tunnelling:         {CriticalityIgnore, false},
	ProcUplinkS1cdma2000tunnelling:           {CriticalityIgnore, false},
	ProcUEContextModification:                {CriticalityReject, true},
	ProcUECapabilityInfoIndication:           {CriticalityIgnore, false},
	ProcUEContextRelease:                     {CriticalityReject, false},
	ProcENBStatusTransfer:                    {CriticalityIgnore, false},
	ProcMMEStatusTransfer:                    {CriticalityIgnore, false},
	ProcDeactivateTrace:                      {CriticalityIgnore, false},
	ProcTraceStart:                           {CriticalityIgnore, false},
	ProcTraceFailureIndication:               {CriticalityIgnore, false},
	ProcENBConfigurationUpdate:               {CriticalityReject, true},
	ProcMMEConfigurationUpdate:               {CriticalityReject, true},
	ProcLocationReportingControl:             {CriticalityIgnore, false},
	ProcLocationReportingFailureIndication:   {CriticalityIgnore, false},
	ProcLocationReport:                       {CriticalityIgnore, false},
	ProcOverloadStart:                        {CriticalityIgnore, false},
	ProcOverloadStop:                         {CriticalityReject, false},
	ProcWriteReplaceWarning:                  {CriticalityReject, false},
	ProcENBDirectInformationTransfer:         {CriticalityIgnore, false},
	ProcMMEDirectInformationTransfer:         {CriticalityIgnore, false},
	ProcPrivateMessage:                       {CriticalityIgnore, false},
	ProcENBConfigurationTransfer:             {CriticalityIgnore, false},
	ProcMMEConfigurationTransfer:             {CriticalityIgnore, false},
	ProcCellTrafficTrace:                     {CriticalityIgnore, false},
	ProcKill:                                 {CriticalityReject, false},
	ProcDownlinkUEAssociatedLPPaTransport:    {CriticalityIgnore, false},
	ProcUplinkUEAssociatedLPPaTransport:      {CriticalityIgnore, false},
	ProcDownlinkNonUEAssociatedLPPaTransport: {CriticalityIgnore, false},
	ProcUplinkNonUEAssociatedLPPaTransport:   {CriticalityIgnore, false},
	ProcUERadioCapabilityMatch:               {CriticalityReject, false},
	ProcPWSRestartIndication:                 {CriticalityIgnore, false},
	ProcERABModificationIndication:           {CriticalityReject, false},
	ProcPWSFailureIndication:                 {CriticalityIgnore, false},
	ProcRerouteNASRequest:                    {CriticalityReject, false},
	ProcUEContextModificationIndication:      {CriticalityReject, false},
	ProcConnectionEstablishmentIndication:    {CriticalityReject, false},
	ProcUEContextSuspend:                     {CriticalityReject, false},
	ProcUEContextResume:                      {CriticalityReject, true},
	ProcNASDeliveryIndication:                {CriticalityIgnore, false},
	ProcRetrieveUEInformation:                {CriticalityReject, false},
	ProcUEInformationTransfer:                {CriticalityReject, false},
	ProcENBCPRelocationIndication:            {CriticalityReject, false},
	ProcMMECPRelocationIndication:            {CriticalityReject, false},
	ProcSecondaryRATDataUsageReport:          {CriticalityIgnore, false},
	ProcUERadioCapabilityIDMapping:           {CriticalityReject, false},
	ProcHandoverSuccess:                      {CriticalityIgnore, false},
	ProcENBEarlyStatusTransfer:               {CriticalityReject, false},
	ProcMMEEarlyStatusTransfer:               {CriticalityIgnore, false},
}

// TS 36.413 §9.3.2.
func ProcedureCriticality(p ProcedureCode) Criticality {
	return procedureInfos[p].crit
}

// §10.3.5 rejects with that message where it exists, and falls back to Error
// Indication where it does not.
func hasUnsuccessfulOutcome(p ProcedureCode) bool {
	return procedureInfos[p].outcome
}
