import React from 'react';
import { Box, Typography } from '@mui/material';

export default function PageHeader({ title, subtitle, action }) {
  return (
    <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 3 }}>
      <Box>
        <Typography variant="h4">{title}</Typography>
        {subtitle && <Typography variant="subtitle1" sx={{ mt: 0.5 }}>{subtitle}</Typography>}
      </Box>
      {action && <Box>{action}</Box>}
    </Box>
  );
}
