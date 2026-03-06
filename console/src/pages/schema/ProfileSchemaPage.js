import React, { useEffect, useState } from 'react';
import {
  Paper, Tabs, Tab, Table, TableBody, TableCell, TableContainer, TableHead,
  TableRow, Button, IconButton, Chip, Box, Dialog, DialogTitle, DialogContent,
  DialogActions, TextField, MenuItem, Stack, Typography, CircularProgress,
  Alert, Snackbar, Tooltip, Switch, FormControlLabel,
} from '@mui/material';
import AddIcon from '@mui/icons-material/Add';
import EditIcon from '@mui/icons-material/Edit';
import DeleteIcon from '@mui/icons-material/DeleteOutline';
import PageHeader from '../../components/PageHeader';
import EmptyState from '../../components/EmptyState';
import ConfirmDialog from '../../components/ConfirmDialog';
import {
  getSchema, getSchemaScope, addSchemaAttributes, patchSchemaAttribute, deleteSchemaAttribute,
} from '../../api/api';

const SCOPES = ['identity_attributes', 'traits', 'application_data'];
const VALUE_TYPES = ['string', 'integer', 'boolean', 'date', 'date_time', 'epoch', 'complex'];
const MERGE_STRATEGIES = ['overwrite', 'combine', 'ignore'];
const MUTABILITY_OPTS = ['readWrite', 'readOnly', 'immutable', 'writeOnly'];

const EMPTY_ATTR = {
  attribute_name: '',
  value_type: 'string',
  merge_strategy: 'overwrite',
  mutability: 'readWrite',
  multi_valued: false,
  application_identifier: '',
};

