// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useEffect, useRef, useState } from "react";
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
  Menu,
  MenuItem,
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
  Hub as HubIcon,
  Router as RouterIcon,
  Logout as LogoutIcon,
  AccountCircle as AccountCircleIcon,
  Person as PersonIcon,
  Brightness6 as ThemeIcon,
  LightMode as LightModeIcon,
  DarkMode as DarkModeIcon,
  Check as CheckIcon,
  Storage as StorageIcon,
  Lan as LanIcon,
  HelpCenter as SupportIcon,
  OpenInNew as OpenInNewIcon,
} from "@mui/icons-material";
import { Link, useLocation, useNavigate } from "react-router-dom";
import Logo from "@/components/Logo";
import ProductTitle from "@/components/ProductTitle";
import SupportModal from "@/components/SupportModal";
import { useAuth } from "@/contexts/AuthContext";
import useMediaQuery from "@mui/material/useMediaQuery";
import { useColorScheme, useTheme } from "@mui/material/styles";
import IconButton from "@mui/material/IconButton";
import MenuIcon from "@mui/icons-material/Menu";
import Footer from "@/components/Footer";
import { logout } from "@/queries/auth";
import { PRODUCT } from "@/utils/product";

const drawerWidth = 250;

const THEME_MODES = [
  { value: "system", label: "System", Icon: ThemeIcon },
  { value: "light", label: "Light", Icon: LightModeIcon },
  { value: "dark", label: "Dark", Icon: DarkModeIcon },
] as const;

const drawerSelectedSx = {
  "& .MuiListItemText-primary": { color: "primary.main" },

  "&:hover": { bgcolor: "transparent" },
  "&.Mui-selected": { bgcolor: "transparent" },
  "&.Mui-selected:hover": { bgcolor: "transparent" },

  "&.Mui-selected .MuiListItemText-primary": {
    fontWeight: 700,
    textDecoration: "underline",
    textDecorationColor: "primary.main",
    textUnderlineOffset: "4px",
    textDecorationThickness: "2px",
  },

  "&:hover .MuiListItemText-primary": {
    textDecoration: "underline",
    textDecorationColor: "primary.main",
    textUnderlineOffset: "4px",
    textDecorationThickness: "2px",
  },
};

function NavItem({
  to,
  label,
  icon,
  match,
  exact = false,
  pathname,
  onNavigate,
}: {
  to: string;
  label: string;
  icon: React.ReactNode;
  match?: string;
  exact?: boolean;
  pathname: string;
  onNavigate: () => void;
}) {
  const base = match ?? to;
  const current = exact ? pathname === base : pathname.startsWith(base);
  return (
    <ListItem disablePadding>
      <ListItemButton
        component={Link}
        to={to}
        selected={current}
        aria-current={current ? "page" : undefined}
        onClick={onNavigate}
        sx={drawerSelectedSx}
      >
        <ListItemIcon>{icon}</ListItemIcon>
        <ListItemText primary={label} />
      </ListItemButton>
    </ListItem>
  );
}

