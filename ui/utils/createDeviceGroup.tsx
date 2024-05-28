interface DeviceGroupArgs {
  name: string;
  ueIpPool: string;
  dns: string;
  mtu: number;
  MBRUpstreamBps: number;
  MBRDownstreamBps: number;
  networkSliceName: string;
}

export const createDeviceGroup = async ({
  name,
  ueIpPool,
  dns,
  mtu,
  MBRUpstreamBps,
  MBRDownstreamBps,
  networkSliceName,
}: DeviceGroupArgs) => {
  const deviceGroupData = {
    "site-info": "demo",
    "ip-domain-name": "pool1",
    "ip-domain-expanded": {
      dnn: "internet",
      "ue-ip-pool": ueIpPool,
      "dns-primary": dns,
      mtu: mtu,
      "ue-dnn-qos": {
        "dnn-mbr-uplink": MBRUpstreamBps,
        "dnn-mbr-downlink": MBRDownstreamBps,
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
    const checkResponse = await fetch(`/api/device-group/${name}`, {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
      },
    });

    if (checkResponse.ok) {
      throw new Error("Device group already exists");
    }

    const response = await fetch(`/api/device-group/${name}`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(deviceGroupData),
    });

    if (!response.ok) {
      throw new Error(
        `Error creating device group. Error code: ${response.status}`,
      );
    }

    const existingSliceResponse = await fetch(
      `/api/network-slice/${networkSliceName}`,
      {
        method: "GET",
        headers: {
          "Content-Type": "application/json",
        },
      },
    );

    const existingSliceData = await existingSliceResponse.json();

    if (!existingSliceData["site-device-group"]) {
      existingSliceData["site-device-group"] = [];
    }

    existingSliceData["site-device-group"].push(name);

    const updateSliceResponse = await fetch(
      `/api/network-slice/${networkSliceName}`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(existingSliceData),
      },
    );

    if (!updateSliceResponse.ok) {
      throw new Error(
        `Error updating network slice. Error code: ${updateSliceResponse.status}`,
      );
    }

    return true;
  } catch (error: unknown) {
    console.error(error);
    const details =
      error instanceof Error
        ? error.message
        : "Failed to configure the network.";
    throw new Error(details);
  }
};
