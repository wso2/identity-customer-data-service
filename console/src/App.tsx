import { ThemeProvider } from '@oxygen-ui/react';
import CssBaseline from '@mui/material/CssBaseline';
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom';
import theme from './theme';
import ConsoleLayout from './layouts/ConsoleLayout';
import ProfilesListPage from './pages/profiles/ProfilesListPage';
import ProfileDetailPage from './pages/profiles/ProfileDetailPage';
import ProfileSchemaPage from './pages/schema/ProfileSchemaPage';
import UnificationRulesPage from './pages/rules/UnificationRulesPage';
import AdminConfigPage from './pages/config/AdminConfigPage';

export default function App() {
  return (
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <BrowserRouter>
        <Routes>
          <Route element={<ConsoleLayout />}>
            <Route path="/" element={<Navigate to="/profiles" replace />} />
            <Route path="/profiles" element={<ProfilesListPage />} />
            <Route path="/profiles/:id" element={<ProfileDetailPage />} />
            <Route path="/schema" element={<ProfileSchemaPage />} />
            <Route path="/rules" element={<UnificationRulesPage />} />
            <Route path="/config" element={<AdminConfigPage />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </ThemeProvider>
  );
}
