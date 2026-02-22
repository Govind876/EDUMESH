import React, { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useApp } from "../context/app.context";
import {
  approveJoin,
  getBase,
  getJoinRequests,
  getMyRooms,
  getLocalNetworkInfo,
  getPeers,
  getRoomDetails,
  getRoomStats,
  getViralShareStatus,
  listMembers,
  startViralShare,
  stopViralShare,
  uploadViralArtifact,
} from "../utils/api";
import { QRCodeCanvas } from "qrcode.react";

const Dashboard = () => {
  const isLoopbackHost = (value) => {
    const host = String(value || "").trim().toLowerCase();
    return host === "localhost" || host === "127.0.0.1" || host === "::1";
  };
  const navigate = useNavigate();
  const { classroomId, classroomTitle, classroomTeacher, autoApprove, setAutoApprove, setClassroom } = useApp();
  const [peers, setPeers] = useState([]);
  const [requests, setRequests] = useState([]);
  const [members, setMembers] = useState([]);
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
  const [shareArtifactName, setShareArtifactName] = useState("");
  const [shareUploadingArtifact, setShareUploadingArtifact] = useState(false);
  const [shareBluetoothFallback, setShareBluetoothFallback] = useState(true);
  const [networkHost, setNetworkHost] = useState("");
  const [mobileWebHost, setMobileWebHost] = useState(() => {
    if (typeof window === "undefined") return "";
    const saved = String(window.localStorage.getItem("edumesh.mobile_host") || "").trim();
    if (saved && !isLoopbackHost(saved)) return saved;
    const host = String(window.location.hostname || "").trim();
    if (!host || isLoopbackHost(host)) {
      return "";
    }
    return host;
  });
  const [approvingRequestId, setApprovingRequestId] = useState("");
  const artifactInputRef = useRef(null);

  const hostname = typeof window !== "undefined" ? window.location.hostname : "localhost";
  const backendURL = useMemo(() => {
    try {
      const base = new URL(getBase());
      const resolvedHost =
        base.hostname === "localhost" || base.hostname === "127.0.0.1" || base.hostname === "::1"
          ? hostname
          : base.hostname;
      const port = base.port || (base.protocol === "https:" ? "443" : "80");
      return `${base.protocol}//${resolvedHost}:${port}`;
    } catch {
      return `http://${hostname}:8080`;
    }
  }, [hostname]);
  const backendPort = useMemo(() => {
    try {
      const u = new URL(backendURL);
      return u.port || (u.protocol === "https:" ? "443" : "80");
    } catch {
      return "8080";
    }
  }, [backendURL]);
  const effectiveMobileHost = useMemo(() => {
    const host = String(mobileWebHost || "").trim();
    if (host && !isLoopbackHost(host)) return host;
    const net = String(networkHost || "").trim();
    if (net && !isLoopbackHost(net)) return net;
    const lower = String(hostname || "").toLowerCase();
    if (lower && !isLoopbackHost(lower)) {
      return hostname;
    }
    const peer = (Array.isArray(peers) ? peers : []).find((p) => {
      const candidate = String(p?.host || p?.ip || "").trim().toLowerCase();
      return candidate && !isLoopbackHost(candidate);
    });
    const fromPeer = String(peer?.host || peer?.ip || "").trim();
    if (fromPeer && !isLoopbackHost(fromPeer)) return fromPeer;
    // Last fallback keeps QR visible; user is warned that localhost is local-only.
    return "localhost";
  }, [mobileWebHost, networkHost, hostname, peers]);
  const mobileWebUrl = useMemo(() => {
    if (!effectiveMobileHost) return "";
    return `http://${effectiveMobileHost}:${backendPort}`;
  }, [effectiveMobileHost, backendPort]);
  const qrRoomId = useMemo(() => {
    if (classroomId) return classroomId;
    return String(myRooms?.[0]?.id || "").trim();
  }, [classroomId, myRooms]);
  const qrData = useMemo(() => {
    if (!qrRoomId || !mobileWebUrl) return "";
    return `${mobileWebUrl}/discover?room=${encodeURIComponent(qrRoomId)}`;
  }, [qrRoomId, mobileWebUrl]);
  const mobileHostLoopback = useMemo(() => {
    const host = String(effectiveMobileHost || "").trim().toLowerCase();
    return host === "localhost" || host === "127.0.0.1" || host === "::1";
  }, [effectiveMobileHost]);

  useEffect(() => {
    if (typeof window === "undefined") return;
    const value = String(mobileWebHost || "").trim();
    if (value && !isLoopbackHost(value)) {
      window.localStorage.setItem("edumesh.mobile_host", value);
      return;
    }
    window.localStorage.removeItem("edumesh.mobile_host");
  }, [mobileWebHost]);

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
    let mounted = true;
    const loadNetworkInfo = async () => {
      try {
        const info = await getLocalNetworkInfo();
        if (!mounted) return;
        const host = String(info?.host || "").trim();
        setNetworkHost(host);
      } catch {
        if (mounted) setNetworkHost("");
      }
    };
    loadNetworkInfo();
    const t = setInterval(loadNetworkInfo, 15000);
    return () => {
      mounted = false;
      clearInterval(t);
    };
  }, []);

  useEffect(() => {
    let mounted = true;
    const loadHistory = async () => {
      try {
        const rooms = await getMyRooms();
        if (!mounted) return;
        setMyRooms(Array.isArray(rooms) ? rooms : []);
      } catch {
        if (mounted) setMyRooms([]);
      }
    };
    loadHistory();
    const t = setInterval(loadHistory, 10000);
    return () => {
      mounted = false;
      clearInterval(t);
    };
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
    const loadMembers = async () => {
      if (!classroomId) {
        if (mounted) setMembers([]);
        return;
      }
      try {
        const result = await listMembers(classroomId);
        if (!mounted) return;
        setMembers(Array.isArray(result?.members) ? result.members : []);
      } catch {
        if (mounted) setMembers([]);
      }
    };
    loadMembers();
    const t = setInterval(loadMembers, 5000);
    return () => {
      mounted = false;
      clearInterval(t);
    };
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
      const artifactPath = shareArtifactPath.trim();
      if (!artifactPath) {
        throw new Error("Artifact path is required.");
      }
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
        artifactPath,
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

  const handlePickArtifact = () => {
    if (shareBusy || shareUploadingArtifact) return;
    artifactInputRef.current?.click();
  };

  const handleArtifactFileChange = async (e) => {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file) return;
    setShareError("");
    setShareUploadingArtifact(true);
    try {
      const uploaded = await uploadViralArtifact(file);
      setShareArtifactPath(String(uploaded?.artifactPath || "").trim());
      setShareArtifactName(String(uploaded?.fileName || file.name || "").trim());
    } catch (err) {
      setShareError(err?.message || "Unable to upload artifact");
    } finally {
      setShareUploadingArtifact(false);
    }
  };

  const activateRoom = async (roomId) => {
    const id = String(roomId || "").trim();
    if (!id) return;
    try {
      const info = await getRoomDetails(id);
      setClassroom(id, info?.title, info?.teacher);
      navigate("/dashboard");
    } catch (e) {
      setShareError(e?.message || "Unable to activate classroom");
    }
  };

  const handleApproveRequest = async (request) => {
    const requestId = String(request?.id || "").trim();
    if (!requestId) return;
    setShareError("");
    setApprovingRequestId(requestId);
    try {
      await approveJoin(requestId, request?.roomId, request?.student);
      setRequests((prev) => prev.filter((r) => r.id !== requestId));
      if (classroomId) {
        const result = await listMembers(classroomId);
        setMembers(Array.isArray(result?.members) ? result.members : []);
      }
      const s = await getRoomStats(classroomId || "");
      setStats({
        classroomCount: s?.classroomCount || 0,
        memberCount: s?.memberCount || 0,
        pendingCount: s?.pendingCount || 0,
      });
    } catch (e) {
      setShareError(e?.message || "Unable to approve join request");
    } finally {
      setApprovingRequestId("");
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
              <button className="btn" onClick={() => activateRoom(r.id)}>
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
            <div className="muted">QR URL: <a href={qrData} target="_blank" rel="noreferrer">{qrData}</a></div>
            {!classroomId && qrRoomId && (
              <div className="muted">Using last classroom: {qrRoomId}</div>
            )}
          </>
        ) : (
          <div className="muted">
            {!qrRoomId
              ? "Create or activate a classroom to show QR."
              : "Enter Mobile Access Host (LAN IP) below to show QR."}
          </div>
        )}
      </div>

      <div className="card">
        <h3>Share SchoolSync</h3>
        <div className="muted">Scan QR and open SchoolSync on mobile browser.</div>
        <label className="label">Mobile Access Host (LAN IP)</label>
        <input
          className="input"
          value={mobileWebHost}
          onChange={(e) => setMobileWebHost(e.target.value)}
          placeholder="192.168.x.x"
        />
        {mobileWebUrl ? (
          <div style={{ marginTop: 10 }}>
            <div className="qr-wrap">
              <QRCodeCanvas value={mobileWebUrl} size={180} />
            </div>
            <div className="muted">Mobile Web URL: <a href={mobileWebUrl} target="_blank" rel="noreferrer">{mobileWebUrl}</a></div>
            <div className="muted">On phone, open URL then browser menu -> Add to Home Screen (install as app).</div>
            {mobileHostLoopback && (
              <div className="alert" style={{ marginTop: 8 }}>
                `localhost` works only on this computer. Use your laptop LAN IP for other phones.
              </div>
            )}
          </div>
        ) : (
          <div className="muted" style={{ marginTop: 8 }}>Enter LAN IP to generate mobile QR.</div>
        )}
        <div className="muted" style={{ marginTop: 12 }}>Optional: direct artifact distribution mode</div>
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
        <label className="label">Artifact Path</label>
        <input
          className="input"
          value={shareArtifactPath}
          onChange={(e) => setShareArtifactPath(e.target.value)}
          placeholder="E:\\builds\\schoolsync-artifact.bin"
        />
        <input
          ref={artifactInputRef}
          type="file"
          style={{ display: "none" }}
          onChange={handleArtifactFileChange}
        />
        <div className="row">
          <button className="btn" onClick={handlePickArtifact} disabled={shareBusy || shareUploadingArtifact}>
            {shareUploadingArtifact ? "Uploading artifact..." : "Choose artifact"}
          </button>
          <div className="muted">
            {shareArtifactName ? `Selected: ${shareArtifactName}` : "Pick file to auto-upload and fill path"}
          </div>
        </div>
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
              <button
                className="btn"
                onClick={() => handleApproveRequest(r)}
                disabled={approvingRequestId === r.id}
              >
                {approvingRequestId === r.id ? "Approving..." : "Approve"}
              </button>
            </div>
          ))}
        </div>
      </div>

      <div className="card">
        <h3>Joined Students</h3>
        <div className="muted">Approved students: {members.length}</div>
        {members.length === 0 && <div className="muted">No students joined yet.</div>}
        <div className="list">
          {members.map((m, idx) => (
            <div key={`${m.student}-${idx}`} className="list-item">
              <div>
                <div className="list-title">{m.student}</div>
                <div className="muted">Classroom: {m.roomId}</div>
              </div>
              <div className="status-chip">Joined</div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
};

export default Dashboard;
