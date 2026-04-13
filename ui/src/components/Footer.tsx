import { Box, Container, Typography, Link } from "@mui/material";
import { MAX_WIDTH } from "@/utils/layout";

export default function Footer() {
  return (
    <Box
      component="footer"
      sx={{
        mt: "auto",
        borderTop: 1,
        borderColor: "divider",
        py: 2,
        bgcolor: "background.paper",
      }}
    >
      <Container maxWidth={false} sx={{ maxWidth: MAX_WIDTH }}>
        <Typography
          variant="body2"
          color="textSecondary"
          sx={{
            display: "flex",
            flexWrap: "wrap",
            alignItems: "center",
            gap: "6px",
          }}
        >
          © 2026 Ella Networks Inc.
          <span>·</span>
          <Link
            href="https://ellanetworks.com"
            target="_blank"
            rel="noopener noreferrer"
            color="textSecondary"
            underline="hover"
          >
            ellanetworks.com
          </Link>
        </Typography>
      </Container>
    </Box>
  );
}
