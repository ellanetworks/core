export interface GnbItem {
  name: string;
  tac: number;
}

export const getGnbList = async (): Promise<GnbItem[]> => {
  try {
    const response = await fetch("/api/v1/inventory/radios", {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
      },
    });
    if (!response.ok) {
      throw new Error("Failed to fetch GNB list");
    }
    const radioList = await response.json();
    return radioList.map((radio: GnbItem) => ({
      ...radio,
      tac: Number(radio.tac),
    }));
  } catch (error) {
    console.error(error);
    throw error;
  }
};
