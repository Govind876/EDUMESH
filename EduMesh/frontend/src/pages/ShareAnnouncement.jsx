import React, { useState } from "react";
import {
  Box,
  Button,
  Container,
  TextField,
  Typography,
  Paper,
  CssBaseline,
} from "@mui/material";
import { useParams, useNavigate } from "react-router-dom";
import { createAnnouncement } from "../utils/api";

const ShareAnnouncement = () => {
  const { room } = useParams(); // Extract room code from URL
  const navigate = useNavigate();
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e) => {
    e.preventDefault();
    setError("");
    setLoading(true);

    if (!title || !description) {
      setError("Title and description are required.");
      setLoading(false);
      return;
    }

    try {
      await createAnnouncement({ code: room, title, description });
      navigate(`/classroom/${room}`); // redirect back to room
    } catch (err) {
      setError(err.message || "Failed to share announcement");
    } finally {
      setLoading(false);
    }
  };

  return (
    <>
      <CssBaseline />
      <Container maxWidth="sm" sx={{ mt: 4 }}>
        <Paper elevation={3} sx={{ p: 4 }}>
          <Typography variant="h5" gutterBottom>
            Share an Announcement
          </Typography>
          <Box component="form" onSubmit={handleSubmit} noValidate>
            <TextField
              fullWidth
              label="Title"
              margin="normal"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
            />
            <TextField
              fullWidth
              label="Description"
              multiline
              rows={4}
              margin="normal"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
            {error && (
              <Typography color="error" variant="body2" sx={{ mt: 1 }}>
                {error}
              </Typography>
            )}
            <Button
              type="submit"
              variant="contained"
              color="primary"
              sx={{ mt: 2 }}
              disabled={loading}
            >
              {loading ? "Sharing..." : "Share Announcement"}
            </Button>
          </Box>
        </Paper>
      </Container>
    </>
  );
};

export default ShareAnnouncement;
