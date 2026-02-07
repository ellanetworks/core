import React, { useState } from "react";
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
  BarChart as BarChartIcon,
  ReceiptLong as ReceiptLongIcon,
  Tune as TuneIcon,
  AdminPanelSettings as AdminPanelSettingsIcon,
  Groups as GroupsIcon,
  Dashboard as DashboardIcon,
  Feed as FeedIcon,
  Router as RouterIcon,
  Logout as LogoutIcon,
  AccountCircle as AccountCircleIcon,
  Person as PersonIcon,
  Storage as StorageIcon,
  Lan as LanIcon,
} from "@mui/icons-material";
import { Link, useLocation, useNavigate } from "react-router-dom";
import Logo from "@/components/Logo";
import { useAuth } from "@/contexts/AuthContext";
import useMediaQuery from "@mui/material/useMediaQuery";
import { useTheme } from "@mui/material/styles";
import IconButton from "@mui/material/IconButton";
import MenuIcon from "@mui/icons-material/Menu";
import Footer from "@/components/Footer";
import { logout } from "@/queries/auth";

const drawerWidth = 250;

const drawerSelectedSx = {
  // remove the filled background
  "&.Mui-selected": { bgcolor: "transparent" },
  "&.Mui-selected:hover": { bgcolor: "transparent" },

  // make the label bold + underline
  "&.Mui-selected .MuiListItemText-primary": {
    fontWeight: 700,
    textDecoration: "underline",
    textDecorationColor: "primary.main",
    textUnderlineOffset: "4px",
    textDecorationThickness: "2px",
  },

  // on hover, show the underline even when not selected
  "&:hover .MuiListItemText-primary": {
    textDecorationColor: "primary.main",
    textUnderlineOffset: "4px",
  },
};

