export interface UpfItem {
  hostname: string;
  port: string;
}

export const getUpfList = async (): Promise<UpfItem[]> => {
  return [
    {"hostname": "0.0.0.0", "port": "8806"},
  ]
};
