import React, { useEffect, useState } from "react";
import { useApp } from "../context/app.context";
import {
  downloadFileChunked,
  getBase,
  getPeers,
  getPosts,
  uploadPostChunked,
} from "../utils/api";

const ContentViewer = () => {
  const { classroomId, role } = useApp();
  const [posts, setPosts] = useState([]);
  const [peers, setPeers] = useState([]);
  const [content, setContent] = useState("");
  const [file, setFile] = useState(null);
  const [uploadStatus, setUploadStatus] = useState("");
  const [uploadProgress, setUploadProgress] = useState(0);
  const [uploadChunkState, setUploadChunkState] = useState("");
  const [uploadBusy, setUploadBusy] = useState(false);
  const [downloadProgress, setDownloadProgress] = useState(0);
  const [downloadState, setDownloadState] = useState("Discovery in progress...");
  const [activeSource, setActiveSource] = useState("");
  const [downloadResumeInfo, setDownloadResumeInfo] = useState("");
  const [manualDownload, setManualDownload] = useState(null);

  const resolveViewUrl = (fileUrl) => {
    if (!fileUrl) return "";
    if (/^https?:\/\//i.test(fileUrl)) return fileUrl;
    return `${getBase()}${fileUrl.startsWith("/") ? "" : "/"}${fileUrl}`;
  };

  useEffect(() => {
    const load = async () => {
      try {
        const [p, peers] = await Promise.all([
          classroomId ? getPosts(classroomId) : Promise.resolve([]),
          getPeers(),
        ]);
        setPosts(Array.isArray(p) ? p : []);
        setPeers(Array.isArray(peers) ? peers : []);
      } catch {
        setPosts([]);
        setPeers([]);
      }
    };
    load();
  }, [classroomId]);

  useEffect(() => {
    return () => {
      if (manualDownload?.href) {
        URL.revokeObjectURL(manualDownload.href);
      }
    };
  }, [manualDownload]);

  const reloadPosts = async () => {
    if (!classroomId) return;
    try {
      const p = await getPosts(classroomId);
      setPosts(Array.isArray(p) ? p : []);
    } catch {
      setPosts([]);
    }
  };

  const handleUpload = async (e) => {
    e.preventDefault();
    setUploadStatus("");
    if (!classroomId) {
      setUploadStatus("Join a classroom first.");
      return;
    }
    if (!content.trim() || !file) {
      setUploadStatus("Add content text and choose a file.");
      return;
    }
    if (uploadBusy) return;
    try {
      setUploadBusy(true);
      setUploadProgress(0);
      setUploadChunkState("Preparing chunks and checksums...");
      await uploadPostChunked({
        code: classroomId,
        content: content.trim(),
        file,
        onProgress: (p) => {
          setUploadProgress(p.percent || 0);
          setUploadStatus(
            `Uploading chunk ${p.chunkIndex + 1}/${p.totalChunks} (${p.percent}%)`
          );
        },
        onVerify: (v) => {
          if (v.status === "accepted") {
            setUploadChunkState(
              `Chunk ${v.index + 1}/${v.totalChunks} verified and accepted`
            );
            return;
          }
          if (v.status === "accepted-unverified") {
            setUploadChunkState(
              `Chunk ${v.index + 1}/${v.totalChunks} uploaded (verification limited on this browser)`
            );
            return;
          }
          if (v.status === "rejected") {
            setUploadChunkState(
              `Chunk ${v.index + 1}/${v.totalChunks} rejected, retrying...`
            );
          }
        },
      });
      setUploadProgress(100);
      setUploadStatus("File uploaded successfully.");
      setContent("");
      setFile(null);
      await reloadPosts();
    } catch (err) {
      setUploadStatus(err.message || "Upload failed.");
    } finally {
      setUploadBusy(false);
    }
  };

  const handleChunkDownload = async (post) => {
    if (!post?.fileUrl) return;
    if (!classroomId) {
      setDownloadState("Join a classroom first.");
      return;
    }
    if (manualDownload?.href) {
      URL.revokeObjectURL(manualDownload.href);
    }
    setManualDownload(null);
    setDownloadProgress(0);
    setDownloadState("Fetching manifest from peers...");
    setActiveSource("");
    setDownloadResumeInfo("");
    try {
      const result = await downloadFileChunked({
        fileUrl: post.fileUrl,
        onSource: (source) => setActiveSource(source),
        onProgress: (p) => {
          setDownloadProgress(p.percent || 0);
          if (p.resumedChunks > 0) {
            setDownloadResumeInfo(
              `Resumed with ${p.resumedChunks} chunks already available locally`
            );
          }
          const speedText = p.speedKbps
            ? ` | ${p.speedKbps} KB/s`
            : "";
          const chunkText = p.activeChunkSize
            ? ` | chunk ${Math.round(p.activeChunkSize / 1024)} KB`
            : "";
          setDownloadState(
            `Downloading chunk ${p.chunkIndex + 1}/${p.totalChunks} (${p.percent}%)${speedText}${chunkText}`
          );
        },
        onVerify: (v) => {
          if (v.status === "accepted") {
            setDownloadState(
              `Chunk ${v.index + 1}/${v.totalChunks} verified from peer`
            );
            return;
          }
          if (v.status === "accepted-unverified") {
            setDownloadState(
              `Chunk ${v.index + 1}/${v.totalChunks} downloaded (verification limited on this browser)`
            );
            return;
          }
          if (v.status === "rejected") {
            setDownloadState(
              `Chunk ${v.index + 1}/${v.totalChunks} checksum failed, switching peer...`
            );
          }
        },
      });
      setDownloadProgress(100);
      setDownloadState("Download complete and checksum verified.");
      if (result?.downloadHref) {
        setManualDownload({
          href: result.downloadHref,
          name: result.fileName || "download.bin",
        });
      }
      if (result?.resumedChunks > 0) {
        setDownloadResumeInfo(
          `Resume completed: ${result.resumedChunks} chunks were recovered from previous download`
        );
      }
    } catch (err) {
      setDownloadState(err.message || "Download failed");
    }
  };

  return (
    <div className="page">
      <div className="header">
        <h2>Content Viewer</h2>
        <div className="muted">Classroom ID: {classroomId || "Not Joined"}</div>
      </div>

      <div className="card">
        <div className="row">
          <div className="muted">Download State</div>
          <div className="muted">Peers sharing: {peers.length}</div>
        </div>
        <div className="progress">
          <div className="progress-bar" style={{ width: `${downloadProgress}%` }} />
        </div>
        <div className="muted">{downloadState}</div>
        {downloadResumeInfo && <div className="muted">{downloadResumeInfo}</div>}
        {manualDownload?.href && (
          <div className="muted">
            If file did not auto-save on mobile, tap:
            {" "}
            <a href={manualDownload.href} download={manualDownload.name}>Save File</a>
          </div>
        )}
        <div className="muted">
          {activeSource ? `Active source: ${activeSource}` : "Discovery in progress..."}
        </div>
      </div>

      {role === "teacher" && (
        <div className="card">
          <h3>Upload Material</h3>
          <form onSubmit={handleUpload} className="column">
            <label className="label">Material title or note</label>
            <input
              className="input"
              value={content}
              onChange={(e) => setContent(e.target.value)}
              placeholder="e.g. Unit 3 PDF"
            />
            <label className="label">Choose PDF/video/file</label>
            <input
              className="input"
              type="file"
              onChange={(e) => setFile(e.target.files?.[0] || null)}
            />
            <button type="submit" className="btn primary" disabled={uploadBusy}>
              {uploadBusy ? "Uploading..." : "Upload File"}
            </button>
            <div className="progress">
              <div className="progress-bar" style={{ width: `${uploadProgress}%` }} />
            </div>
            {uploadChunkState && <div className="muted">{uploadChunkState}</div>}
            {uploadStatus && <div className="muted">{uploadStatus}</div>}
          </form>
        </div>
      )}

      <div className="card">
        <h3>Materials</h3>
        {posts.length === 0 && <div className="muted">No materials yet.</div>}
        <div className="list">
          {posts.map((p) => (
            <div key={p.id} className="list-item">
              <div>
                <div className="list-title">{p.content || "Untitled"}</div>
                <div className="muted">Available Offline</div>
                <div className="muted">Source peers: {peers.length}</div>
              </div>
              {p.fileUrl && (
                <div className="row">
                  <a
                    className="btn"
                    href={resolveViewUrl(p.fileUrl)}
                    target="_blank"
                    rel="noreferrer"
                  >
                    View
                  </a>
                  <button className="btn" onClick={() => handleChunkDownload(p)}>
                    Download
                  </button>
                </div>
              )}
            </div>
          ))}
        </div>
      </div>
    </div>
  );
};

export default ContentViewer;
