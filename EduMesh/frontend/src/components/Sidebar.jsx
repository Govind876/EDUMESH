import {
  Box,
  Divider,
  List,
  ListItem,
  ListItemIcon,
  ListItemText,
  Toolbar,
  Typography,
} from "@mui/material";
import { AddCircleOutlineOutlined, Login } from "@mui/icons-material";
import { useNavigate } from "react-router-dom";

const Sidebar = ({ drawerWidth }) => {
  const navigate = useNavigate();

  return (
    <Box sx={{ width: drawerWidth - 1 }}>
      <Toolbar />
      <Divider />
      <List>
        <ListItem button onClick={() => navigate("/join-room")}>
          <ListItemIcon>
            <Login />
          </ListItemIcon>
          <ListItemText
            primary={
              <Typography
                noWrap
                sx={{ overflow: "hidden", textOverflow: "ellipsis" }}
              >
                Join Classroom
              </Typography>
            }
          />
        </ListItem>
        <ListItem button onClick={() => navigate("/create-room")}>
          <ListItemIcon>
            <AddCircleOutlineOutlined />
          </ListItemIcon>
          <ListItemText
            primary={
              <Typography
                noWrap
                sx={{ overflow: "hidden", textOverflow: "ellipsis" }}
              >
                Create Classroom
              </Typography>
            }
          />
        </ListItem>
      </List>
    </Box>
  );
};

export default Sidebar;