export default function DrawerLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const { pathname } = useLocation();
  const navigate = useNavigate();
  const theme = useTheme();
  const isMobile = useMediaQuery(theme.breakpoints.down("lg"));
  const { role, setAuthData } = useAuth();
  const { mode, setMode } = useColorScheme();

  const isFirstRender = useRef(true);
  useEffect(() => {
    if (isFirstRender.current) {
      isFirstRender.current = false;
      return;
    }
    document.getElementById("main-content")?.focus();
    // oxlint-disable-next-line react/exhaustive-effect-dependencies
  }, [pathname]);

  const [supportOpen, setSupportOpen] = useState(false);

  const closeSupport = () => setSupportOpen(false);

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
      <Box
        component="a"
        href="#main-content"
        onClick={(e: React.MouseEvent<HTMLAnchorElement>) => {
          e.preventDefault();
          document.getElementById("main-content")?.focus();
        }}
        sx={{
          position: "absolute",
          left: "-9999px",
          top: "auto",
          width: "1px",
          height: "1px",
          overflow: "hidden",
          zIndex: (t) => t.zIndex.modal + 1,
          "&:focus": {
            position: "fixed",
            top: 8,
            left: 8,
            width: "auto",
            height: "auto",
            overflow: "visible",
            bgcolor: "background.paper",
            color: "primary.main",
            px: 2,
            py: 1,
            borderRadius: 1,
            boxShadow: 3,
            fontWeight: 700,
            textDecoration: "none",
          },
        }}
      >
        Skip to main content
      </Box>
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
          <ProductTitle />

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
            <Divider />
            <ListSubheader disableSticky role="presentation">
              Theme
            </ListSubheader>
            {/* oxlint-disable-next-line jsx-a11y/prefer-tag-over-role */}
            <li role="group" aria-label="Theme">
              {THEME_MODES.map(({ value, label, Icon }) => (
                <MenuItem
                  key={value}
                  component="div"
                  role="menuitemradio"
                  aria-label={`${label} theme`}
                  aria-checked={mode === value}
                  selected={mode === value}
                  onClick={() => {
                    setMode(value);
                    handleAccountClose();
                  }}
                >
                  <ListItemIcon>
                    <Icon fontSize="small" color="primary" />
                  </ListItemIcon>
                  <ListItemText primary={label} />
                  {mode === value && (
                    <CheckIcon fontSize="small" sx={{ ml: 2 }} />
                  )}
                </MenuItem>
              ))}
            </li>
            <Divider />
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
        <Box
          component="nav"
          aria-label="Main"
          sx={{ flexGrow: 1, overflow: "auto" }}
        >
          <List>
            <NavItem
              to="/dashboard"
              label="Dashboard"
              icon={<DashboardIcon color="primary" />}
              exact
              pathname={pathname}
              onNavigate={handleNavClick}
            />
            <NavItem
              to="/operator"
              label="Operator"
              icon={<FeedIcon color="primary" />}
              exact
              pathname={pathname}
              onNavigate={handleNavClick}
            />
            <NavItem
              to="/radios"
              label="Radios"
              icon={<RouterIcon color="primary" />}
              pathname={pathname}
              onNavigate={handleNavClick}
            />
            <NavItem
              to="/networking"
              label="Networking"
              icon={<LanIcon color="primary" />}
              pathname={pathname}
              onNavigate={handleNavClick}
            />
            <NavItem
              to="/profiles"
              label="Profiles"
              icon={<TuneIcon color="primary" />}
              pathname={pathname}
              onNavigate={handleNavClick}
            />
            <NavItem
              to="/subscribers"
              label="Subscribers"
              icon={<GroupsIcon color="primary" />}
              pathname={pathname}
              onNavigate={handleNavClick}
            />
            <NavItem
              to="/traffic/usage"
              label="Traffic"
              icon={<BarChartIcon color="primary" />}
              match="/traffic"
              pathname={pathname}
              onNavigate={handleNavClick}
            />
            {role === "Admin" && (
              <>
                <ListSubheader
                  sx={{
                    borderTop: 1,
                    borderColor: "divider",
                    mt: 1,
                    pt: 1,
                  }}
                >
                  System
                </ListSubheader>
                <NavItem
                  to="/users"
                  label="Users"
                  icon={<AdminPanelSettingsIcon color="primary" />}
                  pathname={pathname}
                  onNavigate={handleNavClick}
                />
                <NavItem
                  to="/audit-logs"
                  label="Audit Logs"
                  icon={<ReceiptLongIcon color="primary" />}
                  exact
                  pathname={pathname}
                  onNavigate={handleNavClick}
                />
                <NavItem
                  to="/backup-restore"
                  label="Backup and Restore"
                  icon={<StorageIcon color="primary" />}
                  exact
                  pathname={pathname}
                  onNavigate={handleNavClick}
                />
                <NavItem
                  to="/cluster"
                  label="Cluster"
                  icon={<HubIcon color="primary" />}
                  exact
                  pathname={pathname}
                  onNavigate={handleNavClick}
                />
              </>
            )}
          </List>
        </Box>

        <Divider />
        <Box>
          <List>
            <ListSubheader>Support</ListSubheader>
            <ListItem disablePadding>
              <ListItemButton
                component="a"
                href={PRODUCT.docsUrl}
                target="_blank"
                rel="noreferrer"
                onClick={handleNavClick}
                sx={drawerSelectedSx}
              >
                <ListItemIcon>
                  <InfoIcon color="primary" />
                </ListItemIcon>
                <ListItemText primary="Documentation" />
                <OpenInNewIcon
                  sx={{ fontSize: 16, ml: 1, color: "action.active" }}
                />
              </ListItemButton>
            </ListItem>
            <ListItem disablePadding>
              <ListItemButton
                component="a"
                href="https://github.com/ellanetworks/core/issues/new/choose"
                target="_blank"
                rel="noreferrer"
                onClick={handleNavClick}
                sx={drawerSelectedSx}
              >
                <ListItemIcon>
                  <BugReportIcon color="primary" />
                </ListItemIcon>
                <ListItemText primary="Report a bug" />
                <OpenInNewIcon
                  sx={{ fontSize: 16, ml: 1, color: "action.active" }}
                />
              </ListItemButton>
            </ListItem>
            {role === "Admin" && (
              <ListItem disablePadding>
                <ListItemButton
                  onClick={() => {
                    setSupportOpen(true);
                    handleNavClick();
                  }}
                  sx={drawerSelectedSx}
                >
                  <ListItemIcon>
                    <SupportIcon color="primary" />
                  </ListItemIcon>
                  <ListItemText primary="Support Bundle" />
                </ListItemButton>
              </ListItem>
            )}
          </List>
        </Box>
        <SupportModal open={supportOpen} onClose={closeSupport} />
      </Drawer>
      <Box
        component="main"
        id="main-content"
        tabIndex={-1}
        sx={{
          outline: "none",
          flexGrow: 1,
          minWidth: 0,
          ml: isMobile ? 0 : `${drawerWidth}px`,
          minHeight: "100vh",
          display: "flex",
          flexDirection: "column",
          pt: 3,
        }}
      >
        <Toolbar />
        <Box sx={{ flexGrow: 1, minWidth: 0 }}>{children}</Box>
        <Footer />
      </Box>
    </Box>
  );
}
