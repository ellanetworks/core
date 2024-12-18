export type NetworkSlice = {
  name: string;
  sst: string;
  sd: string;
  "profiles"?: string[];
  mcc: string;
  mnc: string;
  gNodeBs?: [{
    name: string;
    tac: number;
  }];
  "upf": {
    name: string;
    port: number;
  };
};
