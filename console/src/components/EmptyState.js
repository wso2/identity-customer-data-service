import React from 'react';
import { Box, Typography } from '@mui/material';
import InboxIcon from '@mui/icons-material/InboxOutlined';

export default function EmptyState({ icon: Icon = InboxIcon, title = 'No data', subtitle = '' }) {
  return (
    <Box sx={{ textAlign: 'center', py: 8, color: 'text.secondary' }}>
      <Icon sx={{ fontSize: 56, mb: 2, opacity: 0.4 }} />
      <Typography variant="h6" gutterBottom>{title}</Typography>
      {subtitle && <Typography variant="body2">{subtitle}</Typography>}
    </Box>
  );
}
