import React from "react";
import { useNavigate } from "react-router-dom";

const Splash = () => {
  const navigate = useNavigate();
  return (
    <div className="page center">
      <div className="card splash">
        <div className="badge">Offline Mode Active</div>
        <h1 className="title">EduMesh</h1>
        <p className="subtitle">Offline Peer-to-Peer Learning Platform</p>
        <button className="btn primary" onClick={() => navigate("/role")}>
          Continue
        </button>
      </div>
    </div>
  );
};

export default Splash;
