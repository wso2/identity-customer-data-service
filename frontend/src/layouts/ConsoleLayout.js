import React from 'react';
import { useNavigate, useLocation, Outlet } from 'react-router-dom';
import {
  Box, Drawer, List, ListItemButton, ListItemIcon, ListItemText,
  Typography, Divider, AppBar, Toolbar,
} from '@mui/material';
import PeopleIcon from '@mui/icons-material/PeopleOutlined';
import SchemaIcon from '@mui/icons-material/AccountTreeOutlined';
import RuleIcon from '@mui/icons-material/MergeTypeOutlined';
import ApprovalIcon from '@mui/icons-material/TaskAltOutlined';
import SearchIcon from '@mui/icons-material/SearchOutlined';
import SettingsIcon from '@mui/icons-material/SettingsOutlined';

const DRAWER_WIDTH = 256;

const NAV_SECTIONS = [
  {
    title: 'Customer Data',
    items: [
      { label: 'Profiles',          path: '/profiles', icon: PeopleIcon },
      { label: 'Unification Rules', path: '/rules',    icon: RuleIcon },
    ],
  },
  {
    title: 'Schema',
    items: [
      { label: 'Profile Schema', path: '/schema', icon: SchemaIcon },
    ],
  },
  {
    title: 'Identity Resolution',
    items: [
      { label: 'Approval', path: '/approval', icon: ApprovalIcon },
      { label: 'Search',   path: '/search',   icon: SearchIcon },
    ],
  },
  {
    title: 'Settings',
    items: [
      { label: 'Admin Config', path: '/config', icon: SettingsIcon },
    ],
  },
];

export default function ConsoleLayout() {
  const navigate = useNavigate();
  const { pathname } = useLocation();

  const isActive = (path) => pathname === path || pathname.startsWith(path + '/');

  return (
    <Box sx={{ display: 'flex', minHeight: '100vh', bgcolor: '#f6f4f2' }}>
      {/* ─── Sidebar ──────────────────────────────── */}
      <Drawer
        variant="permanent"
        sx={{
          width: DRAWER_WIDTH,
          flexShrink: 0,
          '& .MuiDrawer-paper': { width: DRAWER_WIDTH, boxSizing: 'border-box' },
        }}
      >
        {/* Brand */}
        <Box sx={{ px: 2.5, py: 2, display: 'flex', alignItems: 'center', gap: 1.5 }}>
          <Box
            sx={{
              width: 32, height: 32, borderRadius: 1,
              background: 'linear-gradient(135deg, #FF7300, #FF9800)',
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              fontSize: 14, fontWeight: 800, color: '#fff',
            }}
          >
            C
          </Box>
          <Box>
            <Typography variant="subtitle2" sx={{ fontWeight: 700, lineHeight: 1.2, color: '#222228' }}>
              Customer Data
            </Typography>
            <Typography variant="caption" sx={{ color: '#999', fontSize: '0.65rem' }}>
              Identity Resolution
            </Typography>
          </Box>
        </Box>

        {/* Navigation sections */}
        <Box sx={{ overflowY: 'auto', flexGrow: 1 }}>
          {NAV_SECTIONS.map((section, idx) => (
            <Box key={section.title}>
              <Typography
                variant="overline"
                sx={{ px: 2.5, pt: idx === 0 ? 2 : 2.5, pb: 0.5, display: 'block', color: '#999', fontSize: '0.65rem', letterSpacing: '0.08em' }}
              >
                {section.title}
              </Typography>
              <List dense sx={{ px: 1, py: 0 }}>
                {section.items.map(({ label, path, icon: Icon }) => {
                  const active = isActive(path);
                  return (
                    <ListItemButton
                      key={path}
                      onClick={() => navigate(path)}
                      sx={{
                        borderRadius: 1, mb: 0.25, py: 0.75,
                        color: active ? '#FF7300' : '#555',
                        bgcolor: active ? 'rgba(255,115,0,0.08)' : 'transparent',
                        borderLeft: active ? '3px solid #FF7300' : '3px solid transparent',
                        '&:hover': {
                          bgcolor: active ? 'rgba(255,115,0,0.12)' : 'rgba(0,0,0,0.04)',
                        },
                      }}
                    >
                      <ListItemIcon sx={{ color: 'inherit', minWidth: 32 }}>
                        <Icon fontSize="small" />
                      </ListItemIcon>
                      <ListItemText
                        primary={label}
                        primaryTypographyProps={{ fontSize: '0.85rem', fontWeight: active ? 600 : 400 }}
                      />
                    </ListItemButton>
                  );
                })}
              </List>
            </Box>
          ))}
        </Box>

        <Divider />
        <Box sx={{ p: 2 }}>
          <Typography variant="caption" sx={{ color: '#bbb', fontSize: '0.7rem' }}>
            WSO2 Identity Server
          </Typography>
        </Box>
      </Drawer>

      {/* ─── Main content ─────────────────────────── */}
      <Box sx={{ flexGrow: 1, display: 'flex', flexDirection: 'column' }}>
        <AppBar
          position="static"
          elevation={0}
          sx={{ bgcolor: '#f6f4f2', borderBottom: 'none', borderLeft: 'none', position: 'sticky', top: 0, zIndex: 10 }}
        >
          <Toolbar variant="dense" sx={{ minHeight: 48 }}>
            <Typography variant="body2" sx={{ color: 'text.secondary' }}>
              carbon.super
            </Typography>
          </Toolbar>
        </AppBar>

        <Box sx={{ flexGrow: 1, p: 3, bgcolor: '#faf9f8', overflow: 'auto', borderTopLeftRadius: 24 }}>
          <Outlet />
        </Box>
      </Box>
    </Box>
  );
}
