function resolveApiBase() {
  if (process.env.REACT_APP_API_BASE) {
    return process.env.REACT_APP_API_BASE;
  }
  if (typeof window === "undefined") {
    return "http://localhost:8080";
  }
  const { protocol, hostname } = window.location;
  const safeHost = hostname || "localhost";
  // In dev we serve UI on :3000 and backend on :8080. On mobile/LAN,
  // hostname is the laptop IP, not localhost.
  if (window.location.port === "3000") {
    return `${protocol}//${safeHost}:8080`;
  }
  return `${protocol}//${safeHost}:8080`;
}

const API_BASE = resolveApiBase();

function makeId() {
  if (typeof crypto !== "undefined" && crypto.randomUUID) {
    return crypto.randomUUID();
  }
  return `${Date.now()}-${Math.random().toString(36).slice(2, 10)}`;
}

function toHex(buffer) {
  return Array.from(new Uint8Array(buffer))
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
}

function hasCryptoDigest() {
  return (
    typeof crypto !== "undefined" &&
    Boolean(crypto.subtle) &&
    typeof crypto.subtle.digest === "function"
  );
}

async function sha256HexFromBuffer(buffer) {
  if (!hasCryptoDigest()) {
    return "";
  }
  const digest = await crypto.subtle.digest("SHA-256", buffer);
  return toHex(digest);
}

async function sha256HexFromBlob(blob) {
  const buffer = await blob.arrayBuffer();
  return sha256HexFromBuffer(buffer);
}

async function requestJson(path, method = "POST", body = undefined) {
  const headers = {};
  let payload;
  if (body !== undefined) {
    headers["Content-Type"] = "application/json";
    payload = JSON.stringify(body);
  }

  const response = await fetch(`${API_BASE}${path}`, {
    method,
    headers,
    body: payload,
  });

  if (!response.ok) {
    const text = await response.text();
    throw new Error(text || "Request failed");
  }

  return response.json().catch(() => ({}));
}

// Add user (no auth needed)
export async function addUser(uname) {
  if (!uname) throw new Error("Username is required");
  return "OK";
}

// Create room
export async function createRoom({ title, teacher, description }) {
  const id = makeId();
  await requestJson("/join_room", "POST", {
    id,
    title,
    description,
    teacher,
  });
  return { code: id };
}

// Join room
export async function joinRoom(uname, code) {
  if (!code) throw new Error("Room code is required");
  const res = await fetch(
    `${API_BASE}/room?id=${encodeURIComponent(code)}`
  );
  if (!res.ok) {
    throw new Error(await res.text());
  }
  return "Joined room";
}

export async function requestJoin(roomId, student) {
  const id = makeId();
  const response = await fetch(`${API_BASE}/request_join`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ id, roomId, student }),
  });
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return { id };
}

export async function getJoinRequests(roomId) {
  const response = await fetch(
    `${API_BASE}/join_requests?roomId=${encodeURIComponent(roomId)}`
  );
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return await response.json();
}

export async function approveJoin(id, roomId, student) {
  const response = await fetch(`${API_BASE}/approve_join`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ id, roomId, student }),
  });
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return await response.text();
}

export async function checkMembership(roomId, student) {
  const response = await fetch(
    `${API_BASE}/membership?roomId=${encodeURIComponent(
      roomId
    )}&student=${encodeURIComponent(student)}`
  );
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return await response.json();
}

// Upload post (with file)
export async function uploadPost({ code, content, file }) {
  return uploadPostChunked({ code, content, file });
}

// Get posts in a room
export async function getPosts(code) {
  return await requestJson("/posts", "POST", { code });
}

// Create announcement
export async function createAnnouncement({ code, title, description }) {
  return await requestJson("/announce", "POST", {
    id: makeId(),
    code,
    title,
    description,
  });
}

// Get announcements
export async function getAnnouncements(code) {
  return await requestJson("/getAnnouncements", "POST", { code });
}

// Get all rooms a user has joined (no auth required)
export async function getMyRooms(uname) {
  const response = await fetch(`${API_BASE}/roomsof`);
  if (!response.ok) throw new Error(await response.text());
  return await response.json();
}

// Get room details
export async function getRoomDetails(code) {
  const response = await fetch(
    `${API_BASE}/room?id=${encodeURIComponent(code)}`
  );
  if (!response.ok) throw new Error(await response.text());
  return await response.json();
}

export function getBase() {
  return API_BASE;
}

