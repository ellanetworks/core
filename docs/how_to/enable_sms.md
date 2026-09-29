---
description: Step-by-step instructions to enable SMS in Ella Core.
---

# Enable SMS (beta)

Ella Core can be integrated with an external Short Message Service Center (SMSC) to allow subscribers in your private mobile network to communicate via SMS.

## 1. Configure the SMSC

Set your SMSC's HSS realm to `epc.mnc<MNC>.mcc<MCC>.3gppnetwork.org`, using your network's MCC and 3-digit MNC.

Example: `epc.mnc001.mcc001.3gppnetwork.org`

## 2. Configure Ella Core's SMS settings

In the Ella Core UI, go to the Operator page. In the SMS section, configure the following settings:

- **SMSC Address**: your SMSC's Diameter IP address.
- **SMSC Port**: your SMSC's Diameter port (default 3868).
- **SMS Number**: your network's E.164 number for SMS, e.g. `+15550001111`.

Click **Update**.

Validate that **SMSC Link** shows **Connected**.

## 3. Give subscribers an MSISDN

For each subscriber that should send or receive SMS:

- Go to the Subscribers page and click on the subscriber you want to configure.
- In Provisioning, click the edit icon next to MSISDN.
- Enter the subscriber's E.164 number, e.g. `+15551230001`, and click on **Update**.

## 4. Set the SMSC number on the phones

On each phone's SIM, set the SMSC number to your SMSC's service centre address, e.g. `+15550000000`.

## 5. Send a test SMS

Send a test SMS message from a phone to another in your private mobile network. The message should be delivered successfully.
