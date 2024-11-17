
export const apiListUPFs = async () => {
  try {
    var response = await fetch(`/api/v1/inventory/upfs`, {
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

export const apiGetUPF = async (id: string) => {
  try {
    const response = await fetch(`/api/v1/inventory/upfs/${id}`, {
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

export const apiPostUPF = async (upfData: any) => {
  try {
    const response = await fetch(`/api/v1/inventory/upfs`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(upfData),
    });
    return response
  } catch (error) {
    console.error(error);
    throw error;
  }
};

export const apiDeleteUPF = async (id: string) => {
  try {
    const response = await fetch(`/api/v1/inventory/upfs/${id}`, {
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

export const deleteUPF = async (id: string) => {
  try {
    const deleteUPFResponse = await apiDeleteUPF(id);
    if (!deleteUPFResponse.ok) {
      throw new Error("Failed to delete upf");
    }

    return true;
  } catch (error) {
    console.error(error);
  }
};

export const listUPFs = async () => {
  try {
    const response = await apiListUPFs();
    if (!response.ok) {
      throw new Error(
        `Failed to fetch upfs. Status: ${response.status}`,
      );
    }

    var jsonResponse = await response.json();
    // Response format:
    // {"result":[1,2,3]}
    if (!jsonResponse["result"]) {
      throw new Error("Failed to fetch upfs.");
    }

    // Fetch all upfs
    // Save as an array of UPF objects
    var allUPFs = await Promise.all(
      jsonResponse["result"].map(async (id: string) => {
        return await getUPF(id);
      }),
    );

    return allUPFs.filter((item) => item !== undefined);
  } catch (error) {
    console.error(error);
    throw error;
  }
};

const getUPF = async (id: string) => {
  try {
    const response = await apiGetUPF(id);
    if (!response.ok)
      throw new Error(
        `Failed to fetch upf. Status: ${response.status}`,
      );
    const responseJson = await response.json();
    if (!responseJson["result"]) {
      throw new Error("Failed to fetch upf.");
    }

    return responseJson["result"];
  } catch (error) {
    console.error(error);
  }
};


interface CreateUPFArgs {
  name: string;
  network_slice_id: number;
}

export const createUPF = async ({
  name,
  network_slice_id,
}: CreateUPFArgs) => {
  const upfData = {
    name: name,
    network_slice_id: network_slice_id,
  };

  try {

    const updateUPFResponse = await apiPostUPF(upfData);
    if (!updateUPFResponse.ok) {
      throw new Error(
        `Error creating upf. Error code: ${updateUPFResponse.status}`,
      );
    }

    return updateUPFResponse.json();
  } catch (error) {
    console.error(error);
    const details =
      error instanceof Error
        ? error.message
        : "Failed to create the upf.";
    throw new Error(details);
  }
};
