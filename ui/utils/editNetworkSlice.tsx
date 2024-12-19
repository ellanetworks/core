import { apiGetNetworkSlice, apiCreateNetworkSlice } from "@/utils/callNetworkSliceApi";

interface GnbItem {
  name: string;
  tac: number;
}

interface EditNetworkSliceArgs {
  name: string;
  mcc: string;
  mnc: string;
  upfName: string;
  upfPort: number;
  radioList: GnbItem[];
}

export const editNetworkSlice = async ({
  name,
  mcc,
  mnc,
  upfName,
  upfPort,
  radioList,
}: EditNetworkSliceArgs) => {

  try {
    const sliceData = await apiGetNetworkSlice(name);
    sliceData.mcc = mcc
    sliceData.mnc = mnc
    sliceData["gNodeBs"] = radioList
    sliceData["upf"]["name"] = upfName
    sliceData["upf"]["port"] = upfPort
    const updateSliceResponse = await apiCreateNetworkSlice(name, sliceData);
    return updateSliceResponse;
  } catch (error: unknown) {
    console.error(error);
    const details =
      error instanceof Error
        ? error.message
        : "Failed to configure the network.";
    throw new Error(details);
  }
};
