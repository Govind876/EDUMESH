import React from "react";
import { Backdrop, CircularProgress } from "@mui/material";

const LoadingScreen = () => {
  return (
    <Backdrop
      open={true}
      sx={{
        color: "#fff",
        zIndex: (theme) => theme.zIndex.drawer + 1,
        flexDirection: "column",
      }}
    >
      <CircularProgress color="inherit" />
    </Backdrop>
  );
};

export default LoadingScreen;
