"use client";

import React from "react";
import { Box, CssBaseline, Toolbar, AppBar, Typography, Drawer, List, ListItem, ListItemButton, ListItemIcon, ListItemText, Divider } from "@mui/material";
import {
  Info as InfoIcon,
  BugReport as BugReportIcon,
  People as PeopleIcon,
  NetworkCheck as NetworkIcon,
} from "@mui/icons-material";
import { usePathname } from "next/navigation";
import { ThemeProvider } from "@mui/material/styles";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import theme from "@/utils/theme";
import "./globals.scss";

const drawerWidth = 250;
const queryClient = new QueryClient();

export default function RootLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();

  return (
    <html lang="en">
      <head>
        <title>Ella</title>
        <link
          rel="shortcut icon"
          href="https://assets.ubuntu.com/v1/49a1a858-favicon-32x32.png"
          type="image/x-icon"
        />
      </head>
      <body>
        <ThemeProvider theme={theme}>
          <CssBaseline />
          <Box sx={{ display: "flex" }}>
            <AppBar
              position="fixed"
              sx={{ zIndex: (theme) => theme.zIndex.drawer + 1 }}
            >
              <Toolbar>
                <Typography variant="h6" noWrap component="div">
                  Ella Private Network
                </Typography>
              </Toolbar>
            </AppBar>

            <Drawer
              variant="permanent"
              sx={{
                width: drawerWidth,
                flexShrink: 0,
                [`& .MuiDrawer-paper`]: {
                  width: drawerWidth,
                  boxSizing: "border-box",
                  display: "flex",
                  flexDirection: "column",
                },
              }}
            >
              <Toolbar />
              <Box sx={{ flexGrow: 1, overflow: "auto" }}>
                <List>
                  <ListItem disablePadding>
                    <ListItemButton
                      component="a"
                      href="/network-configuration"
                      selected={pathname === "/network-configuration"}
                    >
                      <ListItemIcon>
                        <NetworkIcon />
                      </ListItemIcon>
                      <ListItemText primary="Network Configuration" />
                    </ListItemButton>
                  </ListItem>
                  <ListItem disablePadding>
                    <ListItemButton
                      component="a"
                      href="/subscribers"
                      selected={pathname === "/subscribers"}
                    >
                      <ListItemIcon>
                        <PeopleIcon />
                      </ListItemIcon>
                      <ListItemText primary="Subscribers" />
                    </ListItemButton>
                  </ListItem>
                </List>
              </Box>
              <Divider />
              <Box>
                <List>
                  <ListItem disablePadding>
                    <ListItemButton
                      component="a"
                      href="https://github.com/yeastengine/ella"
                      target="_blank"
                      rel="noreferrer"
                    >
                      <ListItemIcon>
                        <InfoIcon />
                      </ListItemIcon>
                      <ListItemText primary="Documentation" />
                    </ListItemButton>
                  </ListItem>
                  <ListItem disablePadding>
                    <ListItemButton
                      component="a"
                      href="https://github.com/yeastengine/ella/issues/new/choose"
                      target="_blank"
                      rel="noreferrer"
                    >
                      <ListItemIcon>
                        <BugReportIcon />
                      </ListItemIcon>
                      <ListItemText primary="Report a bug" />
                    </ListItemButton>
                  </ListItem>
                </List>
              </Box>
            </Drawer>
            <Box component="main" sx={{ flexGrow: 1, p: 3 }}>
              <Toolbar />
              <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
            </Box>
          </Box>
        </ThemeProvider>
      </body>
    </html>
  );
}
