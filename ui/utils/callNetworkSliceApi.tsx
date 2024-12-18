function isValidNetworkSliceName(name: string): boolean {
  return /^[a-zA-Z0-9-_]+$/.test(name);
}

export const apiGetAllNetworkSlices = async () => {
  try {
    const networkSlicesResponse = await fetch(`/api/v1/network-slices`, {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
      },
    });
    return networkSlicesResponse
  } catch (error) {
    console.error(error);
    throw error;
  }
};

export const apiGetNetworkSlice = async (name: string) => {
  if (!isValidNetworkSliceName(name)) {
    throw new Error(`Error getting network slice: Invalid name provided: ${name}`);
  }
  try {
    const response = await fetch(`/api/v1/network-slices/${name}`, {
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

export const apiCreateNetworkSlice = async (name: string, sliceData: any) => {
  if (!isValidNetworkSliceName(name)) {
    throw new Error(`Error updating network slice: Invalid name provided.`);
  }
  try {
    const response = await fetch(`/api/v1/network-slices`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(sliceData),
    });
    return response
  } catch (error) {
    console.error(error);
    throw error;
  }
};

export const apiDeleteNetworkSlice = async (name: string) => {
  if (!isValidNetworkSliceName(name)) {
    throw new Error(`Error deleting network slice: Invalid name provided.`);
  }
  try {
    const response = await fetch(`/api/v1/network-slices/${name}`, {
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