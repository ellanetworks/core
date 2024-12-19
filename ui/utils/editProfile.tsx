import { apiGetProfile, apiPostProfile } from "@/utils/callProfileApi";

interface ProfileArgs {
  name: string;
  ueIpPool: string;
  dns: string;
  mtu: number;
  MBRUpstreamBps: number;
  MBRDownstreamBps: number;
}

export const editProfile = async ({
  name,
  ueIpPool,
  dns,
  mtu,
  MBRUpstreamBps,
  MBRDownstreamBps,
}: ProfileArgs) => {
  try {
    const currentConfig = await apiGetProfile(name)
    var imsis = currentConfig["imsis"]

    const deviceGroupData = {
      "imsis": imsis,
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

    await apiPostProfile(name, deviceGroupData);
    return true;
  } catch (error: unknown) {
    console.error(error);
    const details =
      error instanceof Error
        ? error.message
        : "Failed edit device group.";
    throw new Error(details);
  }
};
