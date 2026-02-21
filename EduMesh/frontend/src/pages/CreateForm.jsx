import React, { useState } from "react";
import {
  Box,
  Button,
  Container,
  TextField,
  Typography,
  CssBaseline,
  Paper,
  Alert,
} from "@mui/material";
import { useNavigate } from "react-router-dom";
import { createRoom } from "./../utils/api"; // adjust import path as needed
import { useProfile } from "../context/profile.context";

export default function CreateRoomPage() {
  const [title, setTitle] = useState("");
  const { profile } = useProfile();
  const teacher = profile?.name || "Instructor";
  const [description, setDescription] = useState("");
  const [successCode, setSuccessCode] = useState(null);
  const [error, setError] = useState("");
  const navigate = useNavigate(); // 👈 Add navigate

  const handleSubmit = async (e) => {
    e.preventDefault();
    setError("");
    setSuccessCode(null);
    try {
      const result = await createRoom({ title, teacher, description });
      setSuccessCode(result.code);
    } catch (err) {
      setError(err.message);
    }
  };

  return (
    <>
      <CssBaseline />
      <Container component="main" maxWidth="sm">
        <Paper elevation={3} sx={{ mt: 8, p: 4, borderRadius: 4 }}>
          <Typography component="h1" variant="h4" align="center" gutterBottom>
            Create a Room
          </Typography>
          <Box
            component="form"
            onSubmit={handleSubmit}
            noValidate
            sx={{ mt: 3 }}
          >
            <TextField
              fullWidth
              required
              label="Room Title"
              margin="normal"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
            />
            <TextField
              fullWidth
              required
              label="Description"
              margin="normal"
              multiline
              rows={4}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />

            <Button
              type="submit"
              fullWidth
              variant="contained"
              sx={{ mt: 3, py: 1.5 }}
            >
              Create Room
            </Button>

            {successCode && (
              <Alert severity="success" sx={{ mt: 2 }}>
                Room created successfully! Code: <strong>{successCode}</strong>
              </Alert>
            )}
            {error && (
              <Alert severity="error" sx={{ mt: 2 }}>
                {error}
              </Alert>
            )}
          </Box>

          {/* 👇 Back to Home Button */}
          <Button
            variant="outlined"
            fullWidth
            sx={{ mt: 2 }}
            onClick={() => navigate("/")}
          >
            Back to Home
          </Button>
        </Paper>
      </Container>
    </>
  );
}
