"use client";

import React, { useCallback, useState, useEffect } from "react";
import {
  Box,
  Toolbar,
  AppBar,
  Drawer,
  List,
  ListSubheader,
  ListItem,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  Divider,
  Typography,
  Menu,
  MenuItem,
  Chip,
} from "@mui/material";
import {
  Info as InfoIcon,
  BugReport as BugReportIcon,
  Tune as TuneIcon,
  AdminPanelSettings as AdminPanelSettingsIcon,
  Sensors as SensorsIcon,
  Groups as GroupsIcon,
  Dashboard as DashboardIcon,
  Router as RouterIcon,
  Logout as LogoutIcon,
  AccountCircle as AccountCircleIcon,
  Storage as StorageIcon,
  Cable as CableIcon,
  Lan as LanIcon,
} from "@mui/icons-material";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import Logo from "@/components/Logo";
import { getLoggedInUser } from "@/queries/users";
import { useCookies } from "react-cookie";
import { useAuth } from "@/contexts/AuthContext";

const drawerWidth = 250;

export default function DrawerLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const pathname = usePathname();
  const router = useRouter();
  const [cookies] = useCookies(["user_token"]);
  const { role } = useAuth();

  if (!cookies.user_token) {
    router.push("/login");
  }

  // We still fetch email if needed, but you can also get it from the AuthContext.
  const [localEmail, setLocalEmail] = useState("");
  const [anchorEl, setAnchorEl] = useState<null | HTMLElement>(null);
  const open = Boolean(anchorEl);

  const handleMenuClick = (event: React.MouseEvent<HTMLElement>) => {
    setAnchorEl(event.currentTarget);
  };

  const handleMenuClose = () => {
    setAnchorEl(null);
  };

  const handleLogout = () => {
    localStorage.removeItem("user_token");
    router.push("/login");
  };

  const fetchUser = useCallback(async () => {
    try {
      const data = await getLoggedInUser(cookies.user_token);
      setLocalEmail(data.email);
    } catch (error) {
      console.error("Error fetching user:", error);
    }
  }, [cookies.user_token]);

  useEffect(() => {
    fetchUser();
  }, [fetchUser]);

  return (
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
          <Chip
            label={role}
            color={"warning"}
            variant="outlined"
            sx={{ ml: 2 }}
          />
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
                component={Link}
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
                component={Link}
                href="/operator"
                selected={pathname === "/operator"}
              >
                <ListItemIcon>
                  <SensorsIcon />
                </ListItemIcon>
                <ListItemText primary="Operator" />
              </ListItemButton>
            </ListItem>
            <ListItem disablePadding>
              <ListItemButton
                component={Link}
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
                component={Link}
                href="/data-networks"
                selected={pathname === "/data-networks"}
              >
                <ListItemIcon>
                  <LanIcon />
                </ListItemIcon>
                <ListItemText primary="Data Networks" />
              </ListItemButton>
            </ListItem>
            <ListItem disablePadding>
              <ListItemButton
                component={Link}
                href="/policies"
                selected={pathname === "/policies"}
              >
                <ListItemIcon>
                  <TuneIcon />
                </ListItemIcon>
                <ListItemText primary="Policies" />
              </ListItemButton>
            </ListItem>
            <ListItem disablePadding>
              <ListItemButton
                component={Link}
                href="/subscribers"
                selected={pathname === "/subscribers"}
              >
                <ListItemIcon>
                  <GroupsIcon />
                </ListItemIcon>
                <ListItemText primary="Subscribers" />
              </ListItemButton>
            </ListItem>
            <ListItem disablePadding>
              <ListItemButton
                component={Link}
                href="/routes"
                selected={pathname === "/routes"}
              >
                <ListItemIcon>
                  <CableIcon />
                </ListItemIcon>
                <ListItemText primary="Routes" />
              </ListItemButton>
            </ListItem>
            {role === "Admin" && (
              <>
                <Divider />
                <ListSubheader>System</ListSubheader>
                <ListItem disablePadding>
                  <ListItemButton
                    component={Link}
                    href="/users"
                    selected={pathname === "/users"}
                  >
                    <ListItemIcon>
                      <AdminPanelSettingsIcon />
                    </ListItemIcon>
                    <ListItemText primary="Users" />
                  </ListItemButton>
                </ListItem>
                <ListItem disablePadding>
                  <ListItemButton
                    component={Link}
                    href="/backup_restore"
                    selected={pathname === "/backup_restore"}
                  >
                    <ListItemIcon>
                      <StorageIcon />
                    </ListItemIcon>
                    <ListItemText primary="Backup and Restore" />
                  </ListItemButton>
                </ListItem>
              </>
            )}
          </List>
        </Box>
        <Box>
          <ListItemButton onClick={handleMenuClick}>
            <ListItemIcon>
              <AccountCircleIcon />
            </ListItemIcon>
            <ListItemText
              primary={
                <Typography
                  variant="body2"
                  noWrap
                  title={localEmail}
                  sx={{
                    textOverflow: "ellipsis",
                    overflow: "hidden",
                    whiteSpace: "nowrap",
                    maxWidth: "200px",
                  }}
                >
                  {localEmail}
                </Typography>
              }
            />
          </ListItemButton>
          <Menu
            anchorEl={anchorEl}
            open={open}
            onClose={handleMenuClose}
            anchorOrigin={{ vertical: "top", horizontal: "right" }}
            transformOrigin={{ vertical: "top", horizontal: "right" }}
          >
            <MenuItem onClick={handleLogout}>
              <ListItemIcon>
                <LogoutIcon fontSize="small" />
              </ListItemIcon>
              Logout
            </MenuItem>
          </Menu>
        </Box>
        <Divider />
        <Box>
          <List>
            <ListItem disablePadding>
              <ListItemButton
                component="a"
                href="https://docs.ellanetworks.com"
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
  );
}
