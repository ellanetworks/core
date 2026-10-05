---
description: Step-by-step instructions to enable SMS in Ella Core.
---

# Enable SMS (beta)

Ella Core can be integrated with one or more external Short Message Service Centers (SMSCs) to allow subscribers in your private mobile network to communicate via SMS.

## 1. Configure the SMSC

Set your SMSC's HSS realm to `epc.mnc<MNC>.mcc<MCC>.3gppnetwork.org`, using your network's MCC and 3-digit MNC.

Example: `epc.mnc001.mcc001.3gppnetwork.org`

## 2. Turn on SMS in Ella Core

In the Ella Core UI, go to the Operator page and set the SMS switch to **ON**.

Click the edit icon next to **SMS Number**, enter your network's E.164 number for SMS, e.g. `+15550001111`, and click **Update**.

Click **Add Service Center** and configure the following settings:

- **Address**: your SMSC's Diameter IP address.
- **Numbers**: the E.164 service centre addresses your SMSC serves, e.g. `+15550000000`.

Click **Add**.

Validate that each service center's **Status** shows **Connected**.

## 3. Give subscribers an MSISDN

For each subscriber that should send or receive SMS:

- Go to the Subscribers page and click on the subscriber you want to configure.
- In Provisioning, click the edit icon next to MSISDN.
- Enter the subscriber's E.164 number, e.g. `+15551230001`, and click on **Update**.

## 4. Set the SMSC number on the phones

On each phone's SIM, set the SMSC number to one of your SMSCs' service centre numbers, e.g. `+15550000000`. Ella Core rejects messages sent to a service centre number that no SMSC serves.

## 5. Send a test SMS

Send a test SMS message from a phone to another in your private mobile network. The message should be delivered successfully.
