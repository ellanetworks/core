import { NetworkSlice } from "@/components/types";
import { apiGetProfile, apiGetAllProfiles } from "@/utils/callProfileApi";

export const getProfilesFromNetworkSlice = async (slice?: NetworkSlice) => {
  if (!slice || !slice["site-device-group"]) {
    return [];
  }

  const allProfiles = await Promise.all(
    slice["site-device-group"].map(async (name: string) =>
      await getProfile(name),
    ),
  );

  return allProfiles.filter((item) => item !== undefined);
}

export const getProfiles = async () => {
  try {
    const response = await apiGetAllProfiles();
    if (!response.ok)
      throw new Error(
        `Failed to fetch device group. Status: ${response.status}`,
      );
    const deviceGroups = await response.json();

    const deviceGroupsDetails = await Promise.all(
      deviceGroups.map(async (name: string) =>
        await getProfile(name),
      ),
    );

    return deviceGroupsDetails.filter((item) => item !== undefined);

  } catch (error) {
    console.error(error);
  }
};

export const getProfile = async (deviceGroupName: string) => {
  try {
    const response = await apiGetProfile(deviceGroupName);
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