export default function DrawerLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const { pathname } = useLocation();
  const navigate = useNavigate();
  const theme = useTheme();
  const isMobile = useMediaQuery(theme.breakpoints.down("md"));
  const { role, setAuthData } = useAuth();

  const [mobileOpen, setMobileOpen] = useState(false);
  const handleDrawerToggle = () => setMobileOpen(!mobileOpen);
  const handleNavClick = () => {
    if (isMobile) setMobileOpen(false);
  };

  const [accountEl, setAccountEl] = useState<null | HTMLElement>(null);
  const accountMenuOpen = Boolean(accountEl);
  const handleAccountClick = (e: React.MouseEvent<HTMLElement>) =>
    setAccountEl(e.currentTarget);
  const handleAccountClose = () => setAccountEl(null);

  const handleProfile = () => {
    handleAccountClose();
    navigate("/profile");
  };

  const handleLogout = async () => {
    handleAccountClose();
    try {
      await logout();
    } catch {
    } finally {
      setAuthData(null);
      navigate("/login", { replace: true });
    }
  };

  return (
    <Box sx={{ display: "flex" }}>
      <AppBar position="fixed" sx={{ zIndex: (t) => t.zIndex.drawer + 1 }}>
        <Toolbar>
          {isMobile && (
            <IconButton
              color="inherit"
              aria-label="open drawer"
              edge="start"
              onClick={handleDrawerToggle}
              sx={{ mr: 2 }}
            >
              <MenuIcon />
            </IconButton>
          )}

          <Logo width={50} height={50} />
          <Typography variant="h6" noWrap component="div" sx={{ ml: 2 }}>
            Ella Core
          </Typography>

          <Chip
            label="free"
            variant="filled"
            sx={{ ml: 2, color: "text.primary", backgroundColor: "#F5F5F5" }}
          />

          <Box sx={{ flexGrow: 1 }} />

          <IconButton
            size="large"
            edge="end"
            color="inherit"
            aria-label="account menu"
            onClick={handleAccountClick}
          >
            <AccountCircleIcon />
          </IconButton>
          <Menu
            anchorEl={accountEl}
            open={accountMenuOpen}
            onClose={handleAccountClose}
            anchorOrigin={{ vertical: "bottom", horizontal: "right" }}
            transformOrigin={{ vertical: "top", horizontal: "right" }}
          >
            <MenuItem onClick={handleProfile}>
              <ListItemIcon>
                <PersonIcon fontSize="small" color="primary" />
              </ListItemIcon>
              <ListItemText primary="Profile" />
            </MenuItem>
            <MenuItem onClick={handleLogout}>
              <ListItemIcon>
                <LogoutIcon fontSize="small" color="primary" />
              </ListItemIcon>
              <ListItemText primary="Log Out" />
            </MenuItem>
          </Menu>
        </Toolbar>
      </AppBar>

      <Drawer
        variant={isMobile ? "temporary" : "permanent"}
        open={isMobile ? mobileOpen : true}
        onClose={handleDrawerToggle}
        ModalProps={{ keepMounted: true }}
        sx={{
          display: { xs: "block", sm: "block" },
          "& .MuiDrawer-paper": {
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
                to="/dashboard"
                selected={pathname === "/dashboard"}
                onClick={handleNavClick}
                sx={drawerSelectedSx}
              >
                <ListItemIcon>
                  <DashboardIcon color="primary" />
                </ListItemIcon>
                <ListItemText primary="Dashboard" />
              </ListItemButton>
            </ListItem>

            <ListItem disablePadding>
              <ListItemButton
                component={Link}
                to="/operator"
                selected={pathname === "/operator"}
                onClick={handleNavClick}
                sx={drawerSelectedSx}
              >
                <ListItemIcon>
                  <FeedIcon color="primary" />
                </ListItemIcon>
                <ListItemText primary="Operator" />
              </ListItemButton>
            </ListItem>

            <ListItem disablePadding>
              <ListItemButton
                component={Link}
                to="/radios"
                selected={pathname === "/radios"}
                onClick={handleNavClick}
                sx={drawerSelectedSx}
              >
                <ListItemIcon>
                  <RouterIcon color="primary" />
                </ListItemIcon>
                <ListItemText primary="Radios" />
              </ListItemButton>
            </ListItem>

            <ListItem disablePadding>
              <ListItemButton
                component={Link}
                to="/networking"
                selected={pathname === "/networking"}
                onClick={handleNavClick}
                sx={drawerSelectedSx}
              >
                <ListItemIcon>
                  <LanIcon color="primary" />
                </ListItemIcon>
                <ListItemText primary="Networking" />
              </ListItemButton>
            </ListItem>

            <ListItem disablePadding>
              <ListItemButton
                component={Link}
                to="/policies"
                selected={pathname === "/policies"}
                onClick={handleNavClick}
                sx={drawerSelectedSx}
              >
                <ListItemIcon>
                  <TuneIcon color="primary" />
                </ListItemIcon>
                <ListItemText primary="Policies" />
              </ListItemButton>
            </ListItem>

            <ListItem disablePadding>
              <ListItemButton
                component={Link}
                to="/subscribers"
                selected={pathname === "/subscribers"}
                onClick={handleNavClick}
                sx={drawerSelectedSx}
              >
                <ListItemIcon>
                  <GroupsIcon color="primary" />
                </ListItemIcon>
                <ListItemText primary="Subscribers" />
              </ListItemButton>
            </ListItem>

            <ListItem disablePadding>
              <ListItemButton
                component={Link}
                to="/usage"
                selected={pathname === "/usage"}
                onClick={handleNavClick}
                sx={drawerSelectedSx}
              >
                <ListItemIcon>
                  <BarChartIcon color="primary" />
                </ListItemIcon>
                <ListItemText primary="Usage" />
              </ListItemButton>
            </ListItem>

            {role === "Admin" && (
              <>
                <Divider />
                <ListSubheader>System</ListSubheader>

                <ListItem disablePadding>
                  <ListItemButton
                    component={Link}
                    to="/users"
                    selected={pathname === "/users"}
                    onClick={handleNavClick}
                    sx={drawerSelectedSx}
                  >
                    <ListItemIcon>
                      <AdminPanelSettingsIcon color="primary" />
                    </ListItemIcon>
                    <ListItemText primary="Users" />
                  </ListItemButton>
                </ListItem>

                <ListItem disablePadding>
                  <ListItemButton
                    component={Link}
                    to="/audit-logs"
                    selected={pathname === "/audit-logs"}
                    onClick={handleNavClick}
                    sx={drawerSelectedSx}
                  >
                    <ListItemIcon>
                      <ReceiptLongIcon color="primary" />
                    </ListItemIcon>
                    <ListItemText primary="Audit Logs" />
                  </ListItemButton>
                </ListItem>

                <ListItem disablePadding>
                  <ListItemButton
                    component={Link}
                    to="/backup-restore"
                    selected={pathname === "/backup-restore"}
                    onClick={handleNavClick}
                    sx={drawerSelectedSx}
                  >
                    <ListItemIcon>
                      <StorageIcon color="primary" />
                    </ListItemIcon>
                    <ListItemText primary="Backup and Restore" />
                  </ListItemButton>
                </ListItem>
              </>
            )}
          </List>
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
                onClick={handleNavClick}
              >
                <ListItemIcon>
                  <InfoIcon color="primary" />
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
                onClick={handleNavClick}
              >
                <ListItemIcon>
                  <BugReportIcon color="primary" />
                </ListItemIcon>
                <ListItemText primary="Report a bug" />
              </ListItemButton>
            </ListItem>
          </List>
        </Box>
      </Drawer>
      <Box
        component="main"
        sx={{
          flexGrow: 1,
          ml: isMobile ? 0 : `${drawerWidth}px`,
          minHeight: "100vh",
          display: "flex",
          flexDirection: "column",
          p: 3,
        }}
      >
        <Toolbar />
        <Box sx={{ flexGrow: 1 }}>{children}</Box>
        <Footer />
      </Box>
    </Box>
  );
}
