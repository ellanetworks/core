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
    const getSliceResponse = await apiGetNetworkSlice(name);

    if (!getSliceResponse.ok) {
      const result = await getSliceResponse.json();
      if (result.error) {
        throw new Error(result.error);
      }
      throw new Error("Error editing Network Slice " + name);
    }

    var sliceData = await getSliceResponse.json();
    sliceData.mcc = mcc
    sliceData.mnc = mnc
    sliceData["gNodeBs"] = radioList
    sliceData["upf"]["name"] = upfName
    sliceData["upf"]["port"] = upfPort

    const updateSliceResponse = await apiCreateNetworkSlice(name, sliceData);

    if (!updateSliceResponse.ok) {
      const result = await updateSliceResponse.json();
      if (result.error) {
        throw new Error(result.error);
      }
      debugger;
      throw new Error(
        `Error editing network slice. Error code: ${updateSliceResponse.status}`,
      );
    }

    return updateSliceResponse.json();
  } catch (error: unknown) {
    console.error(error);
    const details =
      error instanceof Error
        ? error.message
        : "Failed to configure the network.";
    throw new Error(details);
  }
};