export async function startViralShare({
  ssid = "",
  password = "",
  serverIp = "",
  artifactPath = "",
  enableBluetoothFallback = true,
} = {}) {
  return await requestJson("/vcd/start", "POST", {
    ssid,
    password,
    serverIp,
    artifactPath,
    enableBluetoothFallback,
  });
}

export async function getViralShareStatus() {
  const response = await fetch(`${API_BASE}/vcd/status`);
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return await response.json();
}

export async function stopViralShare() {
  return await requestJson("/vcd/stop", "POST", {});
}

export async function getPeers() {
  const response = await fetch(`${API_BASE}/peers`);
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return await response.json();
}

export async function getChunkOwners(fileId, chunkIndex) {
  const response = await fetch(
    `${API_BASE}/chunks/owners?fileId=${encodeURIComponent(fileId)}&chunkIndex=${encodeURIComponent(chunkIndex)}`
  );
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return await response.json();
}

export async function getRoomStats(roomId = "") {
  const suffix = roomId
    ? `?roomId=${encodeURIComponent(roomId)}`
    : "";
  const response = await fetch(`${API_BASE}/room_stats${suffix}`);
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return await response.json();
}

export async function createAssignment({
  code,
  title,
  description,
  createdBy,
}) {
  return await requestJson("/assignments/create", "POST", {
    id: makeId(),
    code,
    title,
    description,
    createdBy,
    createdAt: new Date().toISOString(),
  });
}

export async function listAssignments(code) {
  return await requestJson("/assignments/list", "POST", { code });
}

export async function submitAssignment(payload) {
  return await requestJson("/assignments/submit", "POST", {
    ...payload,
    id: payload?.id || makeId(),
    submittedAt: payload?.submittedAt || new Date().toISOString(),
  });
}

export async function listSubmissions(code, assignmentId = "") {
  return await requestJson("/assignments/submissions", "POST", {
    code,
    assignmentId,
  });
}

export async function uploadPostChunked({
  code,
  content,
  file,
  onProgress,
  onVerify,
}) {
  if (!file) throw new Error("File is required");
  const fileSha256 = await sha256HexFromBlob(file);
  const totalChunks = Math.max(1, Math.ceil(file.size / (1024 * 1024)));
  const started = await requestJson("/upload/start", "POST", {
    id: makeId(),
    code,
    content,
    fileName: file.name || "upload.bin",
    fileSize: file.size,
    totalChunks,
    fileSha256,
  });
  const uploadId = started.uploadId;
  const actualChunkSize = started.chunkSize || 1024 * 1024;
  if (!uploadId) throw new Error("Upload session failed");

  let uploadedBytes = 0;
  for (let index = 0; index < totalChunks; index += 1) {
    const begin = index * actualChunkSize;
    const end = Math.min(file.size, begin + actualChunkSize);
    const chunkBlob = file.slice(begin, end);
    const checksum = await sha256HexFromBlob(chunkBlob);
    onVerify?.({
      type: "upload",
      status: checksum ? "hash-ready" : "hash-skipped",
      index,
      totalChunks,
      checksum,
    });

    let accepted = false;
    let attempts = 0;
    while (!accepted && attempts < 3) {
      attempts += 1;
      const formData = new FormData();
      formData.append("uploadId", uploadId);
      formData.append("index", String(index));
      formData.append("checksum", checksum || "");
      formData.append("chunk", chunkBlob, `chunk-${index}.part`);

      const response = await fetch(`${API_BASE}/upload/chunk`, {
        method: "POST",
        body: formData,
      });
      if (response.ok) {
        accepted = true;
        onVerify?.({
          type: "upload",
          status: checksum ? "accepted" : "accepted-unverified",
          index,
          totalChunks,
          checksum,
        });
      } else {
        onVerify?.({
          type: "upload",
          status: "rejected",
          index,
          totalChunks,
          checksum,
        });
        if (attempts >= 3) {
          const text = await response.text();
          throw new Error(text || `Chunk ${index + 1} rejected`);
        }
      }
    }

    uploadedBytes += chunkBlob.size;
    onProgress?.({
      type: "upload",
      loadedBytes: uploadedBytes,
      totalBytes: file.size,
      chunkIndex: index,
      totalChunks,
      percent: Math.round((uploadedBytes / Math.max(file.size, 1)) * 100),
    });
  }

  const finish = await fetch(`${API_BASE}/upload/finish`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ uploadId }),
  });
  if (!finish.ok) {
    const text = await finish.text();
    throw new Error(text || "Failed to finalize upload");
  }
  return finish.json().catch(() => ({}));
}

