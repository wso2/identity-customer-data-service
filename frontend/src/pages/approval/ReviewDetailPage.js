import React, { useEffect, useState } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  Box, Paper, Typography, Button, IconButton, Chip, Divider,
  Table, TableBody, TableCell, TableRow, CircularProgress, Alert,
  Snackbar, Stack, Card, CardContent, CardHeader, TextField,
  Dialog, DialogTitle, DialogContent, DialogActions,
} from '@mui/material';
import ArrowBackIcon from '@mui/icons-material/ArrowBack';
import CheckCircleIcon from '@mui/icons-material/CheckCircle';
import CancelIcon from '@mui/icons-material/Cancel';
import ScoreBar from '../../components/ScoreBar';
import { getReviewTasksByProfile, getProfile, resolveReviewTask } from '../../api';

const STATUS_COLOR = { PENDING: 'warning', APPROVED: 'success', REJECTED: 'error' };

export default function ReviewDetailPage() {
  const { profileId } = useParams();
  const navigate = useNavigate();
  const [tasks, setTasks] = useState([]);
  const [sourceProfile, setSourceProfile] = useState(null);
  const [targetProfiles, setTargetProfiles] = useState({});
  const [loading, setLoading] = useState(true);
  const [toast, setToast] = useState({ open: false, msg: '', severity: 'success' });

  // Decision dialog
  const [decisionDialog, setDecisionDialog] = useState({ open: false, taskId: null, decision: '' });
  const [notes, setNotes] = useState('');

  const load = async () => {
    setLoading(true);
    try {
      const res = await getReviewTasksByProfile(profileId);
      const taskList = res.tasks || [];
      setTasks(taskList);

      // Load source profile
      try { setSourceProfile(await getProfile(profileId)); } catch { /* ignore */ }

      // Load target profiles
      const targets = {};
      const uniqueTargets = [...new Set(taskList.map((t) => t.target_profile_id))];
      await Promise.allSettled(
        uniqueTargets.map(async (tid) => { targets[tid] = await getProfile(tid); })
      );
      setTargetProfiles(targets);
    } catch {
      setToast({ open: true, msg: 'Failed to load review tasks', severity: 'error' });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, [profileId]); // eslint-disable-line

  const handleResolve = async () => {
    try {
      await resolveReviewTask(decisionDialog.taskId, decisionDialog.decision, notes);
      setDecisionDialog({ open: false, taskId: null, decision: '' });
      setNotes('');
      setToast({ open: true, msg: `Task ${decisionDialog.decision.toLowerCase()}`, severity: 'success' });
      load();
    } catch {
      setToast({ open: true, msg: 'Failed to resolve task', severity: 'error' });
    }
  };

  const renderAttrs = (profile) => {
    const attrs = profile?.identity_attributes || {};
    return (
      <Table size="small">
        <TableBody>
          {Object.entries(attrs).map(([k, v]) => (
            <TableRow key={k}>
              <TableCell sx={{ fontWeight: 500, color: 'text.secondary', width: 140, py: 0.5 }}>{k}</TableCell>
              <TableCell sx={{ py: 0.5 }}>{typeof v === 'object' ? JSON.stringify(v) : String(v ?? '')}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    );
  };

  if (loading) return <Box sx={{ display: 'flex', justifyContent: 'center', py: 10 }}><CircularProgress /></Box>;

  return (
    <>
      {/* Header */}
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 3 }}>
        <IconButton onClick={() => navigate('/approval')}>
          <ArrowBackIcon />
        </IconButton>
        <Box>
          <Typography variant="h4">Review Tasks</Typography>
          <Typography variant="subtitle1" sx={{ fontFamily: 'monospace', fontSize: '0.8rem' }}>
            Source: {profileId}
          </Typography>
        </Box>
      </Box>

      <Stack spacing={3}>
        {tasks.length === 0 ? (
          <Alert severity="info">No review tasks for this profile.</Alert>
        ) : (
          tasks.map((task) => {
            const target = targetProfiles[task.target_profile_id];
            return (
              <Card key={task.id}>
                <CardHeader
                  title={
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                      <Typography variant="h6" sx={{ fontSize: '0.95rem' }}>
                        vs {task.target_profile_id.slice(0, 12)}…
                      </Typography>
                      <Chip label={task.status} size="small" color={STATUS_COLOR[task.status] || 'default'} />
                    </Box>
                  }
                  subheader={`Created: ${task.created_at ? new Date(task.created_at).toLocaleString() : '—'}`}
                  action={
                    task.status === 'PENDING' && (
                      <Stack direction="row" spacing={1}>
                        <Button
                          size="small"
                          variant="contained"
                          color="success"
                          startIcon={<CheckCircleIcon />}
                          onClick={() => setDecisionDialog({ open: true, taskId: task.id, decision: 'APPROVED' })}
                        >
                          Approve
                        </Button>
                        <Button
                          size="small"
                          variant="outlined"
                          color="error"
                          startIcon={<CancelIcon />}
                          onClick={() => setDecisionDialog({ open: true, taskId: task.id, decision: 'REJECTED' })}
                        >
                          Reject
                        </Button>
                      </Stack>
                    )
                  }
                />
                <Divider />
                <CardContent>
                  {/* Score */}
                  <Box sx={{ mb: 2 }}>
                    <Typography variant="body2" color="text.secondary" gutterBottom>Match Score</Typography>
                    <ScoreBar value={task.match_score} />
                  </Box>

                  {/* Score Breakdown */}
                  {task.score_breakdown && Object.keys(task.score_breakdown).length > 0 && (
                    <Box sx={{ mb: 2 }}>
                      <Typography variant="body2" color="text.secondary" gutterBottom>Score Breakdown</Typography>
                      <Stack spacing={0.5}>
                        {Object.entries(task.score_breakdown).map(([key, val]) => (
                          <Box key={key} sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                            <Typography variant="caption" sx={{ width: 280, color: 'text.secondary', fontFamily: 'monospace' }}>
                              {key}
                            </Typography>
                            <Typography variant="caption" sx={{ fontWeight: 600 }}>
                              {typeof val === 'number' ? `${Math.round(val * 100)}%` : val}
                            </Typography>
                          </Box>
                        ))}
                      </Stack>
                    </Box>
                  )}

                  {/* Side-by-side profiles */}
                  <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 2 }}>
                    <Paper variant="outlined" sx={{ p: 2 }}>
                      <Typography variant="subtitle2" gutterBottom color="primary">Source Profile</Typography>
                      {renderAttrs(sourceProfile)}
                    </Paper>
                    <Paper variant="outlined" sx={{ p: 2 }}>
                      <Typography variant="subtitle2" gutterBottom color="secondary">Target Profile</Typography>
                      {renderAttrs(target)}
                    </Paper>
                  </Box>

                  {/* Resolution info */}
                  {task.status !== 'PENDING' && (
                    <Box sx={{ mt: 2, p: 1.5, bgcolor: 'grey.50', borderRadius: 1 }}>
                      <Typography variant="caption" color="text.secondary">
                        Resolved by {task.resolved_by || '—'} at {task.resolved_at ? new Date(task.resolved_at).toLocaleString() : '—'}
                      </Typography>
                      {task.notes && <Typography variant="body2" sx={{ mt: 0.5 }}>{task.notes}</Typography>}
                    </Box>
                  )}
                </CardContent>
              </Card>
            );
          })
        )}
      </Stack>

      {/* Decision dialog */}
      <Dialog open={decisionDialog.open} onClose={() => setDecisionDialog({ open: false, taskId: null, decision: '' })} maxWidth="sm" fullWidth>
        <DialogTitle>
          {decisionDialog.decision === 'APPROVED' ? 'Approve Merge' : 'Reject Merge'}
        </DialogTitle>
        <DialogContent>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            {decisionDialog.decision === 'APPROVED'
              ? 'This will merge the two profiles. This action cannot be undone.'
              : 'This will reject the merge suggestion. The profiles will remain separate.'}
          </Typography>
          <TextField
            label="Notes (optional)"
            fullWidth
            multiline
            rows={3}
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
          />
        </DialogContent>
        <DialogActions sx={{ px: 3, pb: 2 }}>
          <Button color="inherit" onClick={() => setDecisionDialog({ open: false, taskId: null, decision: '' })}>
            Cancel
          </Button>
          <Button
            variant="contained"
            color={decisionDialog.decision === 'APPROVED' ? 'success' : 'error'}
            onClick={handleResolve}
          >
            {decisionDialog.decision === 'APPROVED' ? 'Approve' : 'Reject'}
          </Button>
        </DialogActions>
      </Dialog>

      <Snackbar open={toast.open} autoHideDuration={4000} onClose={() => setToast({ ...toast, open: false })} anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}>
        <Alert severity={toast.severity} variant="filled" onClose={() => setToast({ ...toast, open: false })}>{toast.msg}</Alert>
      </Snackbar>
    </>
  );
}
