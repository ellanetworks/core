---
description: Explanation of SIM card management in 5G Core networks.
---

# Managing SIM Cards

As a network operator, you will need to provision a SIM card for each subscriber you create in Ella Core. There are two main approaches to managing SIM cards in a Private 5G network.

## Using Physical SIM Cards

You can obtain physical SIM cards from a SIM card vendor (ex. [Sysmocom Programmable SIM Cards](https://sysmocom.de/products/sim/sysmoisim-sja5/index.html)) and burn the necessary information (IMSI, Key, OPc) onto the SIM cards. These SIM cards can then be distributed to subscribers for use in their devices.

### Burning SIM Cards

You can burn the SIM card using a card reader/writer (ex. [OmniKey 3121](https://www.hidglobal.com/products/readers/omnikey-3121)) along with software provided by the SIM card vendor. The software will allow you to input the subscriber information (IMSI, Key, OPc) and write it to the SIM card.

For example, using Osmocom's [pysim](https://github.com/osmocom/pysim) software, you can burn a SIM card with the following command:

```shell
export IMSI=001018435063221
export KEY=525c8e65e8449a7067c1ca4367098c60
export OPC=a5db238bfaa2c9f01704332378f10f65
export MCC=001
export MNC=01
export ADMIN_CODE=76543210
./pySim-prog.py -p0 -n Ella -t sysmoISIM-SJA5 -i $IMSI -c $MCC -x $MCC -y $MNC -o $OPC -k $KEY -a $ADMIN_CODE -j 1
```

## Using eSIM

eSIMs (embedded SIMs) allow for remote provisioning of SIM profiles. This can simplify the management of SIM cards, especially in large-scale deployments. You can use an eSIM management platform to create and manage subscriber profiles, which can then be downloaded to eSIM-enabled devices.
