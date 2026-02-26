import { apiFetch, apiFetchVoid } from "@/queries/utils";

export type FlowAccountingInfo = {
  enabled: boolean;
};

export const getFlowAccountingInfo = async (
  authToken: string,
): Promise<FlowAccountingInfo> => {
  return apiFetch<FlowAccountingInfo>(`/api/v1/networking/flow-accounting`, {
    authToken,
  });
};

export const updateFlowAccountingInfo = async (
  authToken: string,
  enabled: boolean,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/networking/flow-accounting`, {
    method: "PUT",
    authToken,
    body: { enabled },
  });
};
