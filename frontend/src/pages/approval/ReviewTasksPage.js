import React, { useEffect, useState, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Paper, Box, Typography, Chip, CircularProgress, Alert, Snackbar,
  Table, TableBody, TableCell, TableContainer, TableHead, TableRow,
  IconButton, Tooltip, Button, TextField, InputAdornment, MenuItem,
} from '@mui/material';
import OpenInNewIcon from '@mui/icons-material/OpenInNew';
import SearchIcon from '@mui/icons-material/Search';
import FilterListIcon from '@mui/icons-material/FilterList';
import PageHeader from '../../components/PageHeader';
import EmptyState from '../../components/EmptyState';
import ScoreBar from '../../components/ScoreBar';
import { getReviewTasks } from '../../api';

const STATUS_COLOR = {
  PENDING: 'warning',
  APPROVED: 'success',
  REJECTED: 'error',
};

export default function ReviewTasksPage() {
  const navigate = useNavigate();
  const [tasks, setTasks] = useState([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');
  const [statusFilter, setStatusFilter] = useState('ALL');
  const [toast, setToast] = useState({ open: false, msg: '', severity: 'success' });

  useEffect(() => {
    (async () => {
      setLoading(true);
      try {
        const res = await getReviewTasks(200);
        setTasks(res.tasks || []);
      } catch {
        setToast({ open: true, msg: 'Failed to load tasks', severity: 'error' });
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  const grouped = useMemo(() => {
    const groups = {};
    tasks.forEach((t) => {
      const key = t.source_profile_id;
      if (!groups[key]) groups[key] = { source: key, tasks: [], bestScore: 0 };
      groups[key].tasks.push(t);
      if (t.match_score > groups[key].bestScore) groups[key].bestScore = t.match_score;
    });
    return Object.values(groups).sort((a, b) => b.bestScore - a.bestScore);
  }, [tasks]);

  const filtered = grouped.filter((g) => {
    if (filter && !g.source.toLowerCase().includes(filter.toLowerCase())) return false;
    if (statusFilter !== 'ALL') {
      const hasStatus = g.tasks.some((t) => t.status === statusFilter);
      if (!hasStatus) return false;
    }
    return true;
  });

  const pendingCount = tasks.filter((t) => t.status === 'PENDING').length;

  return (
    <>
      <PageHeader
        title="Approval"
        subtitle={`${pendingCount} pending review task${pendingCount !== 1 ? 's' : ''}`}
      />

      {/* Filters */}
      <Box sx={{ display: 'flex', gap: 2, mb: 2 }}>
        <TextField
          size="small"
          placeholder="Filter by profile ID…"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          slotProps={{
            input: {
              startAdornment: <InputAdornment position="start"><SearchIcon fontSize="small" /></InputAdornment>,
            },
          }}
          sx={{ width: 280 }}
        />
        <TextField
          select size="small" value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
          sx={{ width: 160 }}
          slotProps={{
            input: {
              startAdornment: <InputAdornment position="start"><FilterListIcon fontSize="small" /></InputAdornment>,
            },
          }}
        >
          <MenuItem value="ALL">All Statuses</MenuItem>
          <MenuItem value="PENDING">Pending</MenuItem>
          <MenuItem value="APPROVED">Approved</MenuItem>
          <MenuItem value="REJECTED">Rejected</MenuItem>
        </TextField>
      </Box>

      {/* Tasks table */}
      <Paper>
        <TableContainer>
          <Table>
            <TableHead>
              <TableRow>
                <TableCell>Source Profile</TableCell>
                <TableCell>Candidates</TableCell>
                <TableCell>Best Score</TableCell>
                <TableCell>Status</TableCell>
                <TableCell align="right">Actions</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell colSpan={5} align="center" sx={{ py: 6 }}><CircularProgress size={24} /></TableCell>
                </TableRow>
              ) : filtered.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5}><EmptyState title="No review tasks" subtitle="Identity resolution tasks will appear here" /></TableCell>
                </TableRow>
              ) : (
                filtered.map((g) => {
                  const statuses = [...new Set(g.tasks.map((t) => t.status))];
                  return (
                    <TableRow key={g.source} hover sx={{ cursor: 'pointer' }} onClick={() => navigate(`/approval/${g.source}`)}>
                      <TableCell>
                        <Typography variant="body2" sx={{ fontFamily: 'monospace', fontSize: '0.8rem' }}>
                          {g.source.slice(0, 12)}…
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Chip label={`${g.tasks.length} candidate${g.tasks.length !== 1 ? 's' : ''}`} size="small" variant="outlined" />
                      </TableCell>
                      <TableCell sx={{ width: 200 }}>
                        <ScoreBar value={g.bestScore} />
                      </TableCell>
                      <TableCell>
                        {statuses.map((s) => (
                          <Chip key={s} label={s} size="small" color={STATUS_COLOR[s] || 'default'} sx={{ mr: 0.5 }} />
                        ))}
                      </TableCell>
                      <TableCell align="right" onClick={(e) => e.stopPropagation()}>
                        <Tooltip title="Review">
                          <IconButton size="small" onClick={() => navigate(`/approval/${g.source}`)}>
                            <OpenInNewIcon fontSize="small" />
                          </IconButton>
                        </Tooltip>
                      </TableCell>
                    </TableRow>
                  );
                })
              )}
            </TableBody>
          </Table>
        </TableContainer>
      </Paper>

      <Snackbar open={toast.open} autoHideDuration={4000} onClose={() => setToast({ ...toast, open: false })} anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}>
        <Alert severity={toast.severity} variant="filled" onClose={() => setToast({ ...toast, open: false })}>{toast.msg}</Alert>
      </Snackbar>
    </>
  );
}
