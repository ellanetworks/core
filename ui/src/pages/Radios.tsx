import { useMemo } from "react";
import type { SyntheticEvent } from "react";
import { Box, Tabs, Tab } from "@mui/material";
import { useTheme, createTheme } from "@mui/material/styles";
import { Outlet, useLocation, useNavigate } from "react-router-dom";
import { MAX_WIDTH, PAGE_PADDING_X as PAGE_PAD } from "@/utils/layout";

export default function RadiosPage() {
  const location = useLocation();
  const navigate = useNavigate();

  const match = location.pathname.match(/^\/radios\/([^/]+)/);
  const activeTab = match?.[1] === "events" ? "events" : "list";

  const theme = useTheme();
  const gridTheme = useMemo(
    () =>
      createTheme(theme, {
        palette: { DataGrid: { headerBg: theme.palette.backgroundSubtle } },
      }),
    [theme],
  );

  const handleTabChange = (_: SyntheticEvent, newValue: string) => {
    navigate(newValue === "list" ? "/radios" : `/radios/${newValue}`);
  };

  return (
    <Box sx={{ pt: 6, pb: 4, maxWidth: MAX_WIDTH, mx: "auto", px: PAGE_PAD }}>
      <Tabs
        value={activeTab}
        onChange={handleTabChange}
        aria-label="Radios tabs"
        sx={{ borderBottom: 1, borderColor: "divider", mt: 2 }}
      >
        <Tab value="list" label="Radios" />
        <Tab value="events" label="Events" />
      </Tabs>

      <Outlet context={{ gridTheme }} />
    </Box>
  );
}
