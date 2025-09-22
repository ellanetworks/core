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

export type AuditLogRetentionPolicy = {
  days: number;
};

export type SubscriberLogRetentionPolicy = {
  days: number;
};

export type APIToken = {
  id: number;
  name: string;
  expires_at: string | null;
};
