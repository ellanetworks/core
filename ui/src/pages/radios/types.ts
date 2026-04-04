import type { Theme } from "@mui/material/styles";
import { useOutletContext } from "react-router-dom";

export interface RadiosTabProps {
  gridTheme: Theme;
}

export function useRadiosContext() {
  return useOutletContext<RadiosTabProps>();
}
