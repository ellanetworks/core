export enum RoleID {
  Admin = 1,
  ReadOnly = 2,
  NetworkManager = 3,
}

export type DataNetworkStatus = {
  sessions: number;
};

export type DataNetwork = {
  name: string;
  ipPool: string;
  dns: string;
  mtu: number;
  status?: DataNetworkStatus;
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

type SubscriberSession = {
  ipAddress: string;
};

export type SubscriberStatus = {
  registered?: boolean;
  sessions?: Array<SubscriberSession>;
};

export type Subscriber = {
  imsi: string;
  opc: string;
  sequenceNumber: string;
  key: string;
  policyName: string;
  status: SubscriberStatus;
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
