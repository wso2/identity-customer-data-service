import React, { useEffect, useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Paper, Box, Button, IconButton, Chip, TextField, InputAdornment, Dialog,
  DialogTitle, DialogContent, DialogActions, Tooltip, Typography,
  CircularProgress, Alert, Snackbar, Stack, List, ListItem, ListItemAvatar,
  ListItemText, Avatar, Divider,
} from '@mui/material';
import AddIcon from '@mui/icons-material/Add';
import DeleteIcon from '@mui/icons-material/DeleteOutline';
import SearchIcon from '@mui/icons-material/Search';
import FilterListIcon from '@mui/icons-material/FilterList';
import NavigateBeforeIcon from '@mui/icons-material/NavigateBefore';
import NavigateNextIcon from '@mui/icons-material/NavigateNext';
import PageHeader from '../../components/PageHeader';
import EmptyState from '../../components/EmptyState';
import ConfirmDialog from '../../components/ConfirmDialog';
import { getProfiles, createProfile, deleteProfile, getUnificationRules } from '../../api/api';

const PAGE_SIZE = 15;



export default function ProfilesListPage() {
  const navigate = useNavigate();
  const [profiles, setProfiles] = useState([]);
  const [pagination, setPagination] = useState({});
  const [cursorStack, setCursorStack] = useState(['']);
  const [currentPage, setCurrentPage] = useState(0);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');

  // Dialogs
  const [createOpen, setCreateOpen] = useState(false);
  const [createData, setCreateData] = useState({ user_id: '' });
  const [ruleFields, setRuleFields] = useState([]);  // active rules
  const [rulesLoading, setRulesLoading] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState(null);
  const [toast, setToast] = useState({ open: false, msg: '', severity: 'success' });

  const openCreateDialog = async () => {
    setCreateOpen(true);
    setRulesLoading(true);
    try {
      const res = await getUnificationRules();
      const allRules = Array.isArray(res) ? res : [];
      const active = allRules.filter((r) => r.is_active);
      setRuleFields(active);
      const init = { user_id: '' };
      active.forEach((r) => {
        const key = r.property_name.includes('.') ? r.property_name.split('.').pop() : r.property_name;
        init[key] = '';
      });
      setCreateData(init);
    } catch {
      setRuleFields([]);
      setCreateData({ user_id: '' });
    } finally {
      setRulesLoading(false);
    }
  };

  const load = useCallback(async (cursor = '') => {
    setLoading(true);
    try {
      const res = await getProfiles(PAGE_SIZE, cursor);
      setProfiles(res.profiles || []);
      setPagination(res.pagination || {});
    } catch (e) {
      setToast({ open: true, msg: 'Failed to load profiles', severity: 'error' });
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(); }, [load]);

  const handleNext = () => {
    if (!pagination.next_cursor) return;
    const newStack = [...cursorStack, pagination.next_cursor];
    setCursorStack(newStack);
    setCurrentPage(currentPage + 1);
    load(pagination.next_cursor);
  };

  const handlePrev = () => {
    if (currentPage <= 0) return;
    const newStack = cursorStack.slice(0, -1);
    setCursorStack(newStack);
    setCurrentPage(currentPage - 1);
    load(newStack[newStack.length - 1] || '');
  };

  const handleCreate = async () => {
    try {
      const body = { identity_attributes: {} };
      if (createData.user_id) body.user_id = createData.user_id;
      Object.entries(createData).forEach(([key, val]) => {
        if (key !== 'user_id' && val) body.identity_attributes[key] = val;
      });
      await createProfile(body);
      setCreateOpen(false);
      setCreateData({ user_id: '' });
      setToast({ open: true, msg: 'Profile created', severity: 'success' });
      load(cursorStack[cursorStack.length - 1] || '');
    } catch (e) {
      setToast({ open: true, msg: 'Failed to create profile', severity: 'error' });
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    try {
      await deleteProfile(deleteTarget);
      setDeleteTarget(null);
      setToast({ open: true, msg: 'Profile deleted', severity: 'success' });
      load(cursorStack[cursorStack.length - 1] || '');
    } catch (e) {
      setToast({ open: true, msg: 'Failed to delete profile', severity: 'error' });
    }
  };

  const filtered = search
    ? profiles.filter((p) => {
        const hay = (p.profile_id + ' ' + (p.user_id || '')).toLowerCase();
        return hay.includes(search.toLowerCase());
      })
    : profiles;

  const totalPages = pagination.count
    ? Math.ceil(pagination.count / PAGE_SIZE)
    : currentPage + (pagination.next_cursor ? 2 : 1);

  const isUnified = (p) => !!p.user_id;

  return (
    <>
      <PageHeader
        title="Profiles"
        subtitle="Manage customer profiles which has identity, behavioural and application data"
        action={
          <Button variant="contained" startIcon={<AddIcon />} onClick={openCreateDialog}>
            Add Profile
          </Button>
        }
      />

      {/* Search bar */}
      <Paper sx={{ mb: 0, borderBottom: 'none', borderBottomLeftRadius: 0, borderBottomRightRadius: 0 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', px: 2, py: 1 }}>
          <TextField
            size="small"
            placeholder="Search by profile ID"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            InputProps={{
              startAdornment: <InputAdornment position="start"><SearchIcon fontSize="small" sx={{ color: '#999' }} /></InputAdornment>,
            }}
            variant="standard"
            sx={{ flexGrow: 1, '& .MuiInput-underline:before': { borderBottom: 'none' }, '& .MuiInput-underline:after': { borderBottom: 'none' }, '& .MuiInput-underline:hover:not(.Mui-disabled):before': { borderBottom: 'none' } }}
          />
          <IconButton size="small"><FilterListIcon fontSize="small" /></IconButton>
        </Box>
      </Paper>

      {/* Column headers */}
      <Paper sx={{ borderTop: '1px solid #E8E8E8', borderRadius: 0 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', px: 2, py: 1, borderBottom: '1px solid #E8E8E8' }}>
          <Typography variant="caption" sx={{ flex: '1 1 50%', color: '#999', fontWeight: 600, textTransform: 'uppercase', fontSize: '0.7rem', letterSpacing: '0.5px' }}>
            Profile
          </Typography>
          <Typography variant="caption" sx={{ flex: '0 0 120px', color: '#999', fontWeight: 600, textTransform: 'uppercase', fontSize: '0.7rem', letterSpacing: '0.5px' }}>
            User
          </Typography>
        </Box>

        {loading ? (
          <Box sx={{ display: 'flex', justifyContent: 'center', py: 6 }}><CircularProgress size={28} /></Box>
        ) : filtered.length === 0 ? (
          <EmptyState title="No profiles found" />
        ) : (
          <List disablePadding>
            {filtered.map((p, idx) => {
              const firstChar = (p.profile_id || '').charAt(0).toUpperCase();
              const unified = isUnified(p);
              return (
                <React.Fragment key={p.profile_id}>
                  <ListItem
                    sx={{ px: 2, py: 1.5, cursor: 'pointer', '&:hover': { bgcolor: 'rgba(0,0,0,0.02)' } }}
                    onClick={() => navigate(`/profiles/${p.profile_id}`)}
                    secondaryAction={
                      <IconButton edge="end" size="small" onClick={(e) => { e.stopPropagation(); setDeleteTarget(p.profile_id); }}>
                        <DeleteIcon fontSize="small" sx={{ color: '#999' }} />
                      </IconButton>
                    }
                  >
                    <ListItemAvatar>
                      <Avatar sx={{ bgcolor: '#FF7300', width: 36, height: 36, fontSize: 14, fontWeight: 600 }}>
                        {firstChar}
                      </Avatar>
                    </ListItemAvatar>
                    <ListItemText
                      primary={
                        <Typography variant="body2" sx={{ fontFamily: 'monospace', fontSize: '0.85rem', color: '#222' }}>
                          {p.profile_id}
                        </Typography>
                      }
                      sx={{ flex: '1 1 50%' }}
                    />
                    <Box sx={{ flex: '0 0 160px', display: 'flex', flexDirection: 'row', alignItems: 'flex-start', gap: 2 }}>
                      <Typography variant="body2" color="text.secondary" sx={{ fontSize: '0.85rem' }}>
                        {p.user_id || ''}
                      </Typography>
                    {/* </Box>
                    <Box sx={{ flex: '0 0 140px', display: 'flex', justifyContent: 'flex-end', pr: 4 }}> */}
                      <Chip
                        label={unified ? 'Unified' : 'Anonymous'}
                        size="small"
                        sx={{
                          fontWeight: 500,
                          fontSize: '0.75rem',
                          bgcolor: unified ? '#FFF3E0' : '#F3E5F5',
                          color: unified ? '#E65100' : '#7B1FA2',
                          border: unified ? '1px solid #FFE0B2' : '1px solid #E1BEE7',
                        }}
                      />
                    </Box>
                  </ListItem>
                  {idx < filtered.length - 1 && <Divider component="li" />}
                </React.Fragment>
              );
            })}
          </List>
        )}

        {/* Pagination */}
        {!loading && (
          <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'flex-end', px: 2, py: 1.5, borderTop: '1px solid #E8E8E8' }}>
            <Typography variant="body2" color="text.secondary" sx={{ mr: 2 }}>
              Page {currentPage + 1} of {totalPages}
            </Typography>
            <IconButton size="small" disabled={currentPage === 0} onClick={handlePrev}>
              <NavigateBeforeIcon />
            </IconButton>
            <IconButton size="small" disabled={!pagination.next_cursor} onClick={handleNext}>
              <NavigateNextIcon />
            </IconButton>
          </Box>
        )}
      </Paper>

      {/* ─── Create dialog ──────────────────── */}
      <Dialog open={createOpen} onClose={() => setCreateOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>Create Profile</DialogTitle>
        <DialogContent>
          {rulesLoading ? (
            <Box sx={{ display: 'flex', justifyContent: 'center', py: 4 }}><CircularProgress size={24} /></Box>
          ) : (
            <Stack spacing={2} sx={{ mt: 1 }}>
              <TextField label="User ID" fullWidth size="small" value={createData.user_id || ''}
                onChange={(e) => setCreateData({ ...createData, user_id: e.target.value })}
                helperText="Optional. Associates this profile with a user."
              />
              {ruleFields.map((r) => {
                const key = r.property_name.includes('.') ? r.property_name.split('.').pop() : r.property_name;
                return (
                  <TextField
                    key={r.rule_id}
                    label={key}
                    placeholder={r.property_name}
                    fullWidth
                    size="small"
                    value={createData[key] || ''}
                    onChange={(e) => setCreateData({ ...createData, [key]: e.target.value })}
                  />
                );
              })}
              {ruleFields.length === 0 && (
                <Alert severity="info" sx={{ fontSize: '0.85rem' }}>
                  No active unification rules found. Add rules to see attribute fields here.
                </Alert>
              )}
            </Stack>
          )}
        </DialogContent>
        <DialogActions sx={{ px: 3, pb: 2 }}>
          <Button color="inherit" onClick={() => setCreateOpen(false)}>Cancel</Button>
          <Button variant="contained" onClick={handleCreate} disabled={rulesLoading}>Create</Button>
        </DialogActions>
      </Dialog>

      {/* ─── Delete confirm ─────────────────── */}
      <ConfirmDialog
        open={!!deleteTarget}
        title="Delete Profile"
        message={`Are you sure you want to permanently delete this profile?`}
        confirmLabel="Delete"
        onConfirm={handleDelete}
        onCancel={() => setDeleteTarget(null)}
      />

      {/* ─── Toast ──────────────────────────── */}
      <Snackbar open={toast.open} autoHideDuration={4000} onClose={() => setToast({ ...toast, open: false })} anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}>
        <Alert severity={toast.severity} variant="filled" onClose={() => setToast({ ...toast, open: false })}>{toast.msg}</Alert>
      </Snackbar>
    </>
  );
}
