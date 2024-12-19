export interface GnbItem {
  name: string;
  tac: number;
}

export const getGnbList = async (): Promise<GnbItem[]> => {
  const response = await fetch("/api/v1/radios", {
    method: "GET",
    headers: {
      "Content-Type": "application/json",
    },
  });
  const response_json = await response.json();
  const radioList = response_json.result;
  return radioList.map((radio: GnbItem) => ({
    ...radio,
    tac: Number(radio.tac),
  }));
};
