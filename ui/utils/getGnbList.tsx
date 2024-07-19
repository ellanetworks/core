export interface GnbItem {
  name: string;
  tac: number;
}

export const getGnbList = async (): Promise<GnbItem[]> => {
  return [
    {
      "name": "GNB1",
      "tac": 1
    }
  ]
};
