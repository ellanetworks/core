function isValidProfileName(name: string): boolean {
  return /^[a-zA-Z0-9-_]+$/.test(name);
}

export const apiGetAllProfiles = async () => {
  try {
    const response = await fetch(`/api/v1/profiles/`, {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
      },
    });
    return response
  } catch (error) {
    console.error(error);
    throw error;
  }
};

export const apiGetProfile = async (name: string) => {
  if (!isValidProfileName(name)) {
    throw new Error(`Error getting device group: Invalid name provided.`);
  }
  try {
    const response = await fetch(`/api/v1/profiles/${name}`, {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
      },
    });
    return response
  } catch (error) {
    console.error(error);
    throw error;
  }
};

export const apiPostProfile = async (name: string, deviceGroupData: any) => {
  if (!isValidProfileName(name)) {
    throw new Error(`Error updating device group: Invalid name provided.`);
  }
  try {
    const response = await fetch(`/api/v1/profiles`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(deviceGroupData),
    });
    return response
  } catch (error) {
    console.error(error);
    throw error;
  }
};

export const apiPutProfile = async (name: string, deviceGroupData: any) => {
  if (!isValidProfileName(name)) {
    throw new Error(`Error updating device group: Invalid name provided.`);
  }
  try {
    const response = await fetch(`/api/v1/profiles/${name}`, {
      method: "PUT",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(deviceGroupData),
    });
    return response
  } catch (error) {
    console.error(error);
    throw error;
  }
}

export const apiDeleteProfile = async (name: string) => {
  if (!isValidProfileName(name)) {
    throw new Error(`Error deleting device group: Invalid name provided.`);
  }
  try {
    const response = await fetch(`/api/v1/profiles/${name}`, {
      method: "DELETE",
      headers: {
        "Content-Type": "application/json",
      },
    });
    return response
  } catch (error) {
    console.error(error);
    throw error;
  }
};
