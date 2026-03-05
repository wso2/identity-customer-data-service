import React, { useEffect, useState } from 'react';
import {
  Paper, Table, TableBody, TableCell, TableContainer, TableHead, TableRow,
  Button, IconButton, Chip, Box, Dialog, DialogTitle, DialogContent,
  DialogActions, TextField, MenuItem, Stack, Switch, CircularProgress,
  Alert, Snackbar, Tooltip, Typography,
} from '@mui/material';
import AddIcon from '@mui/icons-material/Add';
import EditIcon from '@mui/icons-material/Edit';
import DeleteIcon from '@mui/icons-material/DeleteOutline';
import PageHeader from '../../components/PageHeader';
import EmptyState from '../../components/EmptyState';
import ConfirmDialog from '../../components/ConfirmDialog';
import {
  getUnificationRules, addUnificationRule, patchUnificationRule, deleteUnificationRule,
} from '../../api';

const EMPTY_RULE = { rule_name: '', property_name: '', priority: 1, is_active: true };

export default function UnificationRulesPage() {
  const [rules, setRules] = useState([]);
  const [loading, setLoading] = useState(true);
  const [toast, setToast] = useState({ open: false, msg: '', severity: 'success' });

  // Dialogs
  const [formOpen, setFormOpen] = useState(false);
  const [formData, setFormData] = useState(EMPTY_RULE);
  const [editingId, setEditingId] = useState(null);
  const [deleteTarget, setDeleteTarget] = useState(null);

  const load = async () => {
    setLoading(true);
    try {
      const res = await getUnificationRules();
      setRules(Array.isArray(res) ? res : []);
    } catch {
      setToast({ open: true, msg: 'Failed to load rules', severity: 'error' });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  const openCreate = () => {
    setEditingId(null);
    setFormData({ ...EMPTY_RULE });
    setFormOpen(true);
  };

  const openEdit = (rule) => {
    setEditingId(rule.rule_id);
    setFormData({
      rule_name: rule.rule_name,
      property_name: rule.property_name,
      priority: rule.priority,
      is_active: rule.is_active,
    });
    setFormOpen(true);
  };

  const handleSave = async () => {
    try {
      if (editingId) {
        const { property_name, ...patchData } = formData;
        await patchUnificationRule(editingId, patchData);
        setToast({ open: true, msg: 'Rule updated', severity: 'success' });
      } else {
        await addUnificationRule(formData);
        setToast({ open: true, msg: 'Rule created', severity: 'success' });
      }
      setFormOpen(false);
      load();
    } catch (e) {
      setToast({ open: true, msg: `Failed: ${e.message || e}`, severity: 'error' });
    }
  };

  const handleToggle = async (rule) => {
    try {
      await patchUnificationRule(rule.rule_id, {
        rule_name: rule.rule_name,
        priority: rule.priority,
        is_active: !rule.is_active,
      });
      load();
    } catch {
      setToast({ open: true, msg: 'Failed to toggle rule', severity: 'error' });
    }
  };

  const handleDelete = async () => {
    try {
      await deleteUnificationRule(deleteTarget);
      setDeleteTarget(null);
      setToast({ open: true, msg: 'Rule deleted', severity: 'success' });
      load();
    } catch {
      setToast({ open: true, msg: 'Failed to delete', severity: 'error' });
    }
  };

  const sorted = [...rules].sort((a, b) => a.priority - b.priority);

  return (
    <>
      <PageHeader
        title="Unification Rules"
        subtitle="Blocking keys used for identity resolution matching"
        action={
          <Button variant="contained" startIcon={<AddIcon />} onClick={openCreate}>
            Add Rule
          </Button>
        }
      />

      <Paper>
        <TableContainer>
          <Table>
            <TableHead>
              <TableRow>
                <TableCell>Priority</TableCell>
                <TableCell>Rule Name</TableCell>
                <TableCell>Property</TableCell>
                <TableCell>Active</TableCell>
                <TableCell align="right">Actions</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell colSpan={5} align="center" sx={{ py: 6 }}><CircularProgress size={24} /></TableCell>
                </TableRow>
              ) : sorted.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5}><EmptyState title="No rules" subtitle="Create a unification rule" /></TableCell>
                </TableRow>
              ) : (
                sorted.map((r) => (
                  <TableRow key={r.rule_id} hover>
                    <TableCell>
                      <Chip label={r.priority} size="small" color="default" variant="outlined" />
                    </TableCell>
                    <TableCell sx={{ fontWeight: 500 }}>{r.rule_name}</TableCell>
                    <TableCell>
                      <Typography variant="body2" sx={{ fontFamily: 'monospace', fontSize: '0.8rem' }}>
                        {r.property_name}
                      </Typography>
                    </TableCell>
                    <TableCell>
                      <Switch
                        size="small"
                        checked={r.is_active}
                        onChange={() => handleToggle(r)}
                        color="primary"
                      />
                    </TableCell>
                    <TableCell align="right">
                      <Tooltip title="Edit"><IconButton size="small" onClick={() => openEdit(r)}><EditIcon fontSize="small" /></IconButton></Tooltip>
                      <Tooltip title="Delete"><IconButton size="small" color="error" onClick={() => setDeleteTarget(r.rule_id)}><DeleteIcon fontSize="small" /></IconButton></Tooltip>
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </TableContainer>
      </Paper>

      {/* ─── Add / Edit dialog ─────────────── */}
      <Dialog open={formOpen} onClose={() => setFormOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>{editingId ? 'Edit Rule' : 'Add Rule'}</DialogTitle>
        <DialogContent>
          <Stack spacing={2} sx={{ mt: 1 }}>
            <TextField
              label="Rule Name" fullWidth value={formData.rule_name}
              onChange={(e) => setFormData({ ...formData, rule_name: e.target.value })}
            />
            <TextField
              label="Property Name" fullWidth value={formData.property_name}
              onChange={(e) => setFormData({ ...formData, property_name: e.target.value })}
              helperText="e.g. identity_attributes.emailaddress"
              disabled={!!editingId}
            />
            <TextField
              label="Priority" type="number" fullWidth value={formData.priority}
              onChange={(e) => setFormData({ ...formData, priority: parseInt(e.target.value) || 1 })}
              slotProps={{ htmlInput: { min: 1 } }}
            />
          </Stack>
        </DialogContent>
        <DialogActions sx={{ px: 3, pb: 2 }}>
          <Button color="inherit" onClick={() => setFormOpen(false)}>Cancel</Button>
          <Button variant="contained" onClick={handleSave} disabled={!formData.rule_name || !formData.property_name}>
            {editingId ? 'Update' : 'Create'}
          </Button>
        </DialogActions>
      </Dialog>

      {/* ─── Delete confirm ────────────────── */}
      <ConfirmDialog
        open={!!deleteTarget}
        title="Delete Rule"
        message="This will remove the rule permanently. Are you sure?"
        confirmLabel="Delete"
        onConfirm={handleDelete}
        onCancel={() => setDeleteTarget(null)}
      />

      <Snackbar open={toast.open} autoHideDuration={4000} onClose={() => setToast({ ...toast, open: false })} anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}>
        <Alert severity={toast.severity} variant="filled" onClose={() => setToast({ ...toast, open: false })}>{toast.msg}</Alert>
      </Snackbar>
    </>
  );
}
