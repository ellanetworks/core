---
description: Step-by-step instructions to integrate Ella Core with a radio.
---

# Integrate with a Radio

Radios are automatically added to Ella Core as they connect to the network as long as they are configured to use the same Tracking Area Code (TAC), Mobile Country Code (MCC), and Mobile Network Code (MNC) as Ella Core.

Follow this guide to integrate Ella Core with a 5G radio. This guide assumes you have already deployed Ella Core.

## 1. Configure the Operator information

1. Open Ella Core in your web browser.
2. Click on the **Operator** tab in the left-hand menu.
3. Click on the **Edit** button.
4. Fill in the operator details:
    - **MCC**: The Mobile Country Code for the operator.
    - **MNC**: The Mobile Network Code for the operator.
    - **Supported TACs**: A list of supported Tracking Area Codes (TACs).

## 2. Configure the radio

In your radio's configuration, you will likely need to specify the following information to connect it with a 5G core network:

- **AMF Address**: The address of the N2 interface on Ella Core.
- **AMF Port**: The port number of the N2 interface on Ella Core.
- **PLMN ID**: The Public Land Mobile Network Identifier. This is a combination of the Mobile Country Code (MCC) and the Mobile Network Code (MNC). You can find this information in Ella Core under **Operator** and **Operator ID**.
- **TAC**: The Tracking Area Code. This is the same value you entered when adding the radio to Ella Core.
- **UPF Subnet**: IP Subnet of the N3 interface on Ella Core. For example, the default N3 IP address on Ella Core is `192.168.252.3` and the subnet is `192.168.252.0/24`.
- **SST**: The Slice/Service Type. This is a unique identifier for a network slice. In Ella Core, this value is hardcoded to `1`.
- **SD**: The Slice Differentiator. This is a unique identifier for a network slice. In Ella Core, this value is hardcoded to `102030`.

!!! note
    
    Each radio has its own configuration interface. Consult the radio's documentation for specific instructions.
