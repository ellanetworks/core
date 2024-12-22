import { createTheme } from "@mui/material/styles";

const theme = createTheme({
    typography: {
        fontFamily: "Source Code Pro, monospace",
        fontWeightMedium: 500, // Set Medium as the default weight
        fontWeightRegular: 500, // Optional: make regular text Medium as well
        body1: {
            fontWeight: 500, // Applies to body text
        },
        h1: {
            fontWeight: 500, // Applies to headers
        },
        h2: {
            fontWeight: 500,
        },
        h3: {
            fontWeight: 500,
        },
    },
});

export default theme;
