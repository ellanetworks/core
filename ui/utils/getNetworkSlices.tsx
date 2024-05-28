import { NetworkSlice } from "@/components/types";

export const getNetworkSlices = async (): Promise<NetworkSlice[]> => {
  try {
    const response = await fetch(`/api/network-slice`, {
      method: "GET",
    });
    if (!response.ok) {
      throw new Error("Failed to fetch network slices");
    }
    const sliceNames = await response.json();

    const sliceDetailsPromises = sliceNames.map(async (sliceName: string) => {
      const detailResponse = await fetch(`/api/network-slice/${sliceName}`, {
        method: "GET",
      });
      if (!detailResponse.ok) {
        throw new Error(`Failed to fetch details for slice: ${sliceName}`);
      }
      return detailResponse.json();
    });

    return await Promise.all(sliceDetailsPromises);
  } catch (error) {
    console.error(error);
    throw error;
  }
};
