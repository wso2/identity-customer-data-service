import React, { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  Box, Paper, Typography, Button, IconButton, Chip, Divider, TextField,
  Table, TableBody, TableCell, TableHead, TableRow, CircularProgress, Alert,
  Snackbar, Stack, Avatar, Tabs, Tab, Tooltip,
} from '@mui/material';
import ArrowBackIosNewIcon from '@mui/icons-material/ArrowBackIosNew';
import EditIcon from '@mui/icons-material/Edit';
import SaveIcon from '@mui/icons-material/Save';
import CancelIcon from '@mui/icons-material/Close';
import ContentCopyIcon from '@mui/icons-material/ContentCopy';
import DeleteIcon from '@mui/icons-material/DeleteOutline';
import { getProfile, patchProfile, deleteProfile } from '../../api';
import ConfirmDialog from '../../components/ConfirmDialog';

export default function ProfileDetailPage() {
  const { id } = useParams();
  const navigate = useNavigate();
  const [profile, setProfile] = useState(null);
  const [loading, setLoading] = useState(true);
  const [tabIdx, setTabIdx] = useState(0);
  const [editing, setEditing] = useState(false);
  const [editData, setEditData] = useState({});
  const [deleteTarget, setDeleteTarget] = useState(null);
  const [toast, setToast] = useState({ open: false, msg: '', severity: 'success' });

  useEffect(() => {
    (async () => {
      setLoading(true);
      try {
        const res = await getProfile(id);
        setProfile(res);
      } catch {
        setToast({ open: true, msg: 'Failed to load profile', severity: 'error' });
      } finally {
        setLoading(false);
      }
    })();
  }, [id]);

  const startEdit = () => {
    setEditData({ ...(profile.identity_attributes || {}) });
    setEditing(true);
  };

  const saveEdit = async () => {
    try {
      await patchProfile(id, { identity_attributes: editData });
      setProfile({ ...profile, identity_attributes: editData });
      setEditing(false);
      setToast({ open: true, msg: 'Profile updated', severity: 'success' });
    } catch {
      setToast({ open: true, msg: 'Failed to update', severity: 'error' });
    }
  };

  const copyId = () => {
    navigator.clipboard.writeText(id);
    setToast({ open: true, msg: 'Profile ID copied', severity: 'info' });
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    try {
      await deleteProfile(deleteTarget);
      setDeleteTarget(null);
      setToast({ open: true, msg: 'Profile deleted', severity: 'success' });
      navigate('/profiles');
    } catch {
      setToast({ open: true, msg: 'Failed to delete profile', severity: 'error' });
    }
  };

  if (loading) return <Box sx={{ display: 'flex', justifyContent: 'center', py: 10 }}><CircularProgress /></Box>;
  if (!profile) return <Alert severity="error">Profile not found</Alert>;

  const identityAttrs = profile.identity_attributes || {};
  const traits = profile.traits || {};
  const appData = profile.application_data || {};
  const meta = profile.meta || {};
  const firstChar = (id || '').charAt(0).toUpperCase();

  // Filter out traits with null/empty values
  const nonEmptyTraits = Object.entries(traits).filter(([, v]) => v != null && v !== '');

  // Unified profiles (merged_from)
  const unifiedProfiles = profile.merged_from || [];

  return (
    <>
      {/* Back link */}
      <Box
        onClick={() => navigate('/profiles')}
        sx={{ display: 'inline-flex', alignItems: 'center', gap: 0.5, cursor: 'pointer', mb: 2, color: 'text.secondary', '&:hover': { color: 'primary.main' } }}
      >
        <ArrowBackIosNewIcon sx={{ fontSize: 12 }} />
        <Typography variant="body2">Go back to Profiles</Typography>
      </Box>

      {/* Profile header: avatar + ID */}
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, mb: 3 }}>
        <Avatar sx={{ bgcolor: '#FF7300', width: 52, height: 52, fontSize: 22, fontWeight: 600 }}>
          {firstChar}
        </Avatar>
        <Box>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <Typography variant="h5" sx={{ fontFamily: 'monospace', fontWeight: 600 }}>{id}</Typography>
            <Tooltip title="Copy profile ID">
              <IconButton size="small" onClick={copyId}><ContentCopyIcon sx={{ fontSize: 16 }} /></IconButton>
            </Tooltip>
          </Box>
          {profile.user_id && (
            <Typography variant="body2" color="text.secondary">User: {profile.user_id}</Typography>
          )}
        </Box>
      </Box>

      {/* Tabs */}
      <Tabs value={tabIdx} onChange={(_, v) => setTabIdx(v)} sx={{ mb: 3, borderBottom: '1px solid #E8E8E8' }}>
        <Tab label="General" />
        <Tab label="Unified Profiles" />
      </Tabs>

      {/* ─── General tab ────────────────────── */}
      {tabIdx === 0 && (
        <Stack spacing={3}>
          {/* Read-only metadata fields */}
          <Paper sx={{ p: 3 }}>
            <Stack spacing={3}>
              {/* Profile ID - full width */}
              <ReadOnlyField label="Profile ID" value={id} />
              {/* User ID, Created, Updated - 3 columns */}
              <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 3 }}>
                <ReadOnlyField label="User ID" value={profile.user_id || identityAttrs.userid} />
                <ReadOnlyField label="Created Date" value={meta.created_at ? new Date(meta.created_at).toLocaleString() : '—'} />
                <ReadOnlyField label="Updated Date" value={meta.updated_at ? new Date(meta.updated_at).toLocaleString() : '—'} />
              </Box>
              {/* Location - full width */}
              <ReadOnlyField label="Location" value={meta.location} />
            </Stack>
          </Paper>

          {/* Profile Data: Identity Attributes */}
          <Box>
            <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 1 }}>
              <Box>
                <Typography variant="h6">Profile Data</Typography>
                <Typography variant="body2" color="text.secondary">
                  Identity attributes associated with this profile
                </Typography>
              </Box>
              {editing ? (
                <Stack direction="row" spacing={1}>
                  <Button size="small" startIcon={<CancelIcon />} onClick={() => setEditing(false)}>Cancel</Button>
                  <Button size="small" variant="contained" startIcon={<SaveIcon />} onClick={saveEdit}>Save</Button>
                </Stack>
              ) : (
                <Button size="small" startIcon={<EditIcon />} onClick={startEdit}>Edit</Button>
              )}
            </Box>
            <Paper>
              {editing ? (
                <Stack spacing={2} sx={{ p: 3 }}>
                  {Object.entries(editData).map(([key, val]) => (
                    <TextField
                      key={key}
                      label={key}
                      value={typeof val === 'object' ? JSON.stringify(val) : val ?? ''}
                      onChange={(e) => setEditData({ ...editData, [key]: e.target.value })}
                      size="small"
                      fullWidth
                    />
                  ))}
                </Stack>
              ) : (
                <Table size="small">
                  <TableHead>
                    <TableRow>
                      <TableCell sx={{ fontWeight: 600, color: '#999', textTransform: 'uppercase', fontSize: '0.7rem' }}>Attribute</TableCell>
                      <TableCell sx={{ fontWeight: 600, color: '#999', textTransform: 'uppercase', fontSize: '0.7rem' }}>Value</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {Object.entries(identityAttrs).map(([key, val]) => (
                      <TableRow key={key}>
                        <TableCell sx={{ fontWeight: 500, width: 220, color: 'text.secondary' }}>{key}</TableCell>
                        <TableCell>{typeof val === 'object' ? JSON.stringify(val) : String(val ?? '—')}</TableCell>
                      </TableRow>
                    ))}
                    {Object.keys(identityAttrs).length === 0 && (
                      <TableRow><TableCell colSpan={2} sx={{ color: 'text.secondary', py: 3, textAlign: 'center' }}>No identity attributes</TableCell></TableRow>
                    )}
                  </TableBody>
                </Table>
              )}
            </Paper>
          </Box>

          {/* Traits (only if non-empty) */}
          {nonEmptyTraits.length > 0 && (
            <Box>
              <Typography variant="h6" sx={{ mb: 1 }}>Traits</Typography>
              <Paper>
                <Table size="small">
                  <TableBody>
                    {nonEmptyTraits.map(([key, val]) => (
                      <TableRow key={key}>
                        <TableCell sx={{ fontWeight: 500, width: 220, color: 'text.secondary' }}>{key}</TableCell>
                        <TableCell>{typeof val === 'object' ? JSON.stringify(val) : String(val)}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </Paper>
            </Box>
          )}

          {/* Application Data */}
          {Object.keys(appData).length > 0 && Object.entries(appData).map(([appId, data]) => (
            <Box key={appId}>
              <Typography variant="h6" sx={{ mb: 1 }}>Application: {appId}</Typography>
              <Paper>
                <Table size="small">
                  <TableBody>
                    {Object.entries(data || {}).map(([k, v]) => (
                      <TableRow key={k}>
                        <TableCell sx={{ fontWeight: 500, width: 220, color: 'text.secondary' }}>{k}</TableCell>
                        <TableCell>{typeof v === 'object' ? JSON.stringify(v) : String(v ?? '—')}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </Paper>
            </Box>
          ))}
        </Stack>
      )}

      {/* ─── Unified Profiles tab ───────────── */}
      {tabIdx === 1 && (
        <Paper sx={{ p: 3 }}>
          {profile.merged_to && (() => {
            const mergedToId = typeof profile.merged_to === 'object' ? profile.merged_to.profile_id : profile.merged_to;
            return (
              <Alert severity="info" sx={{ mb: 2 }}>
                This profile has been merged into{' '}
                <Chip
                  label={mergedToId}
                  size="small"
                  color="primary"
                  variant="outlined"
                  onClick={() => navigate(`/profiles/${mergedToId}`)}
                  sx={{ fontFamily: 'monospace', cursor: 'pointer' }}
                />
              </Alert>
            );
          })()}
          {unifiedProfiles.length > 0 ? (
            <>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                Profiles that have been unified into this profile
              </Typography>
              <Table size="small">
                <TableHead>
                  <TableRow>
                    <TableCell sx={{ fontWeight: 600, color: '#999', textTransform: 'uppercase', fontSize: '0.7rem' }}>Profile ID</TableCell>
                    <TableCell sx={{ fontWeight: 600, color: '#999', textTransform: 'uppercase', fontSize: '0.7rem' }}>Reason</TableCell>
                    <TableCell sx={{ fontWeight: 600, color: '#999', textTransform: 'uppercase', fontSize: '0.7rem' }} align="right">Actions</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {unifiedProfiles.map((entry) => {
                    const pid = typeof entry === 'object' ? entry.profile_id : entry;
                    const reason = typeof entry === 'object' ? entry.reason : '';
                    return (
                      <TableRow key={pid} hover sx={{ cursor: 'pointer' }} onClick={() => navigate(`/profiles/${pid}`)}>
                        <TableCell sx={{ fontFamily: 'monospace', fontSize: '0.85rem' }}>{pid}</TableCell>
                        <TableCell>
                          {reason && <Chip label={reason.replace(/_/g, ' ')} size="small" variant="outlined" />}
                        </TableCell>
                        <TableCell align="right" onClick={(e) => e.stopPropagation()}>
                          <Tooltip title="View profile">
                            <Button size="small" onClick={() => navigate(`/profiles/${pid}`)}>View</Button>
                          </Tooltip>
                          <Tooltip title="Delete profile">
                            <IconButton size="small" onClick={() => setDeleteTarget(pid)}>
                              <DeleteIcon fontSize="small" sx={{ color: '#999' }} />
                            </IconButton>
                          </Tooltip>
                        </TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            </>
          ) : (
            <Typography variant="body2" color="text.secondary">
              No unified profiles. When other profiles are merged into this one, they will appear here.
            </Typography>
          )}
        </Paper>
      )}

      {/* Delete confirm */}
      <ConfirmDialog
        open={!!deleteTarget}
        title="Delete Profile"
        message={`Are you sure you want to permanently delete this profile?`}
        confirmLabel="Delete"
        onConfirm={handleDelete}
        onCancel={() => setDeleteTarget(null)}
      />

      <Snackbar open={toast.open} autoHideDuration={3000} onClose={() => setToast({ ...toast, open: false })} anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}>
        <Alert severity={toast.severity} variant="filled" onClose={() => setToast({ ...toast, open: false })}>{toast.msg}</Alert>
      </Snackbar>
    </>
  );
}

function ReadOnlyField({ label, value }) {
  return (
    <Box>
      <Typography variant="caption" color="text.secondary" sx={{ fontWeight: 600, fontSize: '0.7rem', textTransform: 'uppercase', letterSpacing: '0.5px' }}>
        {label}
      </Typography>
      <Box sx={{ bgcolor: '#F5F5F5', borderRadius: 1, px: 1.5, py: 1, mt: 0.5 }}>
        <Typography variant="body2" sx={{ fontFamily: label === 'Profile ID' ? 'monospace' : 'inherit' }}>
          {value || '—'}
        </Typography>
      </Box>
    </Box>
  );
}
