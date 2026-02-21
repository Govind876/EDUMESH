import React, { useState } from "react";
import {
  Box,
  Button,
  TextField,
  Typography,
  Paper,
  Avatar,
} from "@mui/material";
import LockOutlinedIcon from "@mui/icons-material/LockOutlined";
import { Navigate } from "react-router-dom";
import { useProfile } from "../context/profile.context";
import { addUser } from "../utils/api";

const LoginPage = () => {
  const [name, setName] = useState("");
  const { profile, signIn } = useProfile();
  const handleLoginSubmit = async (e) => {
    e.preventDefault();
    try {
      await addUser(name.trim());
      signIn(name.trim());
    } catch (error) {
      console.error("Login failed:", error.message);
    }
  };

  return profile != null ? (
    <Navigate to={"/"} />
  ) : (
    <Paper
      sx={{
        height: "100vh",
        display: "flex",
        justifyContent: "center",
        alignItems: "center",
        borderRadius: "0%",
      }}
    >
      <Paper elevation={3} sx={{ p: 4, maxWidth: 400, width: "100%" }}>
        <Box
          sx={{
            display: "flex",
            flexDirection: "column",
            alignItems: "center",
          }}
        >
          <Avatar sx={{ m: 1, bgcolor: "primary.main" }}>
            <LockOutlinedIcon />
          </Avatar>
          <Typography component="h1" variant="h5">
            Sign in
          </Typography>
        </Box>

        <Box component="form" onSubmit={handleLoginSubmit} sx={{ mt: 3 }}>
          <TextField
            margin="normal"
            required
            fullWidth
            label="Your Name"
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
          <Button
            type="submit"
            fullWidth
            variant="contained"
            sx={{ mt: 2, mb: 1 }}
          >
            Sign In
          </Button>
        </Box>
      </Paper>
    </Paper>
  );
};

export default LoginPage;
