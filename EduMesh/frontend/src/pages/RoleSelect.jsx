import React, { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useProfile } from "../context/profile.context";
import { useApp } from "../context/app.context";

const RoleSelect = () => {
  const navigate = useNavigate();
  const { signIn } = useProfile();
  const { setRole } = useApp();
  const [name, setName] = useState("");

  const chooseRole = (role) => {
    const finalName = name.trim() || (role === "teacher" ? "Teacher" : "Student");
    signIn(finalName);
    setRole(role);
    navigate("/discover");
  };

  return (
    <div className="page">
      <div className="header">
        <h2>Choose Your Role</h2>
        <p className="muted">
          No login required. Your name is stored locally on this device.
        </p>
      </div>
      <div className="card">
        <label className="label">Your Name</label>
        <input
          className="input"
          placeholder="Enter your name"
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
      </div>
      <div className="grid">
        <button className="role-btn" onClick={() => chooseRole("teacher")}>
          <div className="role-title">Teacher</div>
          <div className="role-desc">Create and manage a classroom</div>
        </button>
        <button className="role-btn" onClick={() => chooseRole("student")}>
          <div className="role-title">Student</div>
          <div className="role-desc">Join and learn offline</div>
        </button>
      </div>
    </div>
  );
};

export default RoleSelect;
