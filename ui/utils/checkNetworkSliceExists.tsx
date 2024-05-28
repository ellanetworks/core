export const checkNetworkSliceExists = async () => {
  try {
    const response = await fetch(`/api/network-slice`, {
      method: "GET",
    });

    return response.status === 200;
  } catch (error) {
    return false;
  }
};
