export const deleteNetworkSlice = async (sliceName: string) => {
  try {
    const response = await fetch(`/api/network-slice/${sliceName}`, {
      method: "DELETE",
    });

    if (!response.ok) {
      throw new Error("Failed to delete network slice");
    }
    return true;
  } catch (error) {
    console.error(error);
    return false;
  }
};
