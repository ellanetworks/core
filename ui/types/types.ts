export enum RoleID {
  Admin = 1,
  ReadOnly = 2,
  NetworkManager = 3,
}

export type Profile = {
  name: string;
  ipPool: string;
  dns: string;
  mtu: number;
  bitrateUp: string;
  bitrateDown: string;
  fiveQi: number;
  priorityLevel: number;
};
