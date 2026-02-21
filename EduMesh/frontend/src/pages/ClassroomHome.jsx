import React, { useState, useEffect } from "react";
import {
  AppBar,
  Toolbar,
  Typography,
  Avatar,
  IconButton,
  Drawer,
  List,
  ListItem,
  ListItemText,
  Box,
  Divider,
  Button,
  useMediaQuery,
  CssBaseline,
} from "@mui/material";
import { useTheme } from "@mui/material/styles";
import MenuIcon from "@mui/icons-material/Menu";
import Sidebar from "../components/Sidebar";
import { Logout } from "@mui/icons-material";
import { useProfile } from "../context/profile.context";
import { getMyRooms, getPeers } from "../utils/api"; // make sure this exists
import { useNavigate } from "react-router-dom";

const drawerWidth = 240;

const ClassroomHome = () => {
  const theme = useTheme();
  const isMobile = useMediaQuery(theme.breakpoints.down("md"));
  const [mobileOpen, setMobileOpen] = useState(false);
  const { profile, signOut } = useProfile();
  const [classes, setClasses] = useState([]);
  const [peerCount, setPeerCount] = useState(0);
  const navigate = useNavigate();

  const handleDrawerToggle = () => {
    setMobileOpen((prev) => !prev);
  };

  useEffect(() => {
    if (profile?.email) {
      getMyRooms(profile.email)
        .then((data) => setClasses(Array.isArray(data) ? data : []))
        .catch((err) => {
          console.error("Failed to fetch joined rooms:", err);
          setClasses([]);
        });
    } else {
      setClasses([]);
    }
  }, [profile]);

  useEffect(() => {
    let mounted = true;
    const loadPeers = async () => {
      try {
        const peers = await getPeers();
        if (mounted) setPeerCount(Array.isArray(peers) ? peers.length : 0);
      } catch {
        if (mounted) setPeerCount(0);
      }
    };
    loadPeers();
    const t = setInterval(loadPeers, 10000);
    return () => {
      mounted = false;
      clearInterval(t);
    };
  }, []);

  return (
    <Box
      sx={{
        display: "flex",
        bgcolor: "background.default",
        color: "text.primary",
      }}
    >
      <CssBaseline />

      {/* AppBar */}
      <AppBar position="fixed" sx={{ zIndex: theme.zIndex.drawer + 1 }}>
        <Toolbar>
          {isMobile && (
            <IconButton
              color="inherit"
              edge="start"
              onClick={handleDrawerToggle}
              sx={{ mr: 2 }}
            >
              <MenuIcon />
            </IconButton>
          )}
          <Typography
            variant="h6"
            noWrap
            sx={{ flexGrow: 1, overflow: "hidden", textOverflow: "ellipsis" }}
          >
            My Classrooms
          </Typography>
          <Typography variant="caption" sx={{ mr: 2, opacity: 0.8 }}>
            Peers online: {peerCount}
          </Typography>
          <Avatar sx={{ width: 32, height: 32, mr: 1 }}>
            {(profile?.name || "U")[0]}
          </Avatar>
          <IconButton color="inherit">
            <Logout onClick={signOut} />
          </IconButton>
        </Toolbar>
      </AppBar>

      {/* Drawer */}
      <Box
        component="nav"
        sx={{ width: { md: drawerWidth }, flexShrink: { md: 0 } }}
        aria-label="mailbox folders"
      >
        {/* Mobile Drawer */}
        {isMobile && (
          <Drawer
            variant="temporary"
            open={mobileOpen}
            onClose={handleDrawerToggle}
            ModalProps={{ keepMounted: true }}
            sx={{
              "& .MuiDrawer-paper": {
                boxSizing: "border-box",
                width: drawerWidth,
              },
            }}
          >
            <Sidebar drawerWidth={drawerWidth} />
          </Drawer>
        )}
        {/* Desktop Static Drawer */}
        {!isMobile && (
          <Drawer
            variant="permanent"
            sx={{
              "& .MuiDrawer-paper": {
                width: drawerWidth,
                boxSizing: "border-box",
              },
            }}
            open
          >
            <Sidebar />
          </Drawer>
        )}
      </Box>

      {/* Main Content */}
      <Box
        component="main"
        sx={{
          flexGrow: 1,
          p: 3,
          width: { md: `calc(100% - ${drawerWidth}px)` },
        }}
      >
        <Toolbar />
        <List>
          {classes.map((course) => (
            <React.Fragment key={course.id}>
              <ListItem
                alignItems="flex-start"
                secondaryAction={
                  <Button
                    variant="outlined"
                    size="small"
                    onClick={() => {
                      navigate("/classroom/" + course.id);
                    }}
                  >
                    Open
                  </Button>
                }
              >
                <ListItemText
                  primary={
                    <Typography
                      variant="subtitle1"
                      fontWeight="bold"
                      noWrap
                      sx={{ overflow: "hidden", textOverflow: "ellipsis" }}
                    >
                      {course.title}
                    </Typography>
                  }
                  secondary={
                    <>
                      <Typography
                        variant="body2"
                        color="text.secondary"
                        noWrap
                        sx={{ overflow: "hidden", textOverflow: "ellipsis" }}
                      >
                        {course.description}
                      </Typography>
                      <Typography
                        variant="caption"
                        color="text.secondary"
                        noWrap
                        sx={{ overflow: "hidden", textOverflow: "ellipsis" }}
                      >
                        {course.teacher}
                      </Typography>
                    </>
                  }
                />
              </ListItem>
              <Divider component="li" />
            </React.Fragment>
          ))}
        </List>
      </Box>
    </Box>
  );
};

export default ClassroomHome;
