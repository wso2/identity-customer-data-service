import React from 'react';
import { Box, Typography, LinearProgress } from '@mui/material';

export default function ScoreBar({ value, label, height = 8, showLabel = true }) {
  const pct = Math.min(Math.max((value ?? 0) * 100, 0), 100);
  const color = pct >= 80 ? 'success' : pct >= 50 ? 'warning' : 'error';

  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, width: '100%' }}>
      <Box sx={{ flexGrow: 1 }}>
        <LinearProgress
          variant="determinate"
          value={pct}
          color={color}
          sx={{ height, borderRadius: height / 2, bgcolor: 'grey.100' }}
        />
      </Box>
      {showLabel && (
        <Typography variant="body2" sx={{ fontWeight: 600, minWidth: 48, textAlign: 'right' }}>
          {label ?? `${pct.toFixed(0)}%`}
        </Typography>
      )}
    </Box>
  );
}
