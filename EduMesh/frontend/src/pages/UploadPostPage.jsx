import React, { useState } from "react";
import {
  Box,
  Button,
  Container,
  TextField,
  Typography,
  Input,
  Alert,
  CssBaseline,
} from "@mui/material";
import { uploadPost } from "../utils/api"; // adjust path if needed
import { useParams } from "react-router-dom";

const UploadPostPage = () => {
  const { room } = useParams(); // room code from URL
  const [content, setContent] = useState("");
  const [file, setFile] = useState(null);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");

  const handleUpload = async (e) => {
    e.preventDefault();

    if (!content || !file) {
      setError("Please provide post content and select a file.");
      return;
    }

    try {
      await uploadPost({ code: room, content, file });
      setMessage("Post uploaded successfully.");
      setError("");
      setContent("");
      setFile(null);
    } catch (err) {
      setError(err.message || "Failed to upload post.");
      setMessage("");
    }
  };

  return (
    <>
      <CssBaseline />
      <Container maxWidth="sm">
        <Box sx={{ mt: 4 }}>
          <Typography variant="h5" gutterBottom>
            Upload Post to Room: {room}
          </Typography>
          {message && <Alert severity="success">{message}</Alert>}
          {error && <Alert severity="error">{error}</Alert>}
          <form onSubmit={handleUpload} encType="multipart/form-data">
            <TextField
              label="Post Content"
              fullWidth
              multiline
              rows={4}
              margin="normal"
              value={content}
              onChange={(e) => setContent(e.target.value)}
            />
            <Input
              type="file"
              fullWidth
              onChange={(e) => setFile(e.target.files[0])}
              sx={{ mt: 2, mb: 2 }}
            />
            <Button type="submit" variant="contained" color="primary" fullWidth>
              Upload
            </Button>
          </form>
        </Box>
      </Container>
    </>
  );
};

export default UploadPostPage;
