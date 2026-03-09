import { useState } from 'react';
import { AppBar, Box, Toolbar, Typography } from '@oxygen-ui/react';
import { Outlet } from 'react-router-dom';
import Sidebar, { DRAWER_WIDTH, DRAWER_WIDTH_COLLAPSED } from './Sidebar';

export default function ConsoleLayout() {
  const [collapsed, setCollapsed] = useState(false);
  const sidebarWidth = collapsed ? DRAWER_WIDTH_COLLAPSED : DRAWER_WIDTH;

  return (
    <Box sx={{ display: 'flex', minHeight: '100vh', bgcolor: '#f6f4f2' }}>
      <Sidebar collapsed={collapsed} onToggle={() => setCollapsed((c) => !c)} />

      <Box
        sx={{
          flexGrow: 1,
          display: 'flex',
          flexDirection: 'column',
          ml: `${sidebarWidth}px`,
          transition: 'margin-left 0.2s ease',
        }}
      >
        <AppBar
          position="sticky"
          elevation={0}
          sx={{ bgcolor: '#f6f4f2', borderBottom: 'none', top: 0, zIndex: 10 }}
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
