import { NetworkSlice } from "@/components/types";
import { apiGetProfile, apiGetAllProfiles } from "@/utils/callProfileApi";

export const getProfilesFromNetworkSlice = async (slice?: NetworkSlice) => {
  if (!slice || !slice["profiles"]) {
    return [];
  }

  const allProfiles = await Promise.all(
    slice["profiles"].map(async (name: string) =>
      await apiGetProfile(name),
    ),
  );

  return allProfiles.filter((item) => item !== undefined);
}

export const getProfiles = async () => {
  try {
    const deviceGroups = await apiGetAllProfiles();
    const deviceGroupsDetails = await Promise.all(
      deviceGroups.map(async (name: string) =>
        await apiGetProfile(name),
      ),
    );

    return deviceGroupsDetails.filter((item) => item !== undefined);

  } catch (error) {
    console.error(error);
  }
};
