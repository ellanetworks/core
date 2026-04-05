import React, { useCallback, useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  Box,
  Button,
  IconButton,
  InputAdornment,
  TextField,
  Typography,
  CircularProgress,
} from "@mui/material";
import { Visibility, VisibilityOff } from "@mui/icons-material";
import * as yup from "yup";
import { ValidationError } from "yup";
import { initialize } from "@/queries/initialize";
import { getStatus } from "@/queries/status";
import { useSnackbar } from "@/contexts/SnackbarContext";

const schema = yup.object().shape({
  email: yup
    .string()
    .email("Must be a valid email")
    .required("Email is required"),
  password: yup
    .string()
    .min(1, "Password is required")
    .required("Password is required"),
});

const InitializePage = () => {
  const navigate = useNavigate();
  const { showSnackbar } = useSnackbar();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [isValid, setIsValid] = useState(false);
  const [loading, setLoading] = useState(false);
  const [checkingInitialization, setCheckingInitialization] = useState(true);

  useEffect(() => {
    const checkInitialization = async () => {
      try {
        const status = await getStatus();
        if (status?.initialized) {
          navigate("/dashboard");
        } else {
          setCheckingInitialization(false);
        }
      } catch (err) {
        console.error("Failed to fetch system status:", err);
        showSnackbar("Failed to check system initialization.", "error");
        setCheckingInitialization(false);
      }
    };

    checkInitialization();
  }, [navigate]);

  const validateField = async (field: string, value: string) => {
    try {
      const fieldSchema = yup.reach(schema, field) as yup.Schema<unknown>;
      await fieldSchema.validate(value);
      setErrors((prev) => ({ ...prev, [field]: "" }));
    } catch (err) {
      if (err instanceof ValidationError) {
        setErrors((prev) => ({ ...prev, [field]: err.message }));
      }
    }
  };

  const handleBlur = (field: string) => {
    setTouched((prev) => ({ ...prev, [field]: true }));
  };

  const validateForm = useCallback(async () => {
    try {
      await schema.validate({ email, password }, { abortEarly: false });
      setIsValid(true);
    } catch {
      setIsValid(false);
    }
  }, [email, password]);

  useEffect(() => {
    validateForm();
  }, [email, password, validateForm]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);

    try {
      const resp = await initialize(email, password);
      navigate("/dashboard", { state: { token: resp.token } });
    } catch (err) {
      const error = err as Error;
      showSnackbar(error.message, "error");
    } finally {
      setLoading(false);
    }
  };

  if (checkingInitialization) {
    return (
      <Box
        sx={{
          height: "100vh",
          display: "flex",
          justifyContent: "center",
          alignItems: "center",
        }}
      >
        <CircularProgress />
      </Box>
    );
  }

  return (
    <Box
      sx={{
        minHeight: "100vh",
        display: "flex",
        justifyContent: "center",
        alignItems: "center",
        p: 2,
      }}
    >
      <Box
        sx={{
          width: "100%",
          maxWidth: 360,
          display: "flex",
          flexDirection: "column",
          gap: 2,
          border: "1px solid",
          borderColor: "divider",
          borderRadius: 2,
          p: 3,
          boxShadow: 2,
        }}
      >
        <form onSubmit={handleSubmit} noValidate>
          <Typography variant="h5" textAlign="center" gutterBottom>
            Initialize Ella Core
          </Typography>
          <Typography variant="body1" sx={{ marginBottom: 2 }}>
            Create the first user
          </Typography>

          <TextField
            label="Email"
            type="email"
            variant="outlined"
            value={email}
            onChange={(e) => {
              setEmail(e.target.value);
              validateField("email", e.target.value);
            }}
            onBlur={() => handleBlur("email")}
            error={!!errors.email && touched.email}
            helperText={touched.email ? errors.email : ""}
            fullWidth
            margin="normal"
            required
            autoFocus
            autoComplete="email"
          />
          <TextField
            label="Password"
            type={showPassword ? "text" : "password"}
            variant="outlined"
            margin="normal"
            value={password}
            onChange={(e) => {
              setPassword(e.target.value);
              validateField("password", e.target.value);
            }}
            onBlur={() => handleBlur("password")}
            error={!!errors.password && touched.password}
            helperText={touched.password ? errors.password : ""}
            fullWidth
            required
            autoComplete="new-password"
            slotProps={{
              input: {
                endAdornment: (
                  <InputAdornment position="end">
                    <IconButton
                      aria-label={
                        showPassword ? "Hide password" : "Show password"
                      }
                      onClick={() => setShowPassword((prev) => !prev)}
                      edge="end"
                    >
                      {showPassword ? <VisibilityOff /> : <Visibility />}
                    </IconButton>
                  </InputAdornment>
                ),
              },
            }}
          />

          <Button
            type="submit"
            variant="contained"
            color="success"
            fullWidth
            sx={{ mt: 2 }}
            disabled={!isValid || loading}
          >
            {loading ? <CircularProgress size={24} /> : "Create"}
          </Button>
        </form>
      </Box>
    </Box>
  );
};

export default InitializePage;
