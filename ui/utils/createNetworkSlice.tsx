import { apiCreateNetworkSlice } from "@/utils/callNetworkSliceApi";
import { apiPostProfile } from "@/utils/callProfileApi";

interface GnbItem {
  name: string;
  tac: number;
}

interface CreateNetworkSliceArgs {
  name: string;
  mcc: string;
  mnc: string;
  upfName: string;
  upfPort: number;
  radioList: GnbItem[];
}

export const createNetworkSlice = async ({
  name,
  mcc,
  mnc,
  upfName,
  upfPort,
  radioList,
}: CreateNetworkSliceArgs) => {
  const deviceGroupName = `${name}-default`;
  const sliceData = {
    name: name,
    sst: "1",
    sd: "102030",
    profiles: [deviceGroupName],
    mcc,
    mnc,
    gNodeBs: radioList,
    upf: {
      name: upfName,
      port: upfPort,
    },
  };

  const deviceGroupData = {
    name: deviceGroupName,
    dnn: "internet",
    "ue-ip-pool": "172.250.1.0/16",
    "dns-primary": "8.8.8.8",
    mtu: 1460,
    "bitrate-uplink": 20 * 1000000,
    "bitrate-downlink": 200 * 1000000,
    "bitrate-unit": "bps",
    arp: 6,
    pdb: 300,
    pelr: 6,
    var5qi: 8,
  };

  try {
    const updateNetworkSliceResponse = await apiCreateNetworkSlice(name, sliceData);
    await apiPostProfile(deviceGroupName, deviceGroupData);
    return updateNetworkSliceResponse;
  } catch (error: unknown) {
    console.error(error);
    const details =
      error instanceof Error
        ? error.message
        : "Failed to configure the network.";
    throw new Error(details);
  }
};