export default function ProfileSchemaPage() {
  const [scopeIdx, setScopeIdx] = useState(0);
  const [attributes, setAttributes] = useState([]);
  const [loading, setLoading] = useState(true);
  const [toast, setToast] = useState({ open: false, msg: '', severity: 'success' });

  // Dialogs
  const [formOpen, setFormOpen] = useState(false);
  const [formData, setFormData] = useState(EMPTY_ATTR);
  const [editingId, setEditingId] = useState(null);
  const [deleteTarget, setDeleteTarget] = useState(null);

  const scope = SCOPES[scopeIdx];

  const load = async () => {
    setLoading(true);
    try {
      const res = await getSchemaScope(scope);
      // identity_attributes & traits return flat array; application_data returns {appId: [attrs]}
      if (Array.isArray(res)) {
        setAttributes(res);
      } else if (res && typeof res === 'object') {
        // Flatten {appId: [attrs]} into single array with application_identifier
        const flat = [];
        Object.entries(res).forEach(([appId, attrs]) => {
          if (Array.isArray(attrs)) {
            attrs.forEach((a) => flat.push({ ...a, application_identifier: a.application_identifier || appId }));
          }
        });
        setAttributes(flat);
      } else {
        setAttributes([]);
      }
    } catch {
      setAttributes([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, [scope]); // eslint-disable-line

  const openCreate = () => {
    setEditingId(null);
    setFormData({ ...EMPTY_ATTR });
    setFormOpen(true);
  };

  const openEdit = (attr) => {
    setEditingId(attr.attribute_id);
    setFormData({
      attribute_name: attr.attribute_name || '',
      value_type: attr.value_type || 'string',
      merge_strategy: attr.merge_strategy || 'overwrite',
      mutability: attr.mutability || 'readWrite',
      multi_valued: !!attr.multi_valued,
      application_identifier: attr.application_identifier || '',
    });
    setFormOpen(true);
  };

  const handleSave = async () => {
    try {
      if (editingId) {
        await patchSchemaAttribute(scope, editingId, formData);
        setToast({ open: true, msg: 'Attribute updated', severity: 'success' });
      } else {
        await addSchemaAttributes(scope, formData);
        setToast({ open: true, msg: 'Attribute added', severity: 'success' });
      }
      setFormOpen(false);
      load();
    } catch (e) {
      setToast({ open: true, msg: `Failed to save: ${e.message || e}`, severity: 'error' });
    }
  };

  const handleDelete = async () => {
    try {
      await deleteSchemaAttribute(scope, deleteTarget);
      setDeleteTarget(null);
      setToast({ open: true, msg: 'Attribute deleted', severity: 'success' });
      load();
    } catch {
      setToast({ open: true, msg: 'Failed to delete', severity: 'error' });
    }
  };

  return (
    <>
      <PageHeader
        title="Profile Schema"
        subtitle="Define attribute schema for each scope"
        action={
          <Button variant="contained" startIcon={<AddIcon />} onClick={openCreate}>
            Add Attribute
          </Button>
        }
      />

      {/* Scope tabs */}
      <Box sx={{ mb: 2 }}>
        <Tabs value={scopeIdx} onChange={(_, v) => setScopeIdx(v)}>
          {SCOPES.map((s) => (
            <Tab key={s} label={s.replace(/_/g, ' ')} sx={{ textTransform: 'capitalize' }} />
          ))}
        </Tabs>
      </Box>

      {/* Attributes table */}
      <Paper>
        <TableContainer>
          <Table>
            <TableHead>
              <TableRow>
                <TableCell>Name</TableCell>
                <TableCell>Type</TableCell>
                <TableCell>Merge Strategy</TableCell>
                <TableCell>Mutability</TableCell>
                <TableCell>Multi-valued</TableCell>
                {scope === 'application_data' && <TableCell>App ID</TableCell>}
                <TableCell align="right">Actions</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell colSpan={7} align="center" sx={{ py: 6 }}><CircularProgress size={24} /></TableCell>
                </TableRow>
              ) : attributes.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={7}><EmptyState title="No attributes" subtitle="Add attributes to this scope" /></TableCell>
                </TableRow>
              ) : (
                attributes.map((a) => (
                  <TableRow key={a.attribute_id} hover>
                    <TableCell sx={{ fontWeight: 500 }}>{a.attribute_name}</TableCell>
                    <TableCell><Chip label={a.value_type} size="small" variant="outlined" /></TableCell>
                    <TableCell>{a.merge_strategy}</TableCell>
                    <TableCell>{a.mutability}</TableCell>
                    <TableCell>{a.multi_valued ? 'Yes' : 'No'}</TableCell>
                    {scope === 'application_data' && <TableCell>{a.application_identifier || '—'}</TableCell>}
                    <TableCell align="right">
                      <Tooltip title="Edit"><IconButton size="small" onClick={() => openEdit(a)}><EditIcon fontSize="small" /></IconButton></Tooltip>
                      <Tooltip title="Delete"><IconButton size="small" color="error" onClick={() => setDeleteTarget(a.attribute_id)}><DeleteIcon fontSize="small" /></IconButton></Tooltip>
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
        <DialogTitle>{editingId ? 'Edit Attribute' : 'Add Attribute'}</DialogTitle>
        <DialogContent>
          <Stack spacing={2} sx={{ mt: 1 }}>
            <TextField
              label="Attribute Name"
              fullWidth
              value={formData.attribute_name}
              onChange={(e) => setFormData({ ...formData, attribute_name: e.target.value })}
              disabled={!!editingId}
            />
            <TextField
              select label="Value Type" fullWidth value={formData.value_type}
              onChange={(e) => setFormData({ ...formData, value_type: e.target.value })}
            >
              {VALUE_TYPES.map((v) => <MenuItem key={v} value={v}>{v}</MenuItem>)}
            </TextField>
            <TextField
              select label="Merge Strategy" fullWidth value={formData.merge_strategy}
              onChange={(e) => setFormData({ ...formData, merge_strategy: e.target.value })}
            >
              {MERGE_STRATEGIES.map((v) => <MenuItem key={v} value={v}>{v}</MenuItem>)}
            </TextField>
            <TextField
              select label="Mutability" fullWidth value={formData.mutability}
              onChange={(e) => setFormData({ ...formData, mutability: e.target.value })}
            >
              {MUTABILITY_OPTS.map((v) => <MenuItem key={v} value={v}>{v}</MenuItem>)}
            </TextField>
            <FormControlLabel
              control={
                <Switch checked={formData.multi_valued} onChange={(e) => setFormData({ ...formData, multi_valued: e.target.checked })} />
              }
              label="Multi-valued"
            />
            {scope === 'application_data' && (
              <TextField
                label="Application Identifier"
                fullWidth
                value={formData.application_identifier}
                onChange={(e) => setFormData({ ...formData, application_identifier: e.target.value })}
              />
            )}
          </Stack>
        </DialogContent>
        <DialogActions sx={{ px: 3, pb: 2 }}>
          <Button color="inherit" onClick={() => setFormOpen(false)}>Cancel</Button>
          <Button variant="contained" onClick={handleSave} disabled={!formData.attribute_name}>
            {editingId ? 'Update' : 'Add'}
          </Button>
        </DialogActions>
      </Dialog>

      {/* ─── Delete confirm ────────────────── */}
      <ConfirmDialog
        open={!!deleteTarget}
        title="Delete Attribute"
        message="Are you sure? This cannot be undone."
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
