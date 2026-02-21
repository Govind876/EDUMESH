import React, { useCallback, useEffect, useMemo, useState } from "react";
import { useApp } from "../context/app.context";
import { useProfile } from "../context/profile.context";
import {
  createAssignment,
  listAssignments,
  listSubmissions,
  submitAssignment,
} from "../utils/api";

const CHUNK_BYTES = 512 * 1024;

function toHex(buffer) {
  return Array.from(new Uint8Array(buffer))
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
}

async function sha256HexFromBuffer(buffer) {
  const digest = await crypto.subtle.digest("SHA-256", buffer);
  return toHex(digest);
}

function uint8ToBase64(bytes) {
  let binary = "";
  const chunk = 0x8000;
  for (let i = 0; i < bytes.length; i += chunk) {
    binary += String.fromCharCode(...bytes.subarray(i, i + chunk));
  }
  return btoa(binary);
}

function base64ToUint8(base64) {
  const binary = atob(base64);
  const out = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i += 1) {
    out[i] = binary.charCodeAt(i);
  }
  return out;
}

const Assignments = () => {
  const { role, classroomId } = useApp();
  const { profile } = useProfile();
  const [data, setData] = useState({ list: [], submissions: [] });
  const [title, setTitle] = useState("");
  const [desc, setDesc] = useState("");
  const [answers, setAnswers] = useState({});
  const [answerFiles, setAnswerFiles] = useState({});
  const [submitState, setSubmitState] = useState("");
  const [submitProgress, setSubmitProgress] = useState(0);
  const [downloadState, setDownloadState] = useState("");
  const [downloadProgress, setDownloadProgress] = useState(0);
  const [status, setStatus] = useState("");

  const statsByAssignment = useMemo(() => {
    const out = {};
    for (const s of data.submissions) {
      out[s.assignmentId] = (out[s.assignmentId] || 0) + 1;
    }
    return out;
  }, [data.submissions]);

  const refresh = useCallback(async () => {
    if (!classroomId) {
      setData({ list: [], submissions: [] });
      return;
    }
    try {
      const [list, submissions] = await Promise.all([
        listAssignments(classroomId),
        listSubmissions(classroomId),
      ]);
      setData({
        list: Array.isArray(list) ? list : [],
        submissions: Array.isArray(submissions) ? submissions : [],
      });
    } catch (e) {
      setStatus(e.message || "Failed to load assignments");
    }
  }, [classroomId]);

  useEffect(() => {
    refresh();
    const t = setInterval(refresh, 8000);
    return () => clearInterval(t);
  }, [refresh]);

  const addAssignment = async () => {
    if (!classroomId || !title.trim()) return;
    try {
      setStatus("");
      await createAssignment({
        code: classroomId,
        title: title.trim(),
        description: desc.trim(),
        createdBy: profile?.name || "Teacher",
      });
      setTitle("");
      setDesc("");
      await refresh();
      setStatus("Assignment created.");
    } catch (e) {
      setStatus(e.message || "Failed to create assignment.");
    }
  };

  const submitAnswer = async (assignmentId) => {
    const answerText = String(answers[assignmentId] || "").trim();
    const file = answerFiles[assignmentId] || null;
    if (!classroomId) {
      setSubmitState("Join a classroom first.");
      return;
    }
    if (!answerText || !file) {
      setSubmitState("Add answer text and choose a file.");
      return;
    }
    setSubmitProgress(0);
    setSubmitState("Chunking submission and computing checksums...");
    const chunks = [];
    let totalBytes = 0;
    for (let i = 0; i < file.size; i += CHUNK_BYTES) {
      const blob = file.slice(i, Math.min(file.size, i + CHUNK_BYTES));
      const buffer = await blob.arrayBuffer();
      const bytes = new Uint8Array(buffer);
      const hash = await sha256HexFromBuffer(buffer);
      chunks.push({
        index: chunks.length,
        hash,
        data: uint8ToBase64(bytes),
      });
      totalBytes += bytes.byteLength;
      setSubmitProgress(Math.round((totalBytes / Math.max(file.size, 1)) * 100));
      setSubmitState(
        `Preparing chunk ${chunks.length}/${Math.ceil(file.size / CHUNK_BYTES)}`
      );
    }
    const fileSha256 = await sha256HexFromBuffer(await file.arrayBuffer());
    try {
      await submitAssignment({
        assignmentId,
        code: classroomId,
        student: profile?.name || profile?.email || "Student",
        answer: answerText,
        status: "Submitted",
        fileName: file.name,
        fileType: file.type || "application/octet-stream",
        fileSize: file.size,
        fileSha256,
        chunks,
      });
      setAnswers((prev) => ({ ...prev, [assignmentId]: "" }));
      setAnswerFiles((prev) => ({ ...prev, [assignmentId]: null }));
      setSubmitProgress(100);
      setSubmitState("Submission uploaded in verified chunks.");
      await refresh();
    } catch (e) {
      setSubmitState(e.message || "Submission failed.");
    }
  };

  const downloadSubmission = async (submission) => {
    const chunks = Array.isArray(submission?.chunks) ? submission.chunks : [];
    if (chunks.length === 0) {
      setDownloadState("No file chunks available for this submission.");
      return;
    }
    setDownloadState("Verifying chunks before download...");
    setDownloadProgress(0);
    const verified = [];
    let done = 0;
    for (let i = 0; i < chunks.length; i += 1) {
      const chunk = chunks[i];
      const bytes = base64ToUint8(chunk.data || "");
      const actual = await sha256HexFromBuffer(bytes.buffer);
      if (actual !== String(chunk.hash || "").toLowerCase()) {
        setDownloadState(
          `Chunk ${i + 1}/${chunks.length} checksum failed. Download stopped.`
        );
        return;
      }
      verified.push(bytes);
      done += bytes.byteLength;
      setDownloadProgress(
        Math.round((done / Math.max(Number(submission.fileSize) || done, 1)) * 100)
      );
      setDownloadState(`Chunk ${i + 1}/${chunks.length} verified`);
    }

    const fileBlob = new Blob(verified, {
      type: submission.fileType || "application/octet-stream",
    });
    const finalHash = await sha256HexFromBuffer(await fileBlob.arrayBuffer());
    if (
      submission.fileSha256 &&
      finalHash !== String(submission.fileSha256).toLowerCase()
    ) {
      setDownloadState("Final file checksum mismatch.");
      return;
    }

    const href = URL.createObjectURL(fileBlob);
    const anchor = document.createElement("a");
    anchor.href = href;
    anchor.download = submission.fileName || "submission.bin";
    document.body.appendChild(anchor);
    anchor.click();
    anchor.remove();
    URL.revokeObjectURL(href);
    setDownloadProgress(100);
    setDownloadState("Download complete and checksum verified.");
  };

  return (
    <div className="page">
      <div className="header">
        <h2>Assignments</h2>
        <div className="muted">Classroom ID: {classroomId || "Not Joined"}</div>
        <div className="muted">Synced across peers in this classroom</div>
      </div>

      {!classroomId && (
        <div className="alert">Join a classroom to access assignments and submissions.</div>
      )}

      {role === "teacher" && classroomId && (
        <div className="card">
          <h3>Create Assignment</h3>
          <label className="label">Title</label>
          <input
            className="input"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
          />
          <label className="label">Description</label>
          <textarea
            className="input"
            rows="3"
            value={desc}
            onChange={(e) => setDesc(e.target.value)}
          />
          <button className="btn primary" onClick={addAssignment} disabled={!title.trim()}>
            Add Assignment
          </button>
        </div>
      )}

      <div className="card">
        <h3>Assignment List</h3>
        {data.list.length === 0 && <div className="muted">No assignments yet.</div>}
        <div className="list">
          {data.list.map((a) => (
            <div key={a.id} className="list-item">
              <div>
                <div className="list-title">{a.title}</div>
                <div className="muted">{a.description || "No description"}</div>
                {role === "teacher" && (
                  <div className="muted">
                    Submissions: {statsByAssignment[a.id] || 0}
                  </div>
                )}
              </div>
              {role === "student" && classroomId && (
                <div className="column">
                  <textarea
                    className="input"
                    rows="2"
                    placeholder="Write your answer"
                    value={answers[a.id] || ""}
                    onChange={(e) =>
                      setAnswers((prev) => ({ ...prev, [a.id]: e.target.value }))
                    }
                  />
                  <input
                    className="input"
                    type="file"
                    onChange={(e) =>
                      setAnswerFiles((prev) => ({
                        ...prev,
                        [a.id]: e.target.files?.[0] || null,
                      }))
                    }
                  />
                  <button className="btn" onClick={() => submitAnswer(a.id)}>
                    Submit
                  </button>
                </div>
              )}
            </div>
          ))}
        </div>
      </div>

      {role === "teacher" && (
        <div className="card">
          <h3>Submissions</h3>
          <div className="progress">
            <div className="progress-bar" style={{ width: `${downloadProgress}%` }} />
          </div>
          {downloadState && <div className="muted">{downloadState}</div>}
          {data.submissions.length === 0 && <div className="muted">No submissions yet.</div>}
          <div className="list">
            {data.submissions.map((s) => (
              <div key={s.id} className="list-item">
                <div>
                  <div className="list-title">{s.student}</div>
                  <div className="muted">{s.answer}</div>
                  {s.fileName && (
                    <div className="muted">
                      File: {s.fileName} ({Math.ceil((s.fileSize || 0) / 1024)} KB)
                    </div>
                  )}
                </div>
                <div className="column">
                  <div className="status-chip">{s.status || "Submitted"}</div>
                  {s.fileName && (
                    <button className="btn" onClick={() => downloadSubmission(s)}>
                      Download File
                    </button>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {role === "student" && (
        <div className="card">
          <div className="progress">
            <div className="progress-bar" style={{ width: `${submitProgress}%` }} />
          </div>
          {submitState && <div className="muted">{submitState}</div>}
        </div>
      )}

      {status && <div className="alert">{status}</div>}
    </div>
  );
};

export default Assignments;
