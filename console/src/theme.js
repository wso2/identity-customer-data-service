import { extendTheme } from '@oxygen-ui/react';

const theme = extendTheme({
  colorSchemes: {
    light: {
      palette: {
        primary: { main: '#FF7300' },
        secondary: { main: '#1B1B2F' },
        background: { default: '#FAFBFC', paper: '#FFFFFF' },
        success: { main: '#4CAF50' },
        error: { main: '#E74C3C' },
        warning: { main: '#FF9800' },
        info: { main: '#2196F3' },
        text: { primary: '#1B1B2F', secondary: '#666666' },
      },
    },
  },
  typography: {
    fontFamily: '"Inter", "Segoe UI", "Roboto", "Helvetica Neue", Arial, sans-serif',
    h4: { fontWeight: 700, fontSize: '1.5rem' },
    h5: { fontWeight: 600, fontSize: '1.25rem' },
    h6: { fontWeight: 600, fontSize: '1rem' },
    subtitle1: { fontWeight: 500, fontSize: '0.875rem', color: '#666' },
    body2: { fontSize: '0.875rem' },
  },
  shape: { borderRadius: 8 },
  components: {
    MuiButton: {
      defaultProps: { disableElevation: true },
      styleOverrides: {
        root: { textTransform: 'none', fontWeight: 500 },
        sizeSmall: { fontSize: '0.8125rem', padding: '4px 12px' },
      },
    },
    MuiPaper: {
      defaultProps: { elevation: 0 },
      styleOverrides: { root: { border: '1px solid #E8E8E8' } },
    },
    MuiTableHead: {
      styleOverrides: { root: { '& .MuiTableCell-root': { fontWeight: 600, fontSize: '0.8125rem', color: '#666', textTransform: 'uppercase', letterSpacing: '0.5px', backgroundColor: '#FAFBFC', borderBottom: '2px solid #E8E8E8' } } },
    },
    MuiTableCell: {
      styleOverrides: { root: { fontSize: '0.875rem', padding: '12px 16px' } },
    },
    MuiChip: {
      styleOverrides: { root: { fontWeight: 500 } },
    },
    MuiDialog: {
      styleOverrides: { paper: { borderRadius: 12 } },
    },
    MuiDrawer: {
      styleOverrides: {
        paper: {
          backgroundColor: '#f6f4f2',
          color: '#222228',
          borderRight: 'none',
        },
      },
    },
  },
});

export default theme;
