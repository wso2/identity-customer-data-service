import {
  Box,
  Divider,
  List,
  ListItem,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  Tooltip,
  Typography,
} from '@oxygen-ui/react';
import { Drawer, IconButton } from '@mui/material';
import ChevronLeftIcon from '@mui/icons-material/ChevronLeft';
import ChevronRightIcon from '@mui/icons-material/ChevronRight';
import PeopleIcon from '@mui/icons-material/PeopleOutlined';
import SchemaIcon from '@mui/icons-material/AccountTreeOutlined';
import RuleIcon from '@mui/icons-material/MergeTypeOutlined';
import SettingsIcon from '@mui/icons-material/SettingsOutlined';
import type { ElementType } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';

export const DRAWER_WIDTH = 220;
export const DRAWER_WIDTH_COLLAPSED = 64;

interface NavItem {
  label: string;
  path: string;
  icon: ElementType;
}

interface NavSection {
  title: string;
  items: NavItem[];
}

const NAV_SECTIONS: NavSection[] = [
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
    title: 'Settings',
    items: [
      { label: 'Admin Config', path: '/config', icon: SettingsIcon },
    ],
  },
];

interface SidebarProps {
  collapsed: boolean;
  onToggle: () => void;
}

export default function Sidebar({ collapsed, onToggle }: SidebarProps) {
  const navigate = useNavigate();
  const { pathname } = useLocation();

  const isActive = (path: string) => pathname === path || pathname.startsWith(`${path}/`);
  const width = collapsed ? DRAWER_WIDTH_COLLAPSED : DRAWER_WIDTH;

  return (
    <Drawer
      variant="permanent"
      sx={{
        width,
        flexShrink: 0,
        transition: 'width 0.2s ease',
        '& .MuiDrawer-paper': {
          width,
          boxSizing: 'border-box',
          overflowX: 'hidden',
          transition: 'width 0.2s ease',
        },
      }}
    >
      {/* Brand */}
      <Box
        sx={{
          px: collapsed ? 1.5 : 2,
          py: 1.5,
          display: 'flex',
          alignItems: 'center',
          gap: 1.5,
          minHeight: 56,
          overflow: 'hidden',
        }}
      >
        <Box
          sx={{
            width: 28, height: 28, borderRadius: 1, flexShrink: 0,
            background: 'linear-gradient(135deg, #FF7300, #FF9800)',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            fontSize: 13, fontWeight: 800, color: '#fff',
          }}
        >
          C
        </Box>
        {!collapsed && (
          <Box sx={{ overflow: 'hidden' }}>
            <Typography variant="subtitle2" sx={{ fontWeight: 700, lineHeight: 1.2, color: '#222228', whiteSpace: 'nowrap' }}>
              Customer Data
            </Typography>
            <Typography variant="caption" sx={{ color: '#999', fontSize: '0.65rem', whiteSpace: 'nowrap' }}>
              Identity Resolution
            </Typography>
          </Box>
        )}
      </Box>

      <Divider />

      {/* Nav sections */}
      <Box sx={{ overflowY: 'auto', overflowX: 'hidden', flexGrow: 1, pt: 1 }}>
        {NAV_SECTIONS.map((section) => (
          <Box key={section.title}>
            {!collapsed && (
              <Typography
                variant="overline"
                sx={{
                  px: 2,
                  pt: 1.5,
                  pb: 0.25,
                  display: 'block',
                  color: '#bbb',
                  fontSize: '0.6rem',
                  letterSpacing: '0.08em',
                  whiteSpace: 'nowrap',
                }}
              >
                {section.title}
              </Typography>
            )}
            {collapsed && <Box sx={{ pt: 1 }} />}
            <List dense sx={{ px: collapsed ? 0.5 : 1, py: 0 }}>
              {section.items.map(({ label, path, icon: Icon }) => {
                const active = isActive(path);
                const button = (
                  <ListItemButton
                    onClick={() => navigate(path)}
                    sx={{
                      borderRadius: 1,
                      py: 0.75,
                      px: collapsed ? 1.5 : 1.25,
                      justifyContent: collapsed ? 'center' : 'flex-start',
                      color: active ? '#FF7300' : '#555',
                      bgcolor: active ? 'rgba(255,115,0,0.08)' : 'transparent',
                      borderLeft: active ? '3px solid #FF7300' : '3px solid transparent',
                      minWidth: 0,
                      '&:hover': {
                        bgcolor: active ? 'rgba(255,115,0,0.12)' : 'rgba(0,0,0,0.04)',
                      },
                    }}
                  >
                    <ListItemIcon
                      sx={{
                        color: 'inherit',
                        minWidth: collapsed ? 0 : 32,
                        justifyContent: 'center',
                      }}
                    >
                      <Icon fontSize="small" />
                    </ListItemIcon>
                    {!collapsed && (
                      <ListItemText
                        primary={label}
                        primaryTypographyProps={{
                          fontSize: '0.82rem',
                          fontWeight: active ? 600 : 400,
                          whiteSpace: 'nowrap',
                        }}
                      />
                    )}
                  </ListItemButton>
                );

                return (
                  <ListItem key={path} disablePadding sx={{ mb: 0.25 }}>
                    {collapsed ? (
                      <Tooltip title={label} placement="right">
                        {button}
                      </Tooltip>
                    ) : button}
                  </ListItem>
                );
              })}
            </List>
          </Box>
        ))}
      </Box>

      <Divider />

      {/* Footer: version + collapse toggle */}
      <Box
        sx={{
          p: 1,
          display: 'flex',
          alignItems: 'center',
          justifyContent: collapsed ? 'center' : 'space-between',
        }}
      >
        {!collapsed && (
          <Typography variant="caption" sx={{ color: '#bbb', fontSize: '0.65rem', pl: 1 }}>
            WSO2 Identity Server
          </Typography>
        )}
        <Tooltip title={collapsed ? 'Expand sidebar' : 'Collapse sidebar'} placement="right">
          <IconButton size="small" onClick={onToggle} sx={{ color: '#bbb' }}>
            {collapsed ? <ChevronRightIcon fontSize="small" /> : <ChevronLeftIcon fontSize="small" />}
          </IconButton>
        </Tooltip>
      </Box>
    </Drawer>
  );
}
