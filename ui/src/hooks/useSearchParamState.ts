// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { useCallback } from "react";
import { useSearchParams } from "react-router-dom";

export function useSearchParamState(
  key: string,
): [string, (next: string) => void] {
  const [searchParams, setSearchParams] = useSearchParams();

  const setValue = useCallback(
    (next: string) => {
      setSearchParams(
        (prev) => {
          const params = new URLSearchParams(prev);

          if (next) {
            params.set(key, next);
          } else {
            params.delete(key);
          }

          return params;
        },
        { replace: true },
      );
    },
    [key, setSearchParams],
  );

  return [searchParams.get(key) ?? "", setValue];
}
