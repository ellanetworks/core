"use client";

import React, { useState } from "react";
import {
  Box,
  CssBaseline,
  Toolbar,
  AppBar,
  Drawer,
  List,
  ListItem,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  Divider,
  Typography,
} from "@mui/material";
import {
  Info as InfoIcon,
  BugReport as BugReportIcon,
  Person as PersonIcon,
  Sensors as SensorsIcon,
  Groups as GroupsIcon,
  Dashboard as DashboardIcon,
  Router as RouterIcon,
} from "@mui/icons-material";
import { usePathname } from "next/navigation";
import { ThemeProvider } from "@mui/material/styles";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import theme from "@/utils/theme";
import "./globals.scss";
import Logo from "@/components/Logo";

const drawerWidth = 250;

export default function RootLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const [queryClient] = useState(() => new QueryClient());

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
        <QueryClientProvider client={queryClient}>
          <ThemeProvider theme={theme}>
            <CssBaseline />
            <Box sx={{ display: "flex" }}>
              <AppBar
                position="fixed"
                sx={{ zIndex: (theme) => theme.zIndex.drawer + 1 }}
              >
                <Toolbar>
                  <Logo width={50} height={50} />
                  <Typography variant="h6" noWrap component="div" sx={{ ml: 2 }}>
                    Ella Core
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
                        href="/dashboard"
                        selected={pathname === "/dashboard"}
                      >
                        <ListItemIcon>
                          <DashboardIcon />
                        </ListItemIcon>
                        <ListItemText primary="Dashboard" />
                      </ListItemButton>
                    </ListItem>
                    <ListItem disablePadding>
                      <ListItemButton
                        component="a"
                        href="/network-configuration"
                        selected={pathname === "/network-configuration"}
                      >
                        <ListItemIcon>
                          <SensorsIcon />
                        </ListItemIcon>
                        <ListItemText primary="Network Configuration" />
                      </ListItemButton>
                    </ListItem>
                    <ListItem disablePadding>
                      <ListItemButton
                        component="a"
                        href="/radios"
                        selected={pathname === "/radios"}
                      >
                        <ListItemIcon>
                          <RouterIcon />
                        </ListItemIcon>
                        <ListItemText primary="Radios" />
                      </ListItemButton>
                    </ListItem>
                    <ListItem disablePadding>
                      <ListItemButton
                        component="a"
                        href="/profiles"
                        selected={pathname === "/profiles"}
                      >
                        <ListItemIcon>
                          <GroupsIcon />
                        </ListItemIcon>
                        <ListItemText primary="Profiles" />
                      </ListItemButton>
                    </ListItem>
                    <ListItem disablePadding>
                      <ListItemButton
                        component="a"
                        href="/subscribers"
                        selected={pathname === "/subscribers"}
                      >
                        <ListItemIcon>
                          <PersonIcon />
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
                        href="https://github.com/ellanetworks/core"
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
                        href="https://github.com/ellanetworks/core/issues/new/choose"
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
                {children}
              </Box>
            </Box>
          </ThemeProvider>
        </QueryClientProvider>
      </body>
    </html>
  );
}
