import React, { useEffect, useState } from 'react';
import {
  Paper, Box, Typography, Button, TextField, Stack, Slider,
  Card, CardContent, Chip, CircularProgress, Alert, Snackbar,
  Table, TableBody, TableCell, TableRow, Divider, InputAdornment,
} from '@mui/material';
import SearchIcon from '@mui/icons-material/Search';
import ClearIcon from '@mui/icons-material/Clear';
import PersonIcon from '@mui/icons-material/PersonOutline';
import PageHeader from '../../components/PageHeader';
import EmptyState from '../../components/EmptyState';
import ScoreBar from '../../components/ScoreBar';
import { getUnificationRules, searchProfiles } from '../../api';

export default function SearchPage() {
  const [rules, setRules] = useState([]);
  const [formValues, setFormValues] = useState({});
  const [threshold, setThreshold] = useState(0.3);
  const [maxResults, setMaxResults] = useState(10);
  const [results, setResults] = useState(null);
  const [loading, setLoading] = useState(false);
  const [rulesLoading, setRulesLoading] = useState(true);
  const [toast, setToast] = useState({ open: false, msg: '', severity: 'success' });

  // Load active unification rules for form fields
  useEffect(() => {
    (async () => {
      try {
        const res = await getUnificationRules();
        const allRules = Array.isArray(res) ? res : [];
        const active = allRules.filter((r) => r.is_active);
        setRules(active);
        const init = {};
        active.forEach((r) => { init[r.property_name] = ''; });
        setFormValues(init);
      } catch {
        setToast({ open: true, msg: 'Failed to load rules', severity: 'error' });
      } finally {
        setRulesLoading(false);
      }
    })();
  }, []);

  const handleSearch = async () => {
    // Build identity_attributes from form values
    const identity_attributes = {};
    Object.entries(formValues).forEach(([prop, val]) => {
      if (!val) return;
      // Strip scope prefix: "identity_attributes.emailaddress" → "emailaddress"
      const key = prop.includes('.') ? prop.split('.').pop() : prop;
      identity_attributes[key] = val;
    });

    if (Object.keys(identity_attributes).length === 0) {
      setToast({ open: true, msg: 'Enter at least one attribute', severity: 'warning' });
      return;
    }

    setLoading(true);
    setResults(null);
    try {
      const res = await searchProfiles({ identity_attributes, threshold, max_results: maxResults });
      setResults(res);
    } catch {
      setToast({ open: true, msg: 'Search failed', severity: 'error' });
    } finally {
      setLoading(false);
    }
  };

  const handleClear = () => {
    const cleared = {};
    rules.forEach((r) => { cleared[r.property_name] = ''; });
    setFormValues(cleared);
    setResults(null);
  };

  const matches = results?.matches || [];

  return (
    <>
      <PageHeader title="Search" subtitle="Search for matching profiles using identity resolution" />

      {/* ─── Search Form ──────────────────── */}
      <Paper sx={{ p: 3, mb: 3 }}>
        {rulesLoading ? (
          <Box sx={{ display: 'flex', justifyContent: 'center', py: 4 }}><CircularProgress size={24} /></Box>
        ) : rules.length === 0 ? (
          <Alert severity="info">No active unification rules. Create rules to enable search.</Alert>
        ) : (
          <>
            <Typography variant="subtitle2" color="text.secondary" gutterBottom>
              Search Attributes
            </Typography>
            <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))', gap: 2, mb: 3 }}>
              {rules.map((r) => (
                <TextField
                  key={r.rule_id}
                  label={r.property_name.includes('.') ? r.property_name.split('.').pop() : r.property_name}
                  placeholder={r.property_name}
                  size="small"
                  value={formValues[r.property_name] || ''}
                  onChange={(e) => setFormValues({ ...formValues, [r.property_name]: e.target.value })}
                  onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
                />
              ))}
            </Box>

            <Divider sx={{ my: 2 }} />

            <Box sx={{ display: 'flex', alignItems: 'center', gap: 3, flexWrap: 'wrap' }}>
              <Box sx={{ width: 240 }}>
                <Typography variant="caption" color="text.secondary">
                  Threshold: {threshold.toFixed(2)}
                </Typography>
                <Slider
                  size="small"
                  value={threshold}
                  onChange={(_, v) => setThreshold(v)}
                  min={0} max={1} step={0.05}
                  valueLabelDisplay="auto"
                />
              </Box>
              <TextField
                label="Max Results"
                type="number"
                size="small"
                value={maxResults}
                onChange={(e) => setMaxResults(parseInt(e.target.value) || 10)}
                sx={{ width: 120 }}
                slotProps={{ htmlInput: { min: 1, max: 100 } }}
              />
              <Box sx={{ flexGrow: 1 }} />
              <Button variant="outlined" startIcon={<ClearIcon />} onClick={handleClear}>Clear</Button>
              <Button variant="contained" startIcon={<SearchIcon />} onClick={handleSearch} disabled={loading}>
                {loading ? 'Searching…' : 'Search'}
              </Button>
            </Box>
          </>
        )}
      </Paper>

      {/* ─── Results ──────────────────────── */}
      {results && (
        <>
          <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 2 }}>
            <Typography variant="h6">
              Results
              <Typography component="span" variant="body2" color="text.secondary" sx={{ ml: 1 }}>
                {matches.length} match{matches.length !== 1 ? 'es' : ''}
                {results.processing_time_ms != null && ` · ${results.processing_time_ms}ms`}
              </Typography>
            </Typography>
          </Box>

          {matches.length === 0 ? (
            <Paper sx={{ p: 4 }}>
              <EmptyState icon={PersonIcon} title="No matches found" subtitle="Try different attributes or lower the threshold" />
            </Paper>
          ) : (
            <Stack spacing={2}>
              {matches.map((m, idx) => (
                <Card key={m.candidate_id || idx}>
                  <CardContent>
                    <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 2 }}>
                      <Box sx={{ flexGrow: 1 }}>
                        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1 }}>
                          <Typography variant="subtitle2" sx={{ fontFamily: 'monospace', fontSize: '0.8rem' }}>
                            {m.candidate_id?.slice(0, 12)}…
                          </Typography>
                          {m.user_id && <Chip label={m.user_id} size="small" variant="outlined" />}
                        </Box>

                        {/* Matched attributes */}
                        <Table size="small" sx={{ mb: 1 }}>
                          <TableBody>
                            {m.attributes && Object.entries(m.attributes).map(([k, v]) => (
                              <TableRow key={k}>
                                <TableCell sx={{ py: 0.25, fontWeight: 500, color: 'text.secondary', width: 200, border: 0 }}>
                                  {k.includes('.') ? k.split('.').pop() : k}
                                </TableCell>
                                <TableCell sx={{ py: 0.25, border: 0 }}>
                                  {typeof v === 'object' ? JSON.stringify(v) : String(v ?? '')}
                                </TableCell>
                              </TableRow>
                            ))}
                          </TableBody>
                        </Table>
                      </Box>

                      {/* Score */}
                      <Box sx={{ minWidth: 180, textAlign: 'right' }}>
                        <Typography variant="caption" color="text.secondary">Final Score</Typography>
                        <ScoreBar value={m.final_score} />
                      </Box>
                    </Box>

                    {/* Score breakdown */}
                    {m.score_breakdown && Object.keys(m.score_breakdown).length > 0 && (
                      <Box sx={{ mt: 1.5, pt: 1.5, borderTop: '1px solid', borderColor: 'divider' }}>
                        <Typography variant="caption" color="text.secondary" gutterBottom>Breakdown</Typography>
                        <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(200px, 1fr))', gap: 1, mt: 0.5 }}>
                          {Object.entries(m.score_breakdown).map(([k, v]) => (
                            <Box key={k} sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                              <Typography variant="caption" sx={{ fontFamily: 'monospace', width: 120, flexShrink: 0 }}>
                                {k.includes('.') ? k.split('.').pop() : k}
                              </Typography>
                              <ScoreBar value={v} height={4} />
                            </Box>
                          ))}
                        </Box>
                      </Box>
                    )}
                  </CardContent>
                </Card>
              ))}
            </Stack>
          )}
        </>
      )}

      <Snackbar open={toast.open} autoHideDuration={4000} onClose={() => setToast({ ...toast, open: false })} anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}>
        <Alert severity={toast.severity} variant="filled" onClose={() => setToast({ ...toast, open: false })}>{toast.msg}</Alert>
      </Snackbar>
    </>
  );
}
