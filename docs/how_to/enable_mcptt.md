---
description: Step-by-step instructions to serve Mission Critical Push To Talk (MCPTT) with Ella Core and an IMS.
---

# Enable MCPTT

Ella Core can serve Mission Critical Push To Talk (MCPTT) to subscribers over 4G and 5G, with an external IMS and an MCPTT server. Ella Core is the IMS's HSS and PCRF: it tells the IMS which requests to send to the MCPTT server, and gives MCPTT calls their mission critical QoS.

This guide uses [Ella IMS](https://github.com/ellanetworks/ims), but any 3GPP-compliant IMS can be used.

## Prerequisites

- **IMS**: an IMS that uses Ella Core as its HSS and PCRF, set up as in steps 1 to 3 of [Enable Voice](enable_voice.md). Its P-CSCF must send the MCPTT ICSI, `urn:urn-7:3gpp-service.ims.icsi.mcptt`, in the AF-Application-Identifier of MCPTT calls (TS 29.214 §A.13.1).
- **MCPTT server**: a 3GPP MCPTT server, reachable from the IMS.
- **MCPTT clients**: configured with the MC data network (`mcx` below) as their MC services APN or DNN (TS 23.280 §5.2.7.0).
- **Radio QoS support**: your radios must support the following QoS classes, as QCIs on 4G and 5QIs on 5G.

| QCI / 5QI | Used for |
|---|---|
| 69 | SIP signalling on the MC data network (TS 23.280 §5.2.7.2.1) |
| 65 | MCPTT voice and floor control (TS 23.379 §5.7.3, TS 23.289 §4.3.3.2) |

!!! warning

    QCI/5QI 65 and 69 are mandatory for MCPTT, but radios are not required to support every standardized QCI (TS 23.401 §4.7.5). A radio without them refuses the bearer or QoS flow: without 69, the session on the MC data network fails, and without 65, the call fails. Ella Core has no setting to use other classes instead. Check your radios' documentation before you start.

## 1. Create the MC data network

MCPTT clients use their own data network for MCPTT signalling and media.

In the Ella Core UI, go to the **Networking** page and open the **Data Networks** tab.

Click **Create**, enter `mcx` as the **Name**, set an IP pool, turn on **IMS (voice and mission critical services)**, and click **Create**.

Like `ims`, an IMS data network gives phones the P-CSCF addresses, is never NAT translated, and switches UE-to-UE traffic locally. Only the `ims` data network indicates voice over PS to phones; `mcx` does not.

In your IMS, add a route to the `mcx` IP pool via Ella Core's N6 address, as for the voice data network.

## 2. Create the MC profile

For MCPTT users, create a profile with a policy on the MC data network.

Go to the **Profiles** page and click **Create**. Enter a **Name**, e.g. `mcptt`, and click **Create**.

Click on the new profile, click **Add Policy**, and set:

- **Data Network**: `mcx`.
- **5QI / QCI**: `69 — Mission critical signalling`.
- **Allocation and Retention Priority (ARP)**: a level between 1 and 8, the levels meant for prioritized services (TS 29.213 Table 6.3.2), e.g. `3`.
- **Pre-emption capability**: `May pre-empt` to let the session's default bearer or QoS flow pre-empt others at admission.
- **Pre-emption vulnerability**: `Not pre-emptable`. If the radio pre-empts a session's default bearer or QoS flow, the whole session is released.

Click **Create**.

Policies on other profiles are not pre-emptable by default. To let MC sessions take their radio resources under load, set those policies' **Pre-emption vulnerability** to `Pre-emptable`.

## 3. Add the MCPTT application server

An application server is a SIP server that the IMS sends some requests to. It has:

- A **Server Name**: the SIP URI of the MCPTT server that the S-CSCF sends requests to, e.g. `sip:mcptt.ims.mnc001.mcc001.3gppnetwork.org`.
- **Public Service Identities (PSIs)**: the SIP or tel URIs that the server hosts, such as its MCPTT groups. When a request is addressed to one of them, the IMS asks Ella Core where to send it, and Ella Core answers with the application server, which the I-CSCF reaches directly.

A PSI is either distinct, such as `sip:group-1@mcptt.ims.mnc001.mcc001.3gppnetwork.org`, or wildcarded. A wildcarded PSI holds an extended regular expression delimited by `!` in its user part (TS 23.003 §13.5). For example, `sip:!.*!@mcptt.ims.mnc001.mcc001.3gppnetwork.org` covers every identity at `mcptt.ims.mnc001.mcc001.3gppnetwork.org`.

Go to the **Operator** page and scroll to the **IMS** section. Under **Application Servers**, click **Add Application Server** and enter:

- **Name**: `mcptt`.
- **Server Name**: the SIP URI of your MCPTT server, e.g. `sip:mcptt.ims.mnc001.mcc001.3gppnetwork.org`.
- **Identity 1**: `sip:!.*!@mcptt.<IMS home domain>`, e.g. `sip:!.*!@mcptt.ims.mnc001.mcc001.3gppnetwork.org`.

Click **Add**.

Configure your MCPTT server with identities, such as its group identities, in the same domain, so that they match this PSI.

The same can be done with the [IMS application servers API](../reference/api/operator.md#create-an-ims-application-server).

## 4. Add the MCPTT filter criteria to the MC profile

A profile's IMS initial filter criteria tell the S-CSCF which requests of its subscribers to send to an application server.

Go to the **Profiles** page and click on the MC profile. Click the edit icon next to **IMS Filter Criteria**.

Under **Add from preset**, select the `MCPTT` **Preset** and the `mcptt` **Application Server**, and click **Add**. Click **Save**.

The preset adds two filter criteria, both to the MCPTT server:

1. **Registration**: every REGISTER request, with **Include Register Request** and **Include Register Response**. When a subscriber registers, re-registers or de-registers, the S-CSCF sends a third-party REGISTER to the MCPTT server, with the subscriber's REGISTER request and its 200 (OK) response in the body (TS 24.229 §5.4.1.7A). The MCPTT server authorises the MCPTT client from that REGISTER request (TS 24.379 §7.3.2). If the MCPTT server can't be reached, the registration continues.
2. **Originating MCPTT sessions**: the subscriber's own requests whose P-Asserted-Service is the MCPTT ICSI, `urn:urn-7:3gpp-service.ims.icsi.mcptt`. If the MCPTT server can't be reached, the request fails.

Requests that the MCPTT server sends to subscribers need no filter criteria.

Changes to a profile's filter criteria, or to an application server's server name, reach registered subscribers through a Cx Push Profile request to their S-CSCF. If your IMS does not support it, subscribers get the change when they next register.

With the API, set the same filter criteria with the [profile IMS filter criteria API](../reference/api/profiles.md#update-a-profiles-ims-initial-filter-criteria):

```json
{
    "initial_filter_criteria": [
        {
            "application_server_name": "mcptt",
            "trigger_point": {
                "condition_type_cnf": false,
                "spt": [{"group": [0], "method": "REGISTER"}]
            },
            "default_handling": "SESSION_CONTINUED",
            "include_register_request": true,
            "include_register_response": true
        },
        {
            "application_server_name": "mcptt",
            "trigger_point": {
                "condition_type_cnf": true,
                "spt": [
                    {"group": [0], "session_case": "ORIGINATING_REGISTERED"},
                    {
                        "group": [1],
                        "sip_header": {
                            "header": "^P-Asserted-Service$",
                            "content": "^urn:urn-7:3gpp-service\\.ims\\.icsi\\.mcptt$"
                        }
                    }
                ]
            },
            "default_handling": "SESSION_TERMINATED"
        }
    ]
}
```

## 5. Put MCPTT users on the MC profile

For each MCPTT user:

- Go to the **Subscribers** page and click on the subscriber.
- In Provisioning, click the edit icon next to **Profile**, select the MC profile, and click **Update**.
- Give the subscriber an MSISDN, as in [Enable Voice](enable_voice.md#6-give-subscribers-an-msisdn). Its IMS public identity, `tel:+<msisdn>`, is the identity your MCPTT server maps to an MCPTT ID.

## 6. Configure the call priorities on the MCPTT server

Ella Core sets the priority of an MCPTT call from its Resource-Priority. The P-CSCF only uses an authorised Resource-Priority header field (TS 24.229 §3.1), one received from within the IMS's trust domain, and passes it to Ella Core as the MCPTT-Identifier and Reservation-Priority (TS 29.214 §A.13.2). The MCPTT server must therefore set the Resource-Priority of each call.

Configure your MCPTT server with the following Resource-Priority values:

| Call type | Resource-Priority | ARP priority |
| --------- | ----------------- | ------------ |
| Emergency | `mcpttp.15` | 1 |
| Imminent peril | `mcpttp.13` | 2 |
| Normal | `mcpttp.11` | 3 |

These meet the order TS 23.379 §5.7.3 requires: emergency calls above imminent peril calls, and imminent peril calls above normal calls.

## 7. Check the call QoS

Make an MCPTT call. During the call, each member's session on `mcx` has:

- The default bearer or QoS flow, with QCI/5QI 69 and the policy's ARP.
- One GBR bearer (4G) or QoS flow (5G) with QCI/5QI 65, which carries both voice and floor control.

The call's ARP is derived from its Resource-Priority:

| Resource-Priority | ARP priority | Pre-emption capability | Pre-emption vulnerability |
| ----------------- | ------------ | ---------------------- | ------------------------- |
| `mcpttp.<n>` | 8 − ⌊n / 2⌋, e.g. 1 for `mcpttp.15` | May pre-empt | Not pre-emptable at priority 1 and 2, pre-emptable otherwise |
| `mcpttq.<n>` | 8 − ⌊n / 2⌋ | Shall not pre-empt | Not pre-emptable at priority 1 and 2, pre-emptable otherwise |
| None | 8 | Shall not pre-empt | Pre-emptable |

`mcpttq` is the queueing namespace (RFC 8101), so it never pre-empts. When the call's priority changes, for example when a normal call becomes an emergency call, Ella Core modifies the ARP of the existing bearer or QoS flow instead of creating a new one.

To see them, get the subscriber with the [Subscribers API](../reference/api/subscribers.md#get-a-subscriber): `GET /api/v1/subscribers/{imsi}`. Each session lists its `bearers`, the default one first:

```json
"bearers": [
  {
    "id": 5,
    "default": true,
    "var5qi": 69,
    "arp": 3,
    "preemption_capability": "may_preempt",
    "preemption_vulnerability": "not_preemptable",
    "rules": []
  },
  {
    "id": 6,
    "default": false,
    "var5qi": 65,
    "arp": 3,
    "preemption_capability": "may_preempt",
    "preemption_vulnerability": "preemptable",
    "mbr_uplink": "49 Kbps",
    "mbr_downlink": "49 Kbps",
    "gbr_uplink": "49 Kbps",
    "gbr_downlink": "49 Kbps",
    "rules": ["pcscf.ims.mnc001.mcc001.3gppnetwork.org;1;1#1.1", "pcscf.ims.mnc001.mcc001.3gppnetwork.org;1;1#2.1"]
  }
]
```

If the session has no 65 bearer or QoS flow during a call, check that your radio supports QCI/5QI 65.
