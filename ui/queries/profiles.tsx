import { HTTPStatus } from "@/queries/utils";
import { Profile } from "@/types/types";

export const listProfiles = async (authToken: string): Promise<Profile[]> => {
  const response = await fetch(`/api/v1/profiles`, {
    method: "GET",
    headers: {
      "Content-Type": "application/json",
      Authorization: "Bearer " + authToken,
    },
  });

  let respData;
  try {
    respData = await response.json();
  } catch {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`,
    );
  }

  if (!response.ok) {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${respData?.error || "Unknown error"}`,
    );
  }

  const transformed: Profile[] = respData.result.map((p: any) => ({
    name: p.name,
    ipPool: p["ue-ip-pool"],
    dns: p.dns,
    mtu: p.mtu,
    bitrateUp: p["bitrate-uplink"],
    bitrateDown: p["bitrate-downlink"],
    fiveQi: p["var5qi"],
    priorityLevel: p["priority-level"],
  }));

  return transformed;
};

export const createProfile = async (
  authToken: string,
  name: string,
  ipPool: string,
  dns: string,
  mtu: number,
  bitrateUplink: string,
  bitrateDownlink: string,
  var5qi: number,
  priorityLevel: number,
) => {
  const profileData = {
    name: name,
    "ue-ip-pool": ipPool,
    dns: dns,
    mtu: mtu,
    "bitrate-uplink": bitrateUplink,
    "bitrate-downlink": bitrateDownlink,
    var5qi: var5qi,
    "priority-level": priorityLevel,
  };

  const response = await fetch(`/api/v1/profiles`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: "Bearer " + authToken,
    },
    body: JSON.stringify(profileData),
  });
  let respData;
  try {
    respData = await response.json();
  } catch {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`,
    );
  }

  if (!response.ok) {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${respData?.error || "Unknown error"}`,
    );
  }

  return respData.result;
};

export const updateProfile = async (
  authToken: string,
  name: string,
  ipPool: string,
  dns: string,
  mtu: number,
  bitrateUplink: string,
  bitrateDownlink: string,
  var5qi: number,
  priorityLevel: number,
) => {
  const profileData = {
    name: name,
    "ue-ip-pool": ipPool,
    dns: dns,
    mtu: mtu,
    "bitrate-uplink": bitrateUplink,
    "bitrate-downlink": bitrateDownlink,
    var5qi: var5qi,
    "priority-level": priorityLevel,
  };

  const response = await fetch(`/api/v1/profiles/${name}`, {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
      Authorization: "Bearer " + authToken,
    },
    body: JSON.stringify(profileData),
  });
  let respData;
  try {
    respData = await response.json();
  } catch {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`,
    );
  }

  if (!response.ok) {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${respData?.error || "Unknown error"}`,
    );
  }

  return respData.result;
};

export const deleteProfile = async (authToken: string, name: string) => {
  const response = await fetch(`/api/v1/profiles/${name}`, {
    method: "DELETE",
    headers: {
      "Content-Type": "application/json",
      Authorization: "Bearer " + authToken,
    },
  });
  let respData;
  try {
    respData = await response.json();
  } catch {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`,
    );
  }

  if (!response.ok) {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${respData?.error || "Unknown error"}`,
    );
  }

  return respData.result;
};
