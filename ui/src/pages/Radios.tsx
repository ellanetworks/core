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
    <Box
      sx={{
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        pt: 6,
        flex: 1,
        minHeight: 0,
      }}
    >
      <Box
        sx={{
          width: "100%",
          maxWidth: MAX_WIDTH,
          px: PAGE_PAD,
          flexShrink: 0,
        }}
      >
        <Tabs
          value={activeTab}
          onChange={handleTabChange}
          aria-label="Radios tabs"
          sx={{ borderBottom: 1, borderColor: "divider", mt: 2 }}
        >
          <Tab value="list" label="Radios" />
          <Tab value="events" label="Events" />
        </Tabs>
      </Box>

      <Outlet context={{ gridTheme }} />
    </Box>
  );
}
