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
    name: name,
    dnn: "internet",
    "ue-ip-pool": ueIpPool,
    "dns-primary": dns,
    mtu: mtu,
    "bitrate-uplink": MBRUpstreamBps,
    "bitrate-downlink": MBRDownstreamBps,
    "bitrate-unit": "bps",
    arp: 6,
    pdb: 300,
    pelr: 6,
    qci: 8,
  };

  try {
    const getProfileResponse = await apiGetProfile(name);
    if (getProfileResponse.ok) {
      throw new Error("Device group already exists");
    }

    const updateProfileResponse = await apiPostProfile(name, deviceGroupData);
    if (!updateProfileResponse.ok) {
      throw new Error(
        `Error creating profile. Error code: ${updateProfileResponse.status}`,
      );
    }

    const existingSliceData = await apiGetNetworkSlice(networkSliceName);

    if (!existingSliceData["profiles"]) {
      existingSliceData["profiles"] = [];
    }
    existingSliceData["profiles"].push(name);

    const updateSliceResponse = await apiCreateNetworkSlice(networkSliceName, existingSliceData);

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
