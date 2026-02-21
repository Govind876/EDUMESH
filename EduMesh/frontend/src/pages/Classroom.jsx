import React, { useEffect, useState } from "react";
import {
  AppBar,
  Avatar,
  Box,
  CircularProgress,
  CssBaseline,
  Divider,
  Drawer,
  IconButton,
  List,
  ListItem,
  ListItemIcon,
  ListItemText,
  Paper,
  Toolbar,
  Typography,
  useMediaQuery,
  useTheme,
} from "@mui/material";
import MenuIcon from "@mui/icons-material/Menu";
import ClassIcon from "@mui/icons-material/Class";
import PostAddIcon from "@mui/icons-material/PostAdd";
import AnnouncementIcon from "@mui/icons-material/Announcement";
import { useParams, useNavigate } from "react-router-dom";
import {
  getAnnouncements,
  getBase,
  getPosts,
  getRoomDetails,
} from "../utils/api";
import { NavigateBefore } from "@mui/icons-material";

const drawerWidth = 240;

const ClassroomHomePage = () => {
  const { room } = useParams();
  const [mobileOpen, setMobileOpen] = useState(false);
  const [announcements, setAnnouncements] = useState([]);
  const [posts, setPosts] = useState([]);
  const [roomDetails, setRoomDetails] = useState(null);
  const [loading, setLoading] = useState(true);

  const theme = useTheme();
  const isMobile = useMediaQuery(theme.breakpoints.down("md"));
  const navigate = useNavigate(); // useNavigate hook for navigation

  const handleDrawerToggle = () => {
    setMobileOpen(!mobileOpen);
  };

  useEffect(() => {
    const fetchData = async () => {
      try {
        const [announcementsData, postsData, roomInfo] = await Promise.all([
          getAnnouncements(room),
          getPosts(room),
          getRoomDetails(room),
        ]);
        setAnnouncements(announcementsData);
        setPosts(postsData);
        setRoomDetails(roomInfo);
      } catch (err) {
        console.error("Error fetching data", err);
      } finally {
        setLoading(false);
      }
    };

    fetchData();
  }, [room]);

  const handleMenuClick = (path) => {
    navigate(path);
  };

  const drawer = (
    <Box>
      <Typography variant="h6" sx={{ margin: 2 }} gutterBottom>
        Classroom Menu
      </Typography>
      <List>
        {/* Create Post menu item */}
        <ListItem
          button
          onClick={() => handleMenuClick(`/upload-post/${room}`)}
        >
          <ListItemIcon>
            <PostAddIcon />
          </ListItemIcon>
          <ListItemText primary="Post" />
        </ListItem>

        {/* Create Announcement menu item */}
        <ListItem
          button
          onClick={() => handleMenuClick(`/share-announcement/${room}`)}
        >
          <ListItemIcon>
            <AnnouncementIcon />
          </ListItemIcon>
          <ListItemText primary="Announce" />
        </ListItem>

        <ListItem button onClick={() => handleMenuClick(`/`)}>
          <ListItemIcon>
            <NavigateBefore />
          </ListItemIcon>
          <ListItemText primary="Back" />
        </ListItem>
      </List>
    </Box>
  );

  if (loading) {
    return (
      <>
        <CssBaseline />
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
      </>
    );
  }

  return (
    <>
      <CssBaseline />
      <Box
        sx={{
          display: "flex",
          bgcolor: "background.default",
          color: "text.primary",
        }}
      >
        {/* Mobile AppBar */}
        {isMobile && (
          <AppBar
            position="fixed"
            sx={{ zIndex: (theme) => theme.zIndex.drawer + 1 }}
          >
            <Toolbar>
              <IconButton
                color="inherit"
                edge="start"
                onClick={handleDrawerToggle}
                sx={{ mr: 2 }}
              >
                <MenuIcon />
              </IconButton>
              <Typography variant="h6" noWrap>
                {roomDetails?.title || `Room ${room}`}
              </Typography>
            </Toolbar>
          </AppBar>
        )}

        {/* Drawer */}
        <Box
          component="nav"
          sx={{ width: { md: drawerWidth }, flexShrink: { md: 0 } }}
          aria-label="classroom sidebar"
        >
          {isMobile ? (
            <Drawer
              variant="temporary"
              open={mobileOpen}
              onClose={handleDrawerToggle}
              ModalProps={{ keepMounted: true }}
              sx={{
                display: { xs: "block", md: "none" },
                "& .MuiDrawer-paper": { width: drawerWidth },
              }}
            >
              {drawer}
            </Drawer>
          ) : (
            <Drawer
              variant="permanent"
              sx={{
                display: { xs: "none", md: "block" },
                "& .MuiDrawer-paper": {
                  width: drawerWidth,
                  boxSizing: "border-box",
                },
              }}
              open
            >
              {drawer}
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
          {isMobile && <Toolbar />} {/* Spacer for AppBar */}
          {/* Header */}
          <Paper
            elevation={3}
            sx={{
              p: 3,
              display: "flex",
              alignItems: "center",
              gap: 2,
              bgcolor: "primary.main",
              color: "primary.contrastText",
              flexWrap: "wrap",
            }}
          >
            <Avatar sx={{ bgcolor: "white", color: "primary.main" }}>
              <ClassIcon />
            </Avatar>
            <Box>
              <Typography variant="h5" noWrap>
                {roomDetails?.title || `Room ${room}`}
              </Typography>
              <Typography variant="body2" noWrap>
                {roomDetails?.description || "This is your classroom hub."}
              </Typography>
              <Typography variant="caption" noWrap>
                {roomDetails?.teacher || "Instructor"}
              </Typography>
            </Box>
          </Paper>
          {/* Content in Flexbox */}
          <Box
            sx={{
              display: "flex",
              flexDirection: { xs: "column", md: "row" },
              gap: 3,
              mt: 3,
            }}
          >
            {/* Announcements */}
            <Box sx={{ flex: 1 }}>
              <Typography variant="h6" gutterBottom>
                Announcements
              </Typography>
              <Paper variant="outlined">
                <List disablePadding>
                  {announcements.map((a, i) => (
                    <React.Fragment key={i}>
                      <ListItem alignItems="flex-start">
                        <ListItemText
                          primary={a.title}
                          secondary={
                            <>
                              <Typography variant="body2">
                                {a.description}
                              </Typography>
                              <Typography
                                variant="caption"
                                color="text.secondary"
                              >
                                Posted by {a.author}
                              </Typography>
                            </>
                          }
                        />
                      </ListItem>
                      <Divider component="li" />
                    </React.Fragment>
                  ))}
                </List>
              </Paper>
            </Box>

            {/* Posts */}
            <Box sx={{ flex: 1 }}>
              <Typography variant="h6" gutterBottom>
                Posts
              </Typography>
              <Paper variant="outlined">
                <List disablePadding>
                  {posts.map((p, i) => (
                    <React.Fragment key={i}>
                      <ListItem alignItems="flex-start">
                        <ListItemText
                          primary={p.content}
                          secondary={
                            <>
                              {p.fileUrl && (
                                <Typography variant="body2" color="primary">
                                  <a
                                    href={getBase() + p.fileUrl}
                                    target="_blank"
                                    rel="noreferrer"
                                  >
                                    View Attachment
                                  </a>
                                </Typography>
                              )}
                              <Typography
                                variant="caption"
                                color="text.secondary"
                              >
                                Posted by {p.author}
                              </Typography>
                            </>
                          }
                        />
                      </ListItem>
                      <Divider component="li" />
                    </React.Fragment>
                  ))}
                </List>
              </Paper>
            </Box>
          </Box>
        </Box>
      </Box>
    </>
  );
};

export default ClassroomHomePage;
