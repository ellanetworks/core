import { HTTPStatus } from "@/queries/utils";

export const listProfiles = async () => {
  const response = await fetch(`/api/v1/profiles`, {
    method: "GET",
    headers: {
      "Content-Type": "application/json",
    },
  });
  try {
    const respData = await response.json();
    if (!response.ok) {
      throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${respData.error}`)
    }
    return respData.result
  } catch (error) {
    throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`)
  }
};

export const createProfile = async (name: string, ipPool: string, dns: string, mtu: number, bitrateUplink: string, bitrateDownlink: string, var5qi: number, priorityLevel: number) => {
  const profileData = {
    "name": name,
    "ue-ip-pool": ipPool,
    "dns": dns,
    "mtu": mtu,
    "bitrate-uplink": bitrateUplink,
    "bitrate-downlink": bitrateDownlink,
    "var5qi": var5qi,
    "priority-level": priorityLevel
  }

  const response = await fetch(`/api/v1/profiles`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(profileData),
  });
  try {
    const respData = await response.json();
    if (!response.ok) {
      throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${respData.error}`)
    }
    return respData.result
  } catch (error) {
    throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`)
  }
};

export const updateProfile = async (name: string, ipPool: string, dns: string, mtu: number, bitrateUplink: string, bitrateDownlink: string, var5qi: number, priorityLevel: number) => {
  const profileData = {
    "name": name,
    "ue-ip-pool": ipPool,
    "dns": dns,
    "mtu": mtu,
    "bitrate-uplink": bitrateUplink,
    "bitrate-downlink": bitrateDownlink,
    "var5qi": var5qi,
    "priority-level": priorityLevel
  }

  const response = await fetch(`/api/v1/profiles/${name}`, {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(profileData),
  });
  try {
    const respData = await response.json();
    if (!response.ok) {
      throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${respData.error}`)
    }
    return respData.result
  } catch (error) {
    throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`)
  }
}

export const deleteProfile = async (name: string) => {
  const response = await fetch(`/api/v1/profiles/${name}`, {
    method: "DELETE",
    headers: {
      "Content-Type": "application/json",
    },
  });
  try {
    const respData = await response.json();
    if (!response.ok) {
      throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${respData.error}`)
    }
    return respData.result
  } catch (error) {
    throw new Error(`${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`)
  }
}