function parseFileIdFromUrl(fileUrl) {
  if (!fileUrl) return "";
  const match = fileUrl.match(/\/files\/([^/]+)\//);
  return match?.[1] || "";
}

function normalizeSource(base) {
  return base.replace(/\/+$/, "");
}

function buildPeerSources(peerList) {
  const out = [];
  const seen = new Set();
  const add = (url) => {
    if (!url) return;
    const normalized = normalizeSource(url);
    if (seen.has(normalized)) return;
    seen.add(normalized);
    out.push(normalized);
  };
  add(API_BASE);
  for (const peer of Array.isArray(peerList) ? peerList : []) {
    const host = (peer?.host || peer?.ip || "").trim();
    const port = peer?.port || 8080;
    if (!host) continue;
    add(`http://${host}:${port}`);
  }
  return out;
}

function ownerSourcesFromPayload(payload) {
  const owners = Array.isArray(payload?.owners) ? payload.owners : [];
  const out = [];
  const seen = new Set();
  const add = (url) => {
    if (!url) return;
    const normalized = normalizeSource(url);
    if (seen.has(normalized)) return;
    seen.add(normalized);
    out.push(normalized);
  };
  for (const owner of owners) {
    const host = (owner?.host || "").trim();
    const port = Number(owner?.port || 8080);
    if (!host) continue;
    add(`http://${host}:${port}`);
  }
  return out;
}

const RESUME_DB_NAME = "schoolsync_resume_db";
const RESUME_DB_VERSION = 1;
const RESUME_STORE = "chunks";
const RESUME_STATUS_PREFIX = "schoolsync_download_status_";
const SPEED_TEST_FALLBACK_KBPS = 40;
const SPEED_RECHECK_INTERVAL_MS = 15000;
const SPEED_RECHECK_SEGMENTS = 5;

function getStatusKey(fileId) {
  return `${RESUME_STATUS_PREFIX}${fileId}`;
}

function readDownloadStatus(fileId) {
  if (typeof window === "undefined" || !window.localStorage) return null;
  try {
    const raw = window.localStorage.getItem(getStatusKey(fileId));
    return raw ? JSON.parse(raw) : null;
  } catch {
    return null;
  }
}

function writeDownloadStatus(fileId, status) {
  if (typeof window === "undefined" || !window.localStorage) return;
  try {
    window.localStorage.setItem(getStatusKey(fileId), JSON.stringify(status));
  } catch {
    // ignore storage quota failures
  }
}

function clearDownloadStatus(fileId) {
  if (typeof window === "undefined" || !window.localStorage) return;
  try {
    window.localStorage.removeItem(getStatusKey(fileId));
  } catch {
    // ignore
  }
}

function openResumeDB() {
  if (typeof indexedDB === "undefined") {
    return Promise.resolve(null);
  }
  return new Promise((resolve, reject) => {
    const req = indexedDB.open(RESUME_DB_NAME, RESUME_DB_VERSION);
    req.onupgradeneeded = () => {
      const db = req.result;
      if (!db.objectStoreNames.contains(RESUME_STORE)) {
        db.createObjectStore(RESUME_STORE);
      }
    };
    req.onsuccess = () => resolve(req.result);
    req.onerror = () => reject(req.error || new Error("IndexedDB open failed"));
  });
}

function withStore(db, mode, fn) {
  return new Promise((resolve, reject) => {
    const tx = db.transaction(RESUME_STORE, mode);
    const store = tx.objectStore(RESUME_STORE);
    const req = fn(store);
    tx.oncomplete = () => resolve(req?.result);
    tx.onerror = () => reject(tx.error || new Error("IndexedDB transaction failed"));
    tx.onabort = () => reject(tx.error || new Error("IndexedDB transaction aborted"));
  });
}

async function idbGet(db, key) {
  if (!db) return null;
  return withStore(db, "readonly", (store) => store.get(key));
}

async function idbPut(db, key, value) {
  if (!db) return;
  await withStore(db, "readwrite", (store) => store.put(value, key));
}

async function idbDeletePrefix(db, prefix) {
  if (!db) return;
  await new Promise((resolve, reject) => {
    const tx = db.transaction(RESUME_STORE, "readwrite");
    const store = tx.objectStore(RESUME_STORE);
    const req = store.openCursor();
    req.onsuccess = () => {
      const cursor = req.result;
      if (!cursor) return;
      if (String(cursor.key).startsWith(prefix)) {
        cursor.delete();
      }
      cursor.continue();
    };
    tx.oncomplete = resolve;
    tx.onerror = () => reject(tx.error || new Error("IndexedDB cleanup failed"));
    tx.onabort = () => reject(tx.error || new Error("IndexedDB cleanup aborted"));
  });
}

function rangeKey(fileId, start, end) {
  return `${fileId}:${start}-${end}`;
}

function mergeRanges(ranges) {
  const sorted = (Array.isArray(ranges) ? ranges : [])
    .filter((r) => Number.isFinite(r?.start) && Number.isFinite(r?.end) && r.end > r.start)
    .map((r) => ({ start: Number(r.start), end: Number(r.end) }))
    .sort((a, b) => a.start - b.start);
  if (sorted.length === 0) return [];
  const merged = [sorted[0]];
  for (let i = 1; i < sorted.length; i += 1) {
    const current = sorted[i];
    const prev = merged[merged.length - 1];
    if (current.start <= prev.end) {
      prev.end = Math.max(prev.end, current.end);
      continue;
    }
    merged.push(current);
  }
  return merged;
}

function coveredBytes(ranges) {
  return mergeRanges(ranges).reduce((acc, r) => acc + (r.end - r.start), 0);
}

function nextMissingOffset(ranges, size) {
  const merged = mergeRanges(ranges);
  if (merged.length === 0) return 0;
  if (merged[0].start > 0) return 0;
  let cursor = merged[0].end;
  for (let i = 1; i < merged.length; i += 1) {
    if (merged[i].start > cursor) {
      return cursor;
    }
    cursor = Math.max(cursor, merged[i].end);
  }
  return cursor < size ? cursor : size;
}

function isRangeCovered(ranges, start, end) {
  const merged = mergeRanges(ranges);
  let cursor = start;
  for (const r of merged) {
    if (r.end <= cursor) continue;
    if (r.start > cursor) return false;
    cursor = Math.max(cursor, r.end);
    if (cursor >= end) return true;
  }
  return cursor >= end;
}

function chooseChunkSizeBytes(speedKbps) {
  if (speedKbps < 20) return 150 * 1024;
  if (speedKbps <= 80) return 300 * 1024;
  return 600 * 1024;
}

async function measureSourceSpeedKbps(source) {
  const started = (typeof performance !== "undefined" ? performance.now() : Date.now());
  const resp = await fetch(`${source}/speed-test?ts=${Date.now()}`, {
    method: "GET",
    cache: "no-store",
  });
  if (!resp.ok) {
    throw new Error("Speed test failed");
  }
  const buffer = await resp.arrayBuffer();
  const ended = (typeof performance !== "undefined" ? performance.now() : Date.now());
  const elapsedSec = Math.max((ended - started) / 1000, 0.001);
  return (buffer.byteLength / 1024) / elapsedSec;
}

async function measureNetworkSpeed(sources, onSource) {
  for (const source of sources) {
    try {
      const speed = await measureSourceSpeedKbps(source);
      onSource?.(source);
      return { speedKbps: speed, source };
    } catch {
      // try next source
    }
  }
  return { speedKbps: SPEED_TEST_FALLBACK_KBPS, source: sources[0] || "" };
}

function buildChunkStatusSnapshot(status, totalChunks) {
  const out = {};
  const size = Number(status?.size || 0);
  if (!size || totalChunks <= 0) return out;
  const ranges = status?.ranges || [];
  const unit = Math.max(1, Math.ceil(size / totalChunks));
  for (let i = 0; i < totalChunks; i += 1) {
    const start = i * unit;
    const end = Math.min(size, start + unit);
    out[`chunk_${i}`] = isRangeCovered(ranges, start, end);
  }
  return out;
}

export function getDownloadChunkStatus(fileUrl, totalChunks = 0) {
  const fileId = parseFileIdFromUrl(fileUrl);
  if (!fileId) return {};
  const status = readDownloadStatus(fileId);
  const derived =
    status?.activeChunkSize && status?.size
      ? Math.max(1, Math.ceil(Number(status.size) / Number(status.activeChunkSize)))
      : 0;
  const t = totalChunks || derived || 0;
  return buildChunkStatusSnapshot(status, t);
}

export async function downloadFileChunked({
  fileUrl,
  onProgress,
  onVerify,
  onSource,
}) {
  const fileId = parseFileIdFromUrl(fileUrl);
  if (!fileId) {
    window.open(fileUrl, "_blank", "noopener,noreferrer");
    return { fallback: true };
  }

  const peerList = await getPeers().catch(() => []);
  const sources = buildPeerSources(peerList);

  let manifest = null;
  for (const source of sources) {
    try {
      const resp = await fetch(`${source}/files/${fileId}/manifest`);
      if (!resp.ok) continue;
      const man = await resp.json();
      if (!man?.id) continue;
      manifest = man;
      onSource?.(source);
      break;
    } catch {
      // try next source
    }
  }
  if (!manifest) throw new Error("Manifest unavailable from peers");

  const fileSize = Number(manifest.size || 0);
  const manifestChunkSize = Math.max(1, Number(manifest.chunkSize || 1024 * 1024));
  if (!fileSize) throw new Error("Invalid file size in manifest");
  const db = await openResumeDB().catch(() => null);
  const statusSeed = readDownloadStatus(fileId);
  let status = statusSeed;
  const freshStatusRequired =
    !status ||
    status.fileId !== fileId ||
    Number(status.size || 0) !== fileSize ||
    String(status.sha256 || "") !== String(manifest.sha256 || "");

  if (freshStatusRequired) {
    status = {
      fileId,
      size: fileSize,
      sha256: manifest.sha256 || "",
      fileName: manifest.fileName || `file-${fileId}`,
      ranges: [],
      activeChunkSize: 300 * 1024,
      speedKbps: SPEED_TEST_FALLBACK_KBPS,
      updatedAt: new Date().toISOString(),
    };
    writeDownloadStatus(fileId, status);
    await idbDeletePrefix(db, `${fileId}:`).catch(() => {});
  }
  if (!Array.isArray(status.ranges)) {
    status.ranges = [];
  }
  status.ranges = status.ranges
    .filter((r) => Number.isFinite(r?.start) && Number.isFinite(r?.end) && r.end > r.start)
    .map((r) => ({
      start: Number(r.start),
      end: Number(r.end),
      key: r.key || rangeKey(fileId, Number(r.start), Number(r.end)),
    }));

  const validRanges = [];
  for (const r of status.ranges) {
    const stored = await idbGet(db, r.key).catch(() => null);
    if (stored instanceof ArrayBuffer && stored.byteLength === r.end - r.start) {
      validRanges.push(r);
    }
  }
  status.ranges = validRanges;
  status.updatedAt = new Date().toISOString();
  writeDownloadStatus(fileId, status);

  const firstMeasure = await measureNetworkSpeed(sources, onSource);
  let speedKbps = Number(firstMeasure.speedKbps || SPEED_TEST_FALLBACK_KBPS);
  let activeChunkSize = chooseChunkSizeBytes(speedKbps);
  status.speedKbps = speedKbps;
  status.activeChunkSize = activeChunkSize;
  writeDownloadStatus(fileId, status);

  let loadedBytes = coveredBytes(status.ranges);
  let resumedChunks = status.ranges.length;
  let segmentsSinceRecheck = 0;
  let lastSpeedCheckAt = Date.now();
  let virtualTotalChunks = Math.max(1, Math.ceil(fileSize / Math.max(activeChunkSize, 1)));

  onProgress?.({
    type: "download",
    loadedBytes,
    totalBytes: fileSize || loadedBytes,
    chunkIndex: Math.max(0, resumedChunks - 1),
    totalChunks: virtualTotalChunks,
    percent: Math.round(
      (loadedBytes / Math.max(fileSize || loadedBytes, 1)) * 100
    ),
    resumedChunks,
    speedKbps: Math.round(speedKbps),
    activeChunkSize,
    chunkStatus: buildChunkStatusSnapshot(status, virtualTotalChunks),
  });

  while (true) {
    const start = nextMissingOffset(status.ranges, fileSize);
    if (start >= fileSize) break;
    const reqSize = Math.min(activeChunkSize, fileSize - start);
    const end = start + reqSize;
    const ownerChunkIndex = Math.floor(start / manifestChunkSize);
    let preferredSources = sources;
    try {
      const ownerPayload = await getChunkOwners(fileId, ownerChunkIndex);
      const ownerSources = ownerSourcesFromPayload(ownerPayload);
      if (ownerSources.length > 0) {
        const seen = new Set();
        preferredSources = [];
        for (const s of [...ownerSources, ...sources]) {
          const n = normalizeSource(s);
          if (seen.has(n)) continue;
          seen.add(n);
          preferredSources.push(n);
        }
      }
    } catch {
      // fall back to discovered peers
    }
    let downloaded = false;
    let finalBuffer = null;
    let finalSource = "";
    for (const source of preferredSources) {
      try {
        const resp = await fetch(
          `${source}/files/${fileId}/range?start=${start}&size=${reqSize}`
        );
        if (!resp.ok) continue;
        const expected = (resp.headers.get("X-Chunk-SHA256") || "")
          .trim()
          .toLowerCase();
        const buffer = await resp.arrayBuffer();
        const actual = await sha256HexFromBuffer(buffer);
        const verifyEnabled = Boolean(actual);
        if (verifyEnabled && expected && expected !== actual) {
          onVerify?.({
            type: "download",
            status: "rejected",
            index: status.ranges.length,
            totalChunks: virtualTotalChunks,
            expected,
            actual,
            source,
          });
          continue;
        }
        onVerify?.({
          type: "download",
          status: verifyEnabled ? "accepted" : "accepted-unverified",
          index: status.ranges.length,
          totalChunks: virtualTotalChunks,
          checksum: actual,
          source,
        });
        finalBuffer = buffer;
        finalSource = source;
        downloaded = true;
        break;
      } catch {
        // try next source
      }
    }
    if (!downloaded) {
      throw new Error(`Chunk range ${start}-${end} unavailable from peers`);
    }
    onSource?.(finalSource);
    const key = rangeKey(fileId, start, end);
    await idbPut(db, key, finalBuffer).catch(() => {});
    status.ranges.push({ start, end, key });
    loadedBytes = coveredBytes(status.ranges);
    segmentsSinceRecheck += 1;
    status.updatedAt = new Date().toISOString();
    writeDownloadStatus(fileId, status);

    const chunkIndex = status.ranges.length - 1;
    virtualTotalChunks = Math.max(1, Math.ceil(fileSize / Math.max(activeChunkSize, 1)));
    onProgress?.({
      type: "download",
      loadedBytes,
      totalBytes: fileSize || loadedBytes,
      chunkIndex,
      totalChunks: virtualTotalChunks,
      percent: Math.round((loadedBytes / Math.max(fileSize || loadedBytes, 1)) * 100),
      resumedChunks,
      speedKbps: Math.round(speedKbps),
      activeChunkSize,
      chunkStatus: buildChunkStatusSnapshot(status, virtualTotalChunks),
    });

    const now = Date.now();
    if (
      segmentsSinceRecheck >= SPEED_RECHECK_SEGMENTS ||
      now - lastSpeedCheckAt >= SPEED_RECHECK_INTERVAL_MS
    ) {
      const nextMeasure = await measureNetworkSpeed(sources, onSource);
      speedKbps = Number(nextMeasure.speedKbps || speedKbps || SPEED_TEST_FALLBACK_KBPS);
      activeChunkSize = chooseChunkSizeBytes(speedKbps);
      status.speedKbps = speedKbps;
      status.activeChunkSize = activeChunkSize;
      status.updatedAt = new Date().toISOString();
      writeDownloadStatus(fileId, status);
      segmentsSinceRecheck = 0;
      lastSpeedCheckAt = now;
    }
  }

  const merged = mergeRanges(status.ranges);
  if (!isRangeCovered(merged, 0, fileSize)) {
    throw new Error("Download incomplete. Missing ranges remain.");
  }

  const finalBytes = new Uint8Array(fileSize);
  for (const r of status.ranges) {
    const stored = await idbGet(db, r.key).catch(() => null);
    if (!(stored instanceof ArrayBuffer)) continue;
    const data = new Uint8Array(stored);
    finalBytes.set(data, r.start);
  }

  const blob = new Blob([finalBytes], { type: "application/octet-stream" });
  if (manifest.sha256 && hasCryptoDigest()) {
    const actualFileHash = await sha256HexFromBlob(blob);
    if (actualFileHash !== String(manifest.sha256).toLowerCase()) {
      throw new Error("Final file checksum mismatch");
    }
  }

  const downloadName = manifest.fileName || `file-${fileId}`;
  const href = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = href;
  anchor.download = downloadName;
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(href);
  clearDownloadStatus(fileId);
  await idbDeletePrefix(db, `${fileId}:`).catch(() => {});
  return {
    fileId,
    fileName: downloadName,
    size: blob.size,
    resumedChunks,
    speedKbps: Math.round(speedKbps),
    activeChunkSize,
  };
}
