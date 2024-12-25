"use client";

import React from "react";
import { Box, Typography, Button } from "@mui/material";

interface EmptyStateProps {
    primaryText: string;
    secondaryText: string;
    buttonText: string;
    onCreate: () => void;
}

const EmptyState: React.FC<EmptyStateProps> = ({ primaryText, secondaryText, buttonText, onCreate }) => {
    return (
        <Box
            sx={{
                display: "flex",
                flexDirection: "column",
                alignItems: "flex-start", // Keep text left-aligned
                justifyContent: "flex-start", // Align content towards the top
                margin: "0 auto", // Center horizontally
                padding: 2,
                width: "50%", // Set a width for better layout
                marginTop: 4, // Add some margin from the top
            }}
        >
            <Typography variant="h4" gutterBottom>
                {primaryText}
            </Typography>
            <Typography variant="body1" gutterBottom>
                {secondaryText}
            </Typography>
            <Button
                variant="contained"
                color="success"
                onClick={onCreate}
            >
                {buttonText}
            </Button>
        </Box>
    );
};

export default EmptyState;
