import { useEffect, useState } from 'react';
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  FormControlLabel,
  Paper,
  Snackbar,
  Stack,
  Switch,
  TextField,
  Typography,
} from '@oxygen-ui/react';
import SaveIcon from '@mui/icons-material/Save';
import { getAdminConfig, updateAdminConfig } from '../../api';
import type { AdminConfig } from '../../models';
import PageHeader from '../../components/PageHeader';

interface Toast {
  open: boolean;
  msg: string;
  severity: 'success' | 'error' | 'info' | 'warning';
}

export default function AdminConfigPage() {
  const [config, setConfig] = useState<AdminConfig | null>(null);
  const [draft, setDraft] = useState<AdminConfig | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [dirty, setDirty] = useState(false);
  const [toast, setToast] = useState<Toast>({ open: false, msg: '', severity: 'success' });
  const [newApp, setNewApp] = useState('');

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

  const update = <K extends keyof AdminConfig>(key: K, value: AdminConfig[K]) => {
    if (!draft) return;
    setDraft({ ...draft, [key]: value });
    setDirty(true);
  };

  const handleSave = async () => {
    if (!draft) return;
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

  const handleAddApp = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key !== 'Enter' || !draft) return;
    const app = newApp.trim();
    if (!app) return;
    const apps = draft.system_applications ?? [];
    if (!apps.includes(app)) {
      update('system_applications', [...apps, app]);
    }
    setNewApp('');
  };

  const handleRemoveApp = (app: string) => {
    if (!draft) return;
    update('system_applications', (draft.system_applications ?? []).filter((a) => a !== app));
  };

  if (loading) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', py: 10 }}>
        <CircularProgress />
      </Box>
    );
  }
  if (!draft) return <Alert severity="error">Failed to load admin configuration</Alert>;

  return (
    <>
      <PageHeader
        title="Admin Config"
        subtitle="Organization-level configuration settings"
        action={
          <Button
            variant="contained"
            startIcon={<SaveIcon />}
            onClick={handleSave}
            disabled={!dirty || saving}
          >
            {saving ? 'Saving…' : 'Save Changes'}
          </Button>
        }
      />

      <Stack spacing={3}>
        {/* General */}
        <Paper sx={{ p: 3 }}>
          <Typography variant="h6" gutterBottom>General</Typography>
          <FormControlLabel
            control={
              <Switch
                checked={!!draft.cds_enabled}
                onChange={(e) => update('cds_enabled', e.target.checked)}
              />
            }
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
        </Paper>

        {/* System Applications */}
        <Paper sx={{ p: 3 }}>
          <Typography variant="h6" gutterBottom>System Applications</Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            Application identifiers that are treated as system-level and bypass certain restrictions
          </Typography>
          <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1, mb: 2 }}>
            {(draft.system_applications ?? []).length === 0 ? (
              <Typography variant="body2" color="text.secondary" sx={{ fontStyle: 'italic' }}>
                No system applications configured
              </Typography>
            ) : (
              (draft.system_applications ?? []).map((app) => (
                <Chip key={app} label={app} onDelete={() => handleRemoveApp(app)} variant="outlined" />
              ))
            )}
          </Box>
          <TextField
            size="small"
            placeholder="Type application ID and press Enter"
            value={newApp}
            onChange={(e) => setNewApp(e.target.value)}
            onKeyDown={handleAddApp}
            fullWidth
            helperText="Press Enter to add an application identifier"
          />
        </Paper>
      </Stack>

      <Snackbar
        open={toast.open}
        autoHideDuration={4000}
        onClose={() => setToast({ ...toast, open: false })}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
      >
        <Alert severity={toast.severity} variant="filled" onClose={() => setToast({ ...toast, open: false })}>
          {toast.msg}
        </Alert>
      </Snackbar>
    </>
  );
}
