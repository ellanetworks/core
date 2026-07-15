// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { HTTPStatus } from "@/queries/utils";

// Cannot use apiFetch because the response is text, not JSON.
export const getMetrics = async (): Promise<string> => {
  const response = await fetch(`/api/v1/metrics`, {
    method: "GET",
  });

  if (!response.ok) {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`,
    );
  }

  return response.text();
};
