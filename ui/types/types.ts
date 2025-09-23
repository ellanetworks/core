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
