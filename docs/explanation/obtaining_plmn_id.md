---
description: Explanation of obtaining a PLMN ID for private networks.
---

# Obtaining a PLMN ID for a Private Network

## Overview

A Public Land Mobile Network (PLMN) ID is a globally unique identifier that allows mobile devices to recognize and connect to mobile networks. In 5G private networks, having a correct PLMN ID is critical for ensuring proper network identification and operation.

A PLMN ID consists of two parts:

- **Mobile Country Code (MCC):** A three-digit code that identifies the country.
- **Mobile Network Code (MNC):** A two- or three-digit code that identifies the network operator within that country.

## How PLMN IDs Are Assigned

PLMN IDs are regulated by the [International Telecommunication Union (ITU)](https://www.itu.int/en/Pages/default.aspx). Because there is a limited pool of available PLMN IDs, obtaining one through the ITU can be challenging, particularly for private network operators.

## Options for Private Networks

There are several approaches available for private networks to acquire a PLMN ID:

1. **Using the Reserved PLMN ID:**  
   Private networks can take advantage of reserved network identifiers. A widely supported option is using the Mobile Country Code (MCC) **999**, which is recognized worldwide for private cellular networks. Examples include identifiers such as **999-01** or **999-123**. This option is ideal when a globally unique identifier is not strictly required for your network.

2. **National Authority Assignments:**  
   In some countries, the assignment of private network identifiers is managed by a national authority. If your country follows such a process, you will need to obtain your PLMN ID through the designated national channels. It is important to verify with your local regulatory body to understand the specific requirements and procedures.

3. **Obtaining a Shared PLMN ID:**  
   If your private network requires a globally unique identifier but the ITU process or national assignment is not a practical option, the Alliance for Private Networks offers a [Network Identifier Program](https://www.mfa-tech.org/network-identifier-program/#:~:text=The%20PLMN%20ID%20identifies%20a,in%20any%20available%20spectrum%20today) that provides PLMN IDs for private networks.
