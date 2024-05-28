interface GnbItem {
  name: string;
  tac: number;
}

interface CreateNetworkSliceArgs {
  name: string;
  mcc: string;
  mnc: string;
  upfName: string;
  upfPort: string;
  gnbList: GnbItem[];
}

export const createNetworkSlice = async ({
  name,
  mcc,
  mnc,
  upfName,
  upfPort,
  gnbList,
}: CreateNetworkSliceArgs) => {
  const deviceGroupName = `${name}-default`;
  const sliceData = {
    "slice-id": {
      sst: "1",
      sd: "010203",
    },
    "site-device-group": [deviceGroupName],
    "site-info": {
      "site-name": "demo",
      plmn: {
        mcc,
        mnc,
      },
      gNodeBs: gnbList,
      upf: {
        "upf-name": upfName,
        "upf-port": upfPort,
      },
    },
  };

  const deviceGroupData = {
    "site-info": "demo",
    "ip-domain-name": "pool1",
    "ip-domain-expanded": {
      dnn: "internet",
      "ue-ip-pool": "172.250.1.0/16",
      "dns-primary": "8.8.8.8",
      mtu: 1460,
      "ue-dnn-qos": {
        "dnn-mbr-uplink": 20 * 1000000,
        "dnn-mbr-downlink": 200 * 1000000,
        "bitrate-unit": "bps",
        "traffic-class": {
          name: "platinum",
          arp: 6,
          pdb: 300,
          pelr: 6,
          qci: 8,
        },
      },
    },
  };

  try {
    const checkResponse = await fetch(`/api/network-slice/${name}`, {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
      },
    });

    if (checkResponse.ok) {
      throw new Error("Network slice already exists");
    }

    const networksliceResponse = await fetch(`/api/network-slice/${name}`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(sliceData),
    });

    if (!networksliceResponse.ok) {
      const result = await networksliceResponse.json();
      if (result.error) {
        throw new Error(result.error);
      }
      debugger;
      throw new Error(
        `Error creating network. Error code: ${networksliceResponse.status}`,
      );
    }

    const devicegroupResponse = await fetch(
      `/api/device-group/${deviceGroupName}`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(deviceGroupData),
      },
    );

    if (!devicegroupResponse.ok) {
      throw new Error(
        `Error creating device group. Error code: ${devicegroupResponse.status}`,
      );
    }

    return networksliceResponse.json();
  } catch (error: unknown) {
    console.error(error);
    const details =
      error instanceof Error
        ? error.message
        : "Failed to configure the network.";
    throw new Error(details);
  }
};
