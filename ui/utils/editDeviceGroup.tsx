interface DeviceGroupArgs {
  name: string;
  ueIpPool: string;
  dns: string;
  mtu: number;
  MBRUpstreamBps: number;
  MBRDownstreamBps: number;
}

const getDeviceGroup = async (deviceGroupName: string) => {
  try {
    const response = await fetch(`/api/device-group/${deviceGroupName}`, {
      method: "GET",
    });
    if (!response.ok)
      throw new Error(
        `Failed to fetch device group. Status: ${response.status}`,
      );
    const deviceGroup = await response.json();
    return deviceGroup;
  } catch (error) {
    console.error(error);
  }
};

export const editDeviceGroup = async ({
  name,
  ueIpPool,
  dns,
  mtu,
  MBRUpstreamBps,
  MBRDownstreamBps,
}: DeviceGroupArgs) => {
  try {
    const currentConfig = await getDeviceGroup(name)
    var imsis = currentConfig["imsis"]

    const deviceGroupData = {
      "site-info": "demo",
      "imsis": imsis,
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

    const response = await fetch(`/api/device-group/${name}`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(deviceGroupData),
    });

    if (!response.ok) {
      throw new Error(
        `Error updating device group. Error code: ${response.status}`,
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
