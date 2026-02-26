import React from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { ThemeProvider } from '@oxygen-ui/react';
import CssBaseline from '@mui/material/CssBaseline';
import theme from './theme';
import ConsoleLayout from './layouts/ConsoleLayout';
import ProfilesListPage from './pages/profiles/ProfilesListPage';
import ProfileDetailPage from './pages/profiles/ProfileDetailPage';
import ProfileSchemaPage from './pages/schema/ProfileSchemaPage';
import UnificationRulesPage from './pages/rules/UnificationRulesPage';
import ReviewTasksPage from './pages/approval/ReviewTasksPage';
import ReviewDetailPage from './pages/approval/ReviewDetailPage';
import SearchPage from './pages/search/SearchPage';
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
            <Route path="/approval" element={<ReviewTasksPage />} />
            <Route path="/approval/:profileId" element={<ReviewDetailPage />} />
            <Route path="/search" element={<SearchPage />} />
            <Route path="/config" element={<AdminConfigPage />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </ThemeProvider>
  );
}
