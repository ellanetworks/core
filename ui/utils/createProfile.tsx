import { apiGetNetworkSlice, apiCreateNetworkSlice } from "@/utils/callNetworkSliceApi";
import { apiGetProfile, apiPostProfile } from "@/utils/callProfileApi";

interface ProfileArgs {
  name: string;
  ueIpPool: string;
  dns: string;
  mtu: number;
  MBRUpstreamBps: number;
  MBRDownstreamBps: number;
  networkSliceName: string;
}

export const createProfile = async ({
  name,
  ueIpPool,
  dns,
  mtu,
  MBRUpstreamBps,
  MBRDownstreamBps,
  networkSliceName,
}: ProfileArgs) => {
  const deviceGroupData = {
    "site-info": "demo",
    "ip-domain-name": "pool1",
    "ip-domain-expanded": {
      dnn: "internet",
      "ue-ip-pool": ueIpPool,
      "dns-primary": dns,
      mtu: mtu,
      "ue-dnn-qos": {
        "bitrate-uplink": MBRUpstreamBps,
        "bitrate-downlink": MBRDownstreamBps,
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
    const getProfileResponse = await apiGetProfile(name);
    if (getProfileResponse.ok) {
      throw new Error("Device group already exists");
    }

    const updateProfileResponse = await apiPostProfile(name, deviceGroupData);
    if (!updateProfileResponse.ok) {
      throw new Error(
        `Error creating device group. Error code: ${updateProfileResponse.status}`,
      );
    }

    const existingSliceResponse = await apiGetNetworkSlice(networkSliceName);
    var existingSliceData = await existingSliceResponse.json();

    if (!existingSliceData["site-device-group"]) {
      existingSliceData["site-device-group"] = [];
    }
    existingSliceData["site-device-group"].push(name);

    const updateSliceResponse = await apiCreateNetworkSlice(networkSliceName, existingSliceData);
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
