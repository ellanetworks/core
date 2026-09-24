// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { useCallback } from "react";
import { useSnackbar } from "@/contexts/SnackbarContext";

export const useCopyToClipboard = () => {
  const { showSnackbar } = useSnackbar();

  return useCallback(
    async (text: string, label?: string): Promise<boolean> => {
      if (!navigator.clipboard) {
        showSnackbar(
          "Clipboard API not available. Please use HTTPS or try a different browser.",
          "error",
        );
        return false;
      }
      try {
        await navigator.clipboard.writeText(text);
        showSnackbar(
          label ? `${label} copied to clipboard.` : "Copied to clipboard.",
          "success",
        );
        return true;
      } catch {
        showSnackbar(
          label ? `Failed to copy ${label}.` : "Failed to copy to clipboard.",
          "error",
        );
        return false;
      }
    },
    [showSnackbar],
  );
};
