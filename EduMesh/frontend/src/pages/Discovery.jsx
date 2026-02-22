import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { QRCodeCanvas } from "qrcode.react";
import { useApp } from "../context/app.context";
import { useProfile } from "../context/profile.context";
import {
  createRoom,
  deleteClassroom,
  discoverClassrooms,
  getBase,
  getPeers,
  getRoomDetails,
  approveJoin,
  requestJoin,
  checkMembership,
} from "../utils/api";

const Discovery = () => {
  const navigate = useNavigate();
  const { role, setClassroom } = useApp();
  const { profile } = useProfile();
  const scannerRef = useRef(null);
  const scannerRunningRef = useRef(false);
  const [rooms, setRooms] = useState([]);
  const [peers, setPeers] = useState([]);
  const [manualId, setManualId] = useState("");
  const [qrInput, setQrInput] = useState("");
  const [scannerOn, setScannerOn] = useState(false);
  const [createTitle, setCreateTitle] = useState("");
  const [createDesc, setCreateDesc] = useState("");
  const [createManualId, setCreateManualId] = useState("");
  const [status, setStatus] = useState("");
  const [deletingRoomId, setDeletingRoomId] = useState("");
  const secureContext =
    typeof window !== "undefined" ? Boolean(window.isSecureContext) : false;

  const hostname =
    typeof window !== "undefined" ? window.location.hostname : "localhost";
  const baseHost = useMemo(() => {
    try {
      const base = new URL(getBase());
      const resolvedHost =
        base.hostname === "localhost" ||
        base.hostname === "127.0.0.1" ||
        base.hostname === "::1"
          ? hostname
          : base.hostname;
      const port = base.port || (base.protocol === "https:" ? "443" : "80");
      return `${resolvedHost}:${port}`;
    } catch {
      return `${hostname}:8080`;
    }
  }, [hostname]);

  const qrData = useMemo(() => {
    if (!rooms.length) return "";
    const room = rooms[0];
    return `http://${hostname}:3000/discover?room=${encodeURIComponent(
      room.id
    )}&host=${encodeURIComponent(baseHost)}`;
  }, [rooms, baseHost, hostname]);

  const refresh = useCallback(async () => {
    try {
      const [r, p] = await Promise.all([discoverClassrooms(), getPeers()]);
      setRooms(Array.isArray(r) ? r : []);
      setPeers(Array.isArray(p) ? p : []);
    } catch {
      setRooms([]);
      setPeers([]);
    }
  }, []);

  useEffect(() => {
    refresh();
    const t = setInterval(refresh, 10000);
    return () => clearInterval(t);
  }, [refresh]);

  const handleJoin = useCallback(
    async (roomId, instant = false) => {
      setStatus("");
      if (role !== "student") {
        setStatus(
          "Join actions are for students only. Teachers should create or activate classrooms."
        );
        return;
      }
      try {
        const student = profile?.email || profile?.name || "student";
        if (instant) {
          await approveJoin("", roomId, student);
          const info = await getRoomDetails(roomId);
          setClassroom(roomId, info?.title, info?.teacher);
          navigate("/dashboard");
          return;
        }
        await requestJoin(roomId, student);
        setStatus("Join request sent. Waiting for teacher approval...");
        for (let attempt = 0; attempt < 20; attempt += 1) {
          const check = await checkMembership(roomId, student);
          if (check?.approved) {
            const info = await getRoomDetails(roomId);
            setClassroom(roomId, info?.title, info?.teacher);
            navigate("/dashboard");
            return;
          }
          await new Promise((resolve) => setTimeout(resolve, 2000));
        }
        setStatus("Approval pending. Ask teacher to approve, then tap Join again.");
      } catch (e) {
        setStatus(e.message || "Join failed");
      }
    },
    [profile, role, setClassroom, navigate]
  );

  const activateRoom = useCallback(
    async (roomId) => {
      const id = String(roomId || "").trim();
      if (!id) return;
      try {
        const info = await getRoomDetails(id);
        setClassroom(id, info?.title, info?.teacher);
        navigate("/dashboard");
      } catch (e) {
        setStatus(e.message || "Unable to activate classroom");
      }
    },
    [setClassroom, navigate]
  );

  const handleManualJoin = async () => {
    if (role !== "student") {
      setStatus("Manual classroom ID join is for students.");
      return;
    }
    const id = manualId.trim();
    if (!id) return;
    handleJoin(id);
  };

  const handleQrJoin = async () => {
    if (role !== "student") {
      setStatus("QR join is for students.");
      return;
    }
    const raw = qrInput.trim();
    if (!raw) return;
    try {
      if (raw.includes("room=")) {
        const url = new URL(raw.replace("edumesh://", "http://"));
        const room = url.searchParams.get("room");
        if (room) {
          setManualId(room);
          handleJoin(room, true);
          return;
        }
      }
      setStatus("Invalid QR data");
    } catch {
      setStatus("Invalid QR data");
    }
  };

  const parseAndJoin = useCallback(
    async (raw) => {
      if (!raw || role !== "student") return false;
      if (raw.includes("room=")) {
        const url = new URL(raw.replace("edumesh://", "http://"));
        const room = url.searchParams.get("room");
        if (room) {
          setManualId(room);
          await handleJoin(room, true);
          return true;
        }
      }
      return false;
    },
    [handleJoin, role]
  );

  useEffect(() => {
    if (role !== "student") return;
    const params = new URLSearchParams(window.location.search);
    const room = String(params.get("room") || "").trim();
    if (!room) return;
    setManualId(room);
    handleJoin(room, true);
  }, [handleJoin, role]);

  const handleCreate = async () => {
    setStatus("");
    try {
      const teacher = profile?.name || "Teacher";
      const res = await createRoom({
        title: createTitle.trim(),
        teacher,
        description: createDesc.trim(),
        manualId: createManualId.trim(),
      });
      const id = res?.code;
      if (!id) throw new Error("Room creation failed");
      const info = await getRoomDetails(id);
      setClassroom(id, info?.title, info?.teacher);
      setCreateManualId("");
      navigate("/dashboard");
    } catch (e) {
      setStatus(e.message || "Create failed");
    }
  };

  const handleScanNow = async () => {
    await refresh();
    setStatus("Discovery refreshed from local node and peers.");
  };

  const handleDeleteClassroom = async (roomId) => {
    const id = String(roomId || "").trim();
    if (!id) return;
    if (role !== "teacher") return;
    if (!window.confirm(`Delete classroom "${id}" for all members?`)) {
      return;
    }
    setDeletingRoomId(id);
    setStatus("");
    try {
      await deleteClassroom(id);
      await refresh();
      setStatus(`Classroom "${id}" deleted.`);
    } catch (e) {
      setStatus(e.message || "Failed to delete classroom");
    } finally {
      setDeletingRoomId("");
    }
  };

  const connected = peers.length > 0 || navigator.onLine;

  useEffect(() => {
    let localScanner;
    const start = async () => {
      if (!scannerOn || scannerRunningRef.current || role !== "student") return;
      if (!secureContext) {
        setStatus(
          "Camera scanner needs HTTPS on mobile browsers. Use Manual Classroom ID or paste QR text."
        );
        setScannerOn(false);
        return;
      }
      const { Html5Qrcode } = await import("html5-qrcode");
      localScanner = new Html5Qrcode("qr-reader");
      scannerRef.current = localScanner;
      scannerRunningRef.current = true;
      try {
        await localScanner.start(
          { facingMode: "environment" },
          { fps: 10, qrbox: 240 },
          async (text) => {
            setQrInput(text);
            const ok = await parseAndJoin(text);
            if (ok) {
              await localScanner.stop();
              scannerRunningRef.current = false;
              setScannerOn(false);
            }
          }
        );
      } catch {
        setStatus("Camera access failed");
        scannerRunningRef.current = false;
        setScannerOn(false);
      }
    };
    start();
    return () => {
      if (scannerRef.current && scannerRunningRef.current) {
        scannerRef.current.stop().catch(() => {});
      }
      scannerRunningRef.current = false;
    };
  }, [scannerOn, secureContext, parseAndJoin, role]);

  return (
    <div className="page">
      <div className="header">
        <h2>Local Classroom Discovery</h2>
        <div className="meta-row">
          <span className={`dot ${connected ? "green" : "red"}`} />
          <span>
            {connected ? "Local network connected" : "Local network not detected"}
          </span>
        </div>
        <div className="muted">No Internet Required - Local Network Only</div>
      </div>

      {role === "teacher" && (
        <div className="card">
          <h3>Create a Classroom</h3>
          <label className="label">Title</label>
          <input
            className="input"
            value={createTitle}
            onChange={(e) => setCreateTitle(e.target.value)}
          />
          <label className="label">Description</label>
          <textarea
            className="input"
            rows="3"
            value={createDesc}
            onChange={(e) => setCreateDesc(e.target.value)}
          />
          <label className="label">Manual Classroom ID (optional)</label>
          <input
            className="input"
            value={createManualId}
            onChange={(e) => setCreateManualId(e.target.value)}
            placeholder="e.g. math-class-10A"
          />
          <button
            className="btn primary"
            onClick={handleCreate}
            disabled={!createTitle.trim()}
          >
            Create Classroom
          </button>
        </div>
      )}

      <div className="card">
        <h3>{role === "teacher" ? "Classrooms" : "Discovered Classrooms"}</h3>
        <div className="muted">Peers online: {peers.length}</div>
        <button className="btn" onClick={handleScanNow}>
          Scan Now
        </button>
        {rooms.length === 0 && <div className="muted">No classrooms found yet.</div>}
        <div className="list">
          {rooms.map((r) => (
            <div key={r.id} className="list-item">
              <div>
                <div className="list-title">{r.title || r.id}</div>
                <div className="muted">{r.description || "No description"}</div>
                <div className="muted">Classroom ID: {r.id}</div>
              </div>
              {role === "student" ? (
                <button className="btn" onClick={() => handleJoin(r.id)}>
                  Join
                </button>
              ) : (
                <div className="row">
                  <button className="btn" onClick={() => activateRoom(r.id)}>
                    Activate
                  </button>
                  <button
                    className="btn"
                    onClick={() => handleDeleteClassroom(r.id)}
                    disabled={deletingRoomId === r.id}
                  >
                    {deletingRoomId === r.id ? "Deleting..." : "Delete"}
                  </button>
                </div>
              )}
            </div>
          ))}
        </div>
      </div>

      {role === "student" && (
        <div className="card">
          <h3>QR Code Join (Offline)</h3>
          {!secureContext && (
            <div className="muted">
              Mobile camera scan is blocked on non-HTTPS pages. Use manual join on LAN.
            </div>
          )}
          <div className="qr-wrap">
            {qrData ? (
              <QRCodeCanvas value={qrData} size={140} />
            ) : (
              <div className="muted">Create a classroom to show QR.</div>
            )}
          </div>
          <button className="btn" onClick={() => setScannerOn((v) => !v)}>
            {scannerOn ? "Stop Scanner" : "Start Camera Scanner"}
          </button>
          {scannerOn && <div id="qr-reader" style={{ width: "100%", marginTop: 10 }} />}
          <label className="label">Paste QR Data</label>
          <input
            className="input"
            value={qrInput}
            onChange={(e) => setQrInput(e.target.value)}
          />
          <button className="btn" onClick={handleQrJoin}>
            Join via QR
          </button>
        </div>
      )}

      {role === "student" && (
        <div className="card">
          <h3>Manual Classroom ID Join</h3>
          <label className="label">Classroom ID</label>
          <input
            className="input"
            value={manualId}
            onChange={(e) => setManualId(e.target.value)}
          />
          <button className="btn" onClick={handleManualJoin}>
            Join
          </button>
        </div>
      )}

      {status && <div className="alert">{status}</div>}
    </div>
  );
};

export default Discovery;
