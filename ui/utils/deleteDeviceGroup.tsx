interface DeleteDeviceGroupArgs {
  name: string;
  networkSliceName: string;
}

export const deleteDeviceGroup = async ({
  name,
  networkSliceName,
}: DeleteDeviceGroupArgs) => {
  try {
    const existingSliceResponse = await fetch(
      `/api/network-slice/${networkSliceName}`,
      {
        method: "GET",
        headers: {
          "Content-Type": "application/json",
        },
      },
    );

    if (!existingSliceResponse.ok) {
      throw new Error(
        `Error fetching network slice. Error code: ${existingSliceResponse.status}`,
      );
    }

    const existingSliceData = await existingSliceResponse.json();

    if (existingSliceData["site-device-group"]) {
      const index = existingSliceData["site-device-group"].indexOf(name);
      if (index > -1) {
        existingSliceData["site-device-group"].splice(index, 1);

        const updateSliceResponse = await fetch(
          `/api/network-slice/${networkSliceName}`,
          {
            method: "POST",
            headers: {
              "Content-Type": "application/json",
            },
            body: JSON.stringify(existingSliceData),
          },
        );

        if (!updateSliceResponse.ok) {
          throw new Error(
            `Error updating network slice. Error code: ${updateSliceResponse.status}`,
          );
        }
      }
    }

    const deleteResponse = await fetch(`/api/device-group/${name}`, {
      method: "DELETE",
      headers: {
        "Content-Type": "application/json",
      },
    });

    if (!deleteResponse.ok) {
      throw new Error(
        `Error deleting device group. Error code: ${deleteResponse.status}`,
      );
    }

    return true;
  } catch (error) {
    console.error(error);
    throw new Error("Failed to delete device group.");
  }
};
