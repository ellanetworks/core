import React from "react";
import { Box, Button, Card, CardContent, Typography } from "@mui/material";

interface UserPasswordCardProps {
  onChangePassword: () => void;
  disabled?: boolean;
}

const UserPasswordCard: React.FC<UserPasswordCardProps> = ({
  onChangePassword,
  disabled,
}) => {
  return (
    <Card variant="outlined" sx={{ height: "100%" }}>
      <CardContent>
        <Typography variant="h6" sx={{ mb: 1.5 }}>
          Password
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          Keep this account secure by using a strong password and updating it
          periodically.
        </Typography>
        <Button
          variant="contained"
          onClick={onChangePassword}
          disabled={disabled}
        >
          Change Password
        </Button>
      </CardContent>
    </Card>
  );
};

export default UserPasswordCard;
