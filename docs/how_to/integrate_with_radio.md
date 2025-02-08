---
description: Step-by-step instructions to integrate Ella Core with a radio.
---

# Integrate with a Radio

Radios are automatically added to Ella Core as they connect to the network as long as they are configured to use the same Operator information as Ella Core.

Follow this guide to integrate Ella Core with a 5G radio. This guide assumes you have already deployed Ella Core.

## 1. Configure the Operator information

1. Open Ella Core in your web browser.
2. Click on the **Operator** tab in the left-hand menu.
3. Edit the Operator information:
    - **MCC**: The Mobile Country Code for the operator.
    - **MNC**: The Mobile Network Code for the operator.
    - **Supported TACs**: A list of supported Tracking Area Codes (TACs).
    - **SST**: The Slice/Service Type.
    - **SD**: The Service Differentiator.

## 2. Configure the radio

In your radio's configuration, you will likely need to specify the following information to connect it with a 5G core network:

- **AMF Address**: The address of the N2 interface on Ella Core.
- **PLMN ID**: The Public Land Mobile Network Identifier. This is a combination of the Mobile Country Code (MCC) and the Mobile Network Code (MNC). You can find this information in Ella Core under **Operator** and **Operator ID**.
- **TAC**: The Tracking Area Code. You can find this information in Ella Core under **Operator** and **Supported TACs**.
- **SST**: The Slice/Service Type. This is a unique identifier for a network slice. You can find this information in Ella Core under **Operator** and **SST**.
- **SD**: The Service Differentiator. This is a unique identifier for a network slice. You can find this information in Ella Core under **Operator** and **SD**.

!!! note
    
    Each radio has its own configuration interface. Consult the radio's documentation for specific instructions.
