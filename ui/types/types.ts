export enum RoleID {
  Admin = 1,
  ReadOnly = 2,
  NetworkManager = 3,
}

export type DataNetwork = {
  name: string;
  ipPool: string;
  dns: string;
  mtu: number;
};

export type Policy = {
  name: string;
  bitrateUp: string;
  bitrateDown: string;
  fiveQi: number;
  priorityLevel: number;
  dataNetworkName: string;
};

export type Route = {
  id: string;
  destination: string;
  gateway: string;
  interface: string;
  metric: number;
};

export type Subscriber = {
  imsi: string;
  ipAddress: string;
  opc: string;
  sequenceNumber: string;
  key: string;
  policyName: string;
};

export type User = {
  email: string;
  roleID: RoleID;
};

export const roleIDToLabel = (role: RoleID): string => {
  switch (role) {
    case RoleID.Admin:
      return "Admin";
    case RoleID.NetworkManager:
      return "Network Manager";
    case RoleID.ReadOnly:
      return "Read Only";
    default:
      return "Unknown";
  }
};
