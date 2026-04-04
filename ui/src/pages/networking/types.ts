import type { Theme } from "@mui/material/styles";
import type { AlertColor } from "@mui/material";
import { useOutletContext } from "react-router-dom";

export interface NetworkingTabProps {
  accessToken: string | null;
  canEdit: boolean;
  showSnackbar: (message: string, severity: AlertColor) => void;
  gridTheme: Theme;
}

export function useNetworkingContext() {
  return useOutletContext<NetworkingTabProps>();
}
