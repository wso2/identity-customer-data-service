import React, { useEffect, useState } from 'react';
import {
  Paper, Box, Typography, Button, Switch, TextField, Divider,
  CircularProgress, Alert, Snackbar, Stack, Chip, FormControlLabel,
  Slider,
} from '@mui/material';
import SaveIcon from '@mui/icons-material/Save';
import PageHeader from '../../components/PageHeader';
import { getAdminConfig, updateAdminConfig } from '../../api/api';

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

  const handleAddApp = (e) => {
    if (e.key === 'Enter' && e.target.value.trim()) {
      const app = e.target.value.trim();
      const apps = draft.system_applications || [];
      if (!apps.includes(app)) {
        update('system_applications', [...apps, app]);
      }
      e.target.value = '';
    }
  };

  const handleRemoveApp = (app) => {
    update('system_applications', (draft.system_applications || []).filter((a) => a !== app));
  };

  if (loading) return <Box sx={{ display: 'flex', justifyContent: 'center', py: 10 }}><CircularProgress /></Box>;
  if (!draft) return <Alert severity="error">Failed to load admin configuration</Alert>;

  return (
    <>
      <PageHeader
        title="Admin Config"
        subtitle="Organization-level identity resolution settings"
        action={
          <Button variant="contained" startIcon={<SaveIcon />} onClick={handleSave} disabled={!dirty || saving}>
            {saving ? 'Saving…' : 'Save Changes'}
          </Button>
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

        {/* ─── System Applications ──────────── */}
        <Paper sx={{ p: 3 }}>
          <Typography variant="h6" gutterBottom>System Applications</Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            Application identifiers that are treated as system-level and bypass certain restrictions
          </Typography>
          <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1, mb: 2 }}>
            {(draft.system_applications || []).length === 0 ? (
              <Typography variant="body2" color="text.secondary" sx={{ fontStyle: 'italic' }}>
                No system applications configured
              </Typography>
            ) : (
              (draft.system_applications || []).map((app) => (
                <Chip key={app} label={app} onDelete={() => handleRemoveApp(app)} variant="outlined" />
              ))
            )}
          </Box>
          <TextField
            size="small"
            placeholder="Type application ID and press Enter"
            onKeyDown={handleAddApp}
            fullWidth
            helperText="Press Enter to add an application identifier"
          />
        </Paper>

        {/* ─── Identity Resolution ──────────── */}
        <Paper sx={{ p: 3 }}>
          <Typography variant="h6" gutterBottom>Identity Resolution</Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
            Configure how profiles are automatically merged and resolved
          </Typography>

          <Stack spacing={3}>
            <FormControlLabel
              control={
                <Switch
                  checked={!!draft.auto_merge_enabled}
                  onChange={(e) => update('auto_merge_enabled', e.target.checked)}
                />
              }
              label={
                <Box>
                  <Typography variant="body1">Enable Auto Merge</Typography>
                  <Typography variant="caption" color="text.secondary">
                    Automatically merge profiles when the confidence score exceeds the auto merge threshold
                  </Typography>
                </Box>
              }
              sx={{ alignItems: 'flex-start', ml: 0 }}
            />

            <FormControlLabel
              control={
                <Switch
                  checked={!!draft.smart_resolution_enabled}
                  onChange={(e) => update('smart_resolution_enabled', e.target.checked)}
                />
              }
              label={
                <Box>
                  <Typography variant="body1">Enable Smart Resolution</Typography>
                  <Typography variant="caption" color="text.secondary">
                    Use advanced matching algorithms for identity resolution
                  </Typography>
                </Box>
              }
              sx={{ alignItems: 'flex-start', ml: 0 }}
            />

            <Divider />

            <Box>
              <Typography variant="body1" gutterBottom>Auto Merge Threshold</Typography>
              <Typography variant="caption" color="text.secondary" display="block" sx={{ mb: 1 }}>
                Minimum confidence score required to automatically merge two profiles (0 – 1)
              </Typography>
              <Stack direction="row" spacing={2} alignItems="center">
                <Slider
                  value={draft.auto_merge_threshold ?? 0}
                  onChange={(_, v) => update('auto_merge_threshold', v)}
                  min={0} max={1} step={0.01}
                  valueLabelDisplay="auto"
                  sx={{ flex: 1 }}
                />
                <TextField
                  size="small"
                  type="number"
                  value={draft.auto_merge_threshold ?? 0}
                  onChange={(e) => {
                    let v = parseFloat(e.target.value);
                    if (isNaN(v)) v = 0;
                    if (v < 0) v = 0;
                    if (v > 1) v = 1;
                    update('auto_merge_threshold', v);
                  }}
                  slotProps={{ htmlInput: { min: 0, max: 1, step: 0.01 } }}
                  sx={{ width: 100 }}
                />
              </Stack>
            </Box>

            <Box>
              <Typography variant="body1" gutterBottom>Manual Review Threshold</Typography>
              <Typography variant="caption" color="text.secondary" display="block" sx={{ mb: 1 }}>
                Minimum confidence score required to flag a match for manual review (0 – 1)
              </Typography>
              <Stack direction="row" spacing={2} alignItems="center">
                <Slider
                  value={draft.manual_review_threshold ?? 0}
                  onChange={(_, v) => update('manual_review_threshold', v)}
                  min={0} max={1} step={0.01}
                  valueLabelDisplay="auto"
                  sx={{ flex: 1 }}
                />
                <TextField
                  size="small"
                  type="number"
                  value={draft.manual_review_threshold ?? 0}
                  onChange={(e) => {
                    let v = parseFloat(e.target.value);
                    if (isNaN(v)) v = 0;
                    if (v < 0) v = 0;
                    if (v > 1) v = 1;
                    update('manual_review_threshold', v);
                  }}
                  slotProps={{ htmlInput: { min: 0, max: 1, step: 0.01 } }}
                  sx={{ width: 100 }}
                />
              </Stack>
            </Box>
          </Stack>
        </Paper>
      </Stack>

      <Snackbar open={toast.open} autoHideDuration={4000} onClose={() => setToast({ ...toast, open: false })} anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}>
        <Alert severity={toast.severity} variant="filled" onClose={() => setToast({ ...toast, open: false })}>{toast.msg}</Alert>
      </Snackbar>
    </>
  );
}
