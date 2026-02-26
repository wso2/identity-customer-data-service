import React, { useEffect, useState } from 'react';
import {
  Paper, Box, Typography, Button, Switch, TextField, Divider,
  CircularProgress, Alert, Snackbar, Stack, Chip, FormControlLabel,
  Table, TableBody, TableCell, TableRow,
} from '@mui/material';
import SaveIcon from '@mui/icons-material/Save';
import RefreshIcon from '@mui/icons-material/Refresh';
import PageHeader from '../../components/PageHeader';
import { getAdminConfig, updateAdminConfig } from '../../api';

export default function AdminConfigPage() {
  const [config, setConfig] = useState(null);
  const [draft, setDraft] = useState(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [dirty, setDirty] = useState(false);
  const [toast, setToast] = useState({ open: false, msg: '', severity: 'success' });

  const load = async () => {
    setLoading(true);
    try {
      const res = await getAdminConfig();
      setConfig(res);
      setDraft({ ...res });
      setDirty(false);
    } catch {
      setToast({ open: true, msg: 'Failed to load config', severity: 'error' });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  const update = (key, value) => {
    setDraft({ ...draft, [key]: value });
    setDirty(true);
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      await updateAdminConfig(draft);
      setConfig({ ...draft });
      setDirty(false);
      setToast({ open: true, msg: 'Configuration saved', severity: 'success' });
    } catch {
      setToast({ open: true, msg: 'Failed to save', severity: 'error' });
    } finally {
      setSaving(false);
    }
  };

  if (loading) return <Box sx={{ display: 'flex', justifyContent: 'center', py: 10 }}><CircularProgress /></Box>;
  if (!draft) return <Alert severity="error">Failed to load admin configuration</Alert>;

  return (
    <>
      <PageHeader
        title="Admin Config"
        subtitle="Organization-level identity resolution settings"
        action={
          <Stack direction="row" spacing={1}>
            <Button variant="outlined" startIcon={<RefreshIcon />} onClick={load}>Refresh</Button>
            <Button variant="contained" startIcon={<SaveIcon />} onClick={handleSave} disabled={!dirty || saving}>
              {saving ? 'Saving…' : 'Save Changes'}
            </Button>
          </Stack>
        }
      />

      <Stack spacing={3}>
        {/* ─── General ─────────────────────── */}
        <Paper sx={{ p: 3 }}>
          <Typography variant="h6" gutterBottom>General</Typography>
          <Stack spacing={2}>
            <FormControlLabel
              control={<Switch checked={!!draft.cds_enabled} onChange={(e) => update('cds_enabled', e.target.checked)} />}
              label={
                <Box>
                  <Typography variant="body1">CDS Enabled</Typography>
                  <Typography variant="caption" color="text.secondary">
                    Enable or disable the Customer Data Service for this organization
                  </Typography>
                </Box>
              }
              sx={{ alignItems: 'flex-start', ml: 0 }}
            />
          </Stack>
        </Paper>

        {/* ─── Raw Configuration ──────────── */}
        {Object.keys(draft).filter((k) => k !== 'cds_enabled').length > 0 && (
          <Paper sx={{ p: 3 }}>
            <Typography variant="h6" gutterBottom>Additional Settings</Typography>
            <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
              Other configuration values returned by the API
            </Typography>
            <Table size="small">
              <TableBody>
                {Object.entries(draft)
                  .filter(([k]) => k !== 'cds_enabled')
                  .map(([key, val]) => (
                    <TableRow key={key}>
                      <TableCell sx={{ fontWeight: 500, width: 220, color: 'text.secondary' }}>{key}</TableCell>
                      <TableCell>
                        {typeof val === 'boolean' ? (
                          <Switch checked={val} onChange={(e) => update(key, e.target.checked)} size="small" />
                        ) : typeof val === 'number' ? (
                          <TextField
                            size="small"
                            type="number"
                            value={val}
                            onChange={(e) => update(key, parseFloat(e.target.value) || 0)}
                            sx={{ width: 120 }}
                          />
                        ) : (
                          <Typography variant="body2">{typeof val === 'object' ? JSON.stringify(val) : String(val ?? '')}</Typography>
                        )}
                      </TableCell>
                    </TableRow>
                  ))}
              </TableBody>
            </Table>
          </Paper>
        )}
      </Stack>

      <Snackbar open={toast.open} autoHideDuration={4000} onClose={() => setToast({ ...toast, open: false })} anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}>
        <Alert severity={toast.severity} variant="filled" onClose={() => setToast({ ...toast, open: false })}>{toast.msg}</Alert>
      </Snackbar>
    </>
  );
}
