---
description: How Ella Core joins its OpenTelemetry traces with the RAN's through 3GPP subscriber trace.
---

# RAN Trace Correlation

Ella Core exports OpenTelemetry spans for every NGAP and S1AP message it handles. On its own, each trace stops at the N2 or S1 interface: nothing ties Ella Core's spans for a UE to what the radio did before, between and after them. RAN trace correlation closes that gap by sharing one trace identifier between Ella Core and the radio.

## Why 3GPP subscriber trace

NGAP and S1AP have no field for an OpenTelemetry trace context, but they have one for a trace identifier: the Trace Activation IE that 3GPP defines for subscriber and equipment trace (TS 32.421, TS 32.422). Its NG-RAN or E-UTRAN Trace ID is the key 3GPP uses to correlate trace data across network elements, which is the job RAN trace correlation needs done. The IE also travels where the trace must follow: the gNB-CU passes it to the gNB-DU and gNB-CU-UP over F1AP and E1AP, and a source radio passes it to the target over Xn or X2 at handover.

Both the Trace Start procedure and the Trace Activation IE have criticality "ignore", so a radio that does not implement 3GPP trace ignores them and carries on.

## The trace model

Ella Core acts as the management system, UDM/HSS, AMF and MME of the 3GPP signalling-based activation flow, all in one process. When telemetry is enabled it activates one Trace Session for every subscriber, identified by a Trace Reference made of the operator PLMN and a 3-octet Trace ID. The Trace ID starts with the node's AMF Pointer so that the nodes of a cluster never hand out the same value.

The AMF and the MME each run Trace Recording Sessions within that Trace Session. A recording session starts on a procedure, such as a registration, an attach or a service request, and stops when that procedure ends. Each one gets a new Trace Recording Session Reference (TRSR), and the AMF or MME sends it to the radio in a TRACE START.

A radio keeps the first Trace Reference and TRSR it receives for a UE until it releases the UE context, and ignores later TRACE START messages for the same Trace Reference. The 8-octet Trace ID the radio holds therefore identifies one radio connection. Ella Core uses that same identifier for the connection: every span for the connection carries the OpenTelemetry trace ID built from it, so the radio and the core see one trace per connection.

The connection's trace survives handovers through the core and Xn or X2 path switches, because the target radio receives the same identifier. A 4G to 5G relocation, or the reverse, starts a new trace, because the target AMF or MME starts its own recording session.

## When the trace starts

A subscriber trace can only start once the subscriber is identified and its trace data is available. A UE that registers or attaches with a temporary identity Ella Core already knows is traced from its first NAS message. A UE that registers with a SUCI or attaches with an IMSI is traced from the point Ella Core has authenticated it and obtained its subscription data, so the radio's spans for that first authentication are not part of the trace.

## Departures from 3GPP

Ella Core uses 3GPP subscriber trace for the identifier, not for trace records:

- The Trace Collection Entity IP Address is mandatory in the Trace Activation IE and names the collector of 3GPP trace record files. Ella Core runs no such collector and sends its N2 address. A radio that produces 3GPP trace records fails to upload them.
- TS 32.421 says the AMF and MME should produce trace records. Ella Core produces OpenTelemetry spans instead.

## Impact on radios

A radio that implements 3GPP subscriber trace starts a trace recording for every UE when telemetry is enabled. Commercial radios limit the number of concurrent UE traces and may skip recording once that limit is reached, so correlation can stop after a few UEs and traces started from the operator's own management system can compete for the same slots. Open source radios that do not implement TRACE START log the unsupported procedure.
