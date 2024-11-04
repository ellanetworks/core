
export const apiListRadios = async () => {
  try {
    var response = await fetch(`/api/v1/inventory/radios`, {
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

export const apiGetRadio = async (id: string) => {
  try {
    const response = await fetch(`/api/v1/inventory/radios/${id}`, {
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

export const apiPostRadio = async (radioData: any) => {
  try {
    const response = await fetch(`/api/v1/inventory/radios`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(radioData),
    });
    return response
  } catch (error) {
    console.error(error);
    throw error;
  }
};

export const apiDeleteRadio = async (id: string) => {
  try {
    const response = await fetch(`/api/v1/inventory/radios/${id}`, {
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

export const deleteRadio = async (id: string) => {
  try {
    const deleteRadioResponse = await apiDeleteRadio(id);
    if (!deleteRadioResponse.ok) {
      throw new Error("Failed to delete radio");
    }

    return true;
  } catch (error) {
    console.error(error);
  }
};

export const listRadios = async () => {
  try {
    const response = await apiListRadios();
    if (!response.ok) {
      throw new Error(
        `Failed to fetch radios. Status: ${response.status}`,
      );
    }

    var jsonResponse = await response.json();
    // Response format:
    // {"result":[1,2,3]}
    if (!jsonResponse["result"]) {
      throw new Error("Failed to fetch radios.");
    }

    // Fetch all radios
    // Save as an array of Radio objects
    var allRadios = await Promise.all(
      jsonResponse["result"].map(async (id: string) => {
        return await getRadio(id);
      }),
    );

    return allRadios.filter((item) => item !== undefined);
  } catch (error) {
    console.error(error);
    throw error;
  }
};

const getRadio = async (id: string) => {
  try {
    const response = await apiGetRadio(id);
    if (!response.ok)
      throw new Error(
        `Failed to fetch radio. Status: ${response.status}`,
      );
    const responseJson = await response.json();
    if (!responseJson["result"]) {
      throw new Error("Failed to fetch radio.");
    }

    return responseJson["result"];
  } catch (error) {
    console.error(error);
  }
};


interface CreateRadioArgs {
  name: string;
  tac: string;
  network_slice_id: number;
}

export const createRadio = async ({
  name,
  tac,
  network_slice_id,
}: CreateRadioArgs) => {
  const radioData = {
    name: name,
    tac: tac,
    network_slice_id: network_slice_id,
  };

  try {

    const updateRadioResponse = await apiPostRadio(radioData);
    if (!updateRadioResponse.ok) {
      throw new Error(
        `Error creating radio. Error code: ${updateRadioResponse.status}`,
      );
    }

    return updateRadioResponse.json();
  } catch (error) {
    console.error(error);
    const details =
      error instanceof Error
        ? error.message
        : "Failed to create the radio.";
    throw new Error(details);
  }
};
