---
description: Explanation of obtaining a PLMN ID for private networks.
---

# Obtaining a PLMN ID for a Private Network

## Introduction to PLMN IDs

A Public Land Mobile Network (PLMN) ID is a globally unique identifier that allows mobile devices to recognize and connect to mobile networks. In 5G private networks, having a correct PLMN ID is critical for ensuring proper network identification and operation.

A PLMN ID consists of two parts:

- **Mobile Country Code (MCC):** A three-digit code that identifies the country.
- **Mobile Network Code (MNC):** A two- or three-digit code that identifies the network operator within that country.

Each subscriber in a mobile network has a unique identifier called the International Mobile Subscriber Identity (IMSI). The IMSI is composed of the PLMN ID and a subscriber-specific identifier.

## How PLMN IDs Are Assigned

PLMN IDs are regulated by the [International Telecommunication Union (ITU)](https://www.itu.int/en/Pages/default.aspx). Because there is a limited pool of available PLMN IDs, obtaining one through the ITU can be challenging, particularly for private network operators.

## Options for Private Networks

There are several approaches available for private networks to acquire a PLMN ID:

1. **Using the Reserved PLMN ID:** The Mobile Country Code **999** is globally reserved for private networks. Network operators can choose an MNC (e.g., **01**, **123**) to create a PLMN ID such as **999-01** or **999-123**. This method is ideal if your network does not require a globally unique identifier.

2. **National Authority Assignments:** In some countries, a designated national regulatory body assigns PLMN IDs to private networks. If your country uses this approach, you must apply for a PLMN ID through the appropriate national channels.

3. **Alliance for Private Networks:** The Alliance for Private Networks offers a [Network Identifier Program](https://www.mfa-tech.org/network-identifier-program/#:~:text=The%20PLMN%20ID%20identifies%20a,in%20any%20available%20spectrum%20today) that provides the temporary use of PLMN IDs for private networks. This program allows private network operators to obtain a "slice" of a PLMN ID. They ensure that subscribers from different private networks are uniquely identified and do not overlap.

4. **ITU Assignments:** For private networks that require a globally unique identifier, a PLMN ID can be obtained via the ITU process. This typically involves coordination with your local regulatory authority and compliance with international standards.

## Note on Configuration

Once you have obtained a PLMN ID for your private network, you can configure it in Ella Core via the [Operator API](../reference/api/operator.md) or the user interface. The PLMN ID can only be updated when no subscribers are created.
