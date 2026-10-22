---
description: Step-by-step instructions to serve Mission Critical Push To Talk (MCPTT) with Ella Core and an IMS.
---

# Enable MCPTT

This guide shows how to serve Mission Critical Push To Talk (MCPTT) to subscribers over 4G and 5G, with an external IMS and an MCPTT server. To learn how Ella Core works with the IMS, see [IMS Integration](../explanation/ims_integration.md).

## Prerequisites

- **IMS**: an IMS integrated with Ella Core, as in [Integrate with an IMS](integrate_with_ims.md). Its P-CSCF must send the MCPTT ICSI, `urn:urn-7:3gpp-service.ims.icsi.mcptt`, in the AF-Application-Identifier of MCPTT calls.
- **MCPTT server**: a 3GPP MCPTT server, reachable from the IMS.
- **MCPTT clients**: configured with the MC data network (`mcx` below) as their MC services APN or DNN.
- **Radios**: support for QCI/5QI 69 and 65.

!!! warning

    Ella Core uses QCI/5QI 69 for MC signalling and 65 for MCPTT calls, and has no setting to use other classes. Check your radios' documentation before you start.

## 1. Create the MC data network

In the Ella Core UI, go to the **Networking** page and open the **Data Networks** tab.

Click **Create**, enter `mcx` as the **Name**, set an IP pool, turn on **IMS (voice and mission critical services)**, and click **Create**.

In your IMS, add a route to the `mcx` IP pool via Ella Core's N6 address.

## 2. Create the MC profile

Go to the **Profiles** page and click **Create**. Enter a **Name**, e.g. `mcptt`, and click **Create**.

Click on the new profile, click **Add Policy**, and set:

- **Data Network**: `mcx`.
- **5QI / QCI**: `69 — Mission critical signalling`.
- **Allocation and Retention Priority (ARP)**: a level between 1 and 8, e.g. `3`.
- **Pre-emption capability**: `May pre-empt`.
- **Pre-emption vulnerability**: `Not pre-emptable`.

Click **Create**.

Optionally, to let MC sessions take radio resources from other subscribers under load, set the **Pre-emption vulnerability** of the policies on your other profiles to `Pre-emptable`.

## 3. Add the MCPTT application server

Go to the **Operator** page and scroll to the **IMS** section. Under **Application Servers**, click **Add Application Server** and enter:

- **Name**: `mcptt`.
- **Server Name**: the SIP URI of your MCPTT server, e.g. `sip:mcptt.ims.mnc001.mcc001.3gppnetwork.org`.
- **Identity 1**: `sip:!.*!@mcptt.<IMS home domain>`, e.g. `sip:!.*!@mcptt.ims.mnc001.mcc001.3gppnetwork.org`.

Click **Add**.

Configure your MCPTT server's identities, such as its group identities, in the same domain, so that they match this identity.

The same can be done with the [IMS application servers API](../reference/api/operator.md#create-an-ims-application-server).

## 4. Add the MCPTT filter criteria to the MC profile

Go to the **Profiles** page and click on the MC profile. Click the edit icon next to **IMS Filter Criteria**.

Under **Add from preset**, select the `MCPTT` **Preset** and the `mcptt` **Application Server**, and click **Add**. Click **Save**.

The same can be done with the [profile IMS filter criteria API](../reference/api/profiles.md#update-a-profiles-ims-initial-filter-criteria), with this body:

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
- Give the subscriber an MSISDN, as in [Enable Voice](enable_voice.md#3-give-subscribers-an-msisdn), and map its IMS public identity, `tel:+<msisdn>`, to an MCPTT ID on your MCPTT server.

## 6. Configure the call priorities on the MCPTT server

Configure your MCPTT server to set the Resource-Priority of each call:

| Call type | Resource-Priority | ARP priority |
| --------- | ----------------- | ------------ |
| Emergency | `mcpttp.15` | 1 |
| Imminent peril | `mcpttp.13` | 2 |
| Normal | `mcpttp.11` | 3 |

## 7. Check the call QoS

Make an MCPTT call, and get a member with the [Subscribers API](../reference/api/subscribers.md#get-a-subscriber): `GET /api/v1/subscribers/{imsi}`. During the call, its session on `mcx` lists two `bearers`:

- The default one, with QCI/5QI 69 and the policy's ARP.
- A GBR one with QCI/5QI 65 and the call's ARP, which carries voice and floor control.

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

If the session has no QCI/5QI 65 bearer during a call, check that your radio supports QCI/5QI 65.
