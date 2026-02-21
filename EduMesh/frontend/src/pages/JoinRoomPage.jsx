import React, { useState } from "react";
import {
  Box,
  Button,
  Container,
  TextField,
  Typography,
  CircularProgress,
  Alert,
  CssBaseline,
} from "@mui/material";
import { useNavigate } from "react-router-dom";
import { joinRoom } from "../utils/api"; // adjust path if needed
import { useProfile } from "../context/profile.context";

const JoinRoomPage = () => {
  const [code, setCode] = useState("");
  const [loading, setLoading] = useState(false);
  const [status, setStatus] = useState({ type: "", message: "" });
  const navigate = useNavigate(); // Initialize navigate function
  const { profile } = useProfile();

  const handleJoin = async () => {
    setLoading(true);
    setStatus({ type: "", message: "" });

    try {
      const uname = profile?.email || profile?.name;
      if (!uname) throw new Error("User not logged in");

      await joinRoom(uname, code.trim());
      setStatus({ type: "success", message: "Successfully joined the room!" });
    } catch (err) {
      setStatus({ type: "error", message: err.message });
    } finally {
      setLoading(false);
    }
  };

  return (
    <>
      <Container maxWidth="sm">
        <CssBaseline />
        <Box sx={{ mt: 10, display: "flex", flexDirection: "column", gap: 2 }}>
          <Typography variant="h4" align="center">
            Join a Room
          </Typography>

          <TextField
            label="Room Code"
            variant="outlined"
            value={code}
            onChange={(e) => setCode(e.target.value)}
            fullWidth
          />

          <Button
            variant="contained"
            color="primary"
            onClick={handleJoin}
            disabled={loading || !code.trim()}
          >
            {loading ? <CircularProgress size={24} /> : "Join Room"}
          </Button>

          {status.message && (
            <Alert severity={status.type}>{status.message}</Alert>
          )}

          {/* Back to Home Button */}
          <Button
            variant="outlined"
            color="secondary"
            onClick={() => navigate("/")}
          >
            Back to Home
          </Button>
        </Box>
      </Container>
    </>
  );
};

export default JoinRoomPage;
