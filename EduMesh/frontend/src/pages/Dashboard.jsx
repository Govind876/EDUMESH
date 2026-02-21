import React, { useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useApp } from "../context/app.context";
import {
  approveJoin,
  getBase,
  getJoinRequests,
  getMyRooms,
  getPeers,
  getRoomStats,
  getViralShareStatus,
  startViralShare,
  stopViralShare,
} from "../utils/api";
import { QRCodeCanvas } from "qrcode.react";

const Dashboard = () => {
  const navigate = useNavigate();
  const { classroomId, classroomTitle, classroomTeacher, autoApprove, setAutoApprove } = useApp();
  const [peers, setPeers] = useState([]);
  const [requests, setRequests] = useState([]);
  const [myRooms, setMyRooms] = useState([]);
  const [stats, setStats] = useState({
    classroomCount: 0,
    memberCount: 0,
    pendingCount: 0,
  });
  const [shareSession, setShareSession] = useState(null);
  const [shareBusy, setShareBusy] = useState(false);
  const [shareError, setShareError] = useState("");
  const [shareSSID, setShareSSID] = useState("SchoolSync-Direct");
  const [sharePassword, setSharePassword] = useState("12345678");
  const [shareArtifactPath, setShareArtifactPath] = useState("");
  const [shareBluetoothFallback, setShareBluetoothFallback] = useState(true);

  const hostname = typeof window !== "undefined" ? window.location.hostname : "localhost";
  const baseHost = `${hostname}:8080`;
  const qrData = useMemo(() => {
    if (!classroomId) return "";
    return `edumesh://join?room=${encodeURIComponent(classroomId)}&host=${encodeURIComponent(baseHost)}`;
  }, [classroomId, baseHost]);

  useEffect(() => {
    const loadPeers = async () => {
      try {
        const p = await getPeers();
        setPeers(Array.isArray(p) ? p : []);
      } catch {
        setPeers([]);
      }
    };
    loadPeers();
    const t = setInterval(loadPeers, 10000);
    return () => clearInterval(t);
  }, []);

  useEffect(() => {
    const loadHistory = async () => {
      try {
        const rooms = await getMyRooms();
        setMyRooms(Array.isArray(rooms) ? rooms : []);
      } catch {
        setMyRooms([]);
      }
    };
    loadHistory();
  }, []);

  useEffect(() => {
    const loadStats = async () => {
      try {
        const s = await getRoomStats(classroomId || "");
        setStats({
          classroomCount: s?.classroomCount || 0,
          memberCount: s?.memberCount || 0,
          pendingCount: s?.pendingCount || 0,
        });
      } catch {
        setStats({ classroomCount: 0, memberCount: 0, pendingCount: 0 });
      }
    };
    loadStats();
    const t = setInterval(loadStats, 8000);
    return () => clearInterval(t);
  }, [classroomId]);

  useEffect(() => {
    let mounted = true;
    const loadRequests = async () => {
      if (!classroomId) return;
      try {
        const list = await getJoinRequests(classroomId);
        if (!mounted) return;
        setRequests(Array.isArray(list) ? list : []);
        if (autoApprove) {
          for (const r of list) {
            await approveJoin(r.id);
          }
          const next = await getJoinRequests(classroomId);
          setRequests(Array.isArray(next) ? next : []);
        }
      } catch {
        if (mounted) setRequests([]);
      }
    };
    loadRequests();
    const t = setInterval(loadRequests, 8000);
    return () => {
      mounted = false;
      clearInterval(t);
    };
  }, [classroomId, autoApprove]);

  useEffect(() => {
    const loadShareStatus = async () => {
      try {
        const status = await getViralShareStatus();
        setShareSession(status?.active ? status?.session || null : null);
      } catch {
        setShareSession(null);
      }
    };
    loadShareStatus();
  }, []);

  const handleStartShare = async () => {
    setShareBusy(true);
    setShareError("");
    try {
      const base = new URL(getBase());
      const isLoopbackHost =
        base.hostname === "localhost" ||
        base.hostname === "127.0.0.1" ||
        base.hostname === "::1";
      const session = await startViralShare({
        ssid: shareSSID.trim(),
        password: sharePassword.trim(),
        // Let backend detect LAN IP when frontend is accessed via localhost.
        serverIp: isLoopbackHost ? "" : base.hostname,
        artifactPath: shareArtifactPath.trim(),
        enableBluetoothFallback: shareBluetoothFallback,
      });
      setShareSession(session);
    } catch (e) {
      setShareError(e?.message || "Unable to start sharing");
    } finally {
      setShareBusy(false);
    }
  };

  const handleStopShare = async () => {
    setShareBusy(true);
    setShareError("");
    try {
      await stopViralShare();
      setShareSession(null);
    } catch (e) {
      setShareError(e?.message || "Unable to stop sharing");
    } finally {
      setShareBusy(false);
    }
  };

  return (
    <div className="page">
      <div className="header">
        <h2>Classroom Dashboard</h2>
        <div className="muted">Classroom ID: {classroomId || "Not Joined"}</div>
        <div className="muted">Peers connected: {peers.length}</div>
        <div className="muted">Classrooms available: {stats.classroomCount}</div>
        <div className="muted">Students in active classroom: {stats.memberCount}</div>
      </div>

      <div className="card">
        <div className="row">
          <div>
            <div className="list-title">{classroomTitle || "No active classroom"}</div>
            <div className="muted">{classroomTeacher || "Teacher not set"}</div>
          </div>
          <div className="status-chip">{classroomId ? "Active" : "Inactive"}</div>
        </div>
        <div className="row">
          <div className="muted">Auto-approve students</div>
          <label className="switch">
            <input
              type="checkbox"
              checked={autoApprove}
              onChange={(e) => setAutoApprove(e.target.checked)}
            />
            <span className="slider" />
          </label>
        </div>
      </div>

      <div className="card">
        <h3>Previous Activity</h3>
        <div className="muted">My classrooms: {myRooms.length}</div>
        {myRooms.length === 0 && <div className="muted">No previous classrooms yet.</div>}
        <div className="list">
          {myRooms.slice(0, 5).map((r) => (
            <div key={r.id} className="list-item">
              <div>
                <div className="list-title">{r.title || r.id}</div>
                <div className="muted">{r.description || "No description"}</div>
              </div>
              <button className="btn" onClick={() => navigate(`/dashboard`)}>
                Active
              </button>
            </div>
          ))}
        </div>
      </div>

      <div className="card">
        <h3>QR Code Join</h3>
        {qrData ? (
          <>
            <div className="qr-wrap">
              <QRCodeCanvas value={qrData} size={140} />
            </div>
            <div className="muted">Share this QR so students can join offline.</div>
          </>
        ) : (
          <div className="muted">Create a classroom to show QR.</div>
        )}
      </div>

      <div className="card">
        <h3>Share SchoolSync</h3>
        <div className="muted">Zero-source distribution over local network. Scan once to connect and download.</div>
        <label className="label">Wi-Fi Direct SSID</label>
        <input
          className="input"
          value={shareSSID}
          onChange={(e) => setShareSSID(e.target.value)}
          placeholder="SchoolSync-Direct"
        />
        <label className="label">Wi-Fi Direct Password</label>
        <input
          className="input"
          value={sharePassword}
          onChange={(e) => setSharePassword(e.target.value)}
          placeholder="8+ characters"
        />
        <label className="label">Artifact Path (optional .apk)</label>
        <input
          className="input"
          value={shareArtifactPath}
          onChange={(e) => setShareArtifactPath(e.target.value)}
          placeholder="E:\\builds\\schoolsync.apk"
        />
        <div className="row">
          <div className="muted">Bluetooth OPP fallback</div>
          <label className="switch">
            <input
              type="checkbox"
              checked={shareBluetoothFallback}
              onChange={(e) => setShareBluetoothFallback(e.target.checked)}
            />
            <span className="slider" />
          </label>
        </div>
        <div className="row">
          <button className="btn primary" onClick={handleStartShare} disabled={shareBusy}>
            {shareBusy ? "Starting..." : "Share SchoolSync"}
          </button>
          <button className="btn" onClick={handleStopShare} disabled={shareBusy || !shareSession}>
            Stop Sharing
          </button>
        </div>
        {shareError && <div className="alert" style={{ marginTop: 10 }}>{shareError}</div>}
        {shareSession && (
          <div style={{ marginTop: 12 }}>
            <div className="qr-wrap">
              <QRCodeCanvas value={shareSession.downloadUrl || shareSession.qrPayload} size={180} />
            </div>
            <div className="muted">Download URL: <a href={shareSession.downloadUrl} target="_blank" rel="noreferrer">{shareSession.downloadUrl}</a></div>
            <div className="muted">Binary: {shareSession.executableName} ({Math.round((shareSession.executableSize || 0) / 1024 / 1024)} MB)</div>
            {!String(shareSession.executableName || "").toLowerCase().endsWith(".apk") && (
              <div className="alert" style={{ marginTop: 8 }}>
                Mobile phones usually cannot install `.exe`. Share an Android `.apk` using Artifact Path.
              </div>
            )}
            <div className="muted">Session expires: {shareSession.expiresAt}</div>
          </div>
        )}
      </div>

      <div className="grid">
        <button className="tile" onClick={() => navigate("/content")}>
          <div className="tile-title">Materials</div>
          <div className="muted">PDFs, videos, and uploads</div>
        </button>
        <button className="tile" onClick={() => navigate("/assignments")}>
          <div className="tile-title">Assignments</div>
          <div className="muted">Submit and review offline</div>
        </button>
        <div className="tile">
          <div className="tile-title">Progress</div>
          <div className="muted">Local progress tracking</div>
          <div className="progress">
            <div className="progress-bar" style={{ width: "35%" }} />
          </div>
        </div>
      </div>

      <div className="card">
        <h3>Join Requests</h3>
        <div className="muted">Pending: {stats.pendingCount}</div>
        {requests.length === 0 && <div className="muted">No pending requests.</div>}
        <div className="list">
          {requests.map((r) => (
            <div key={r.id} className="list-item">
              <div>
                <div className="list-title">{r.student}</div>
                <div className="muted">Requested at {r.createdAt}</div>
              </div>
              <button className="btn" onClick={() => approveJoin(r.id)}>
                Approve
              </button>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
};

export default Dashboard;
