import { getBase } from "./api";

const API_BASE = getBase();
// Join a room
export async function joinRoomLocal(det) {
  const res = await fetch(`${API_BASE}/join_room`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(det),
  });
  if (!res.ok) throw new Error("Failed to join room");
  return await res.text();
}

// Upload a post with file
export async function uploadPostLocal({ id, code, content, file }) {
  const formData = new FormData();
  formData.append("id", id);
  formData.append("code", code);
  formData.append("content", content);

  if (typeof file === "string") {
    // Convert URL to File object
    const response = await fetch(getBase() + file);
    const blob = await response.blob();
    const filename = file.split("/").pop() || "upload.dat";
    const fileObject = new File([blob], filename, { type: blob.type });
    formData.append("file", fileObject);
  } else {
    // Direct File object
    formData.append("file", file);
  }

  const res = await fetch(`${API_BASE}/upload`, {
    method: "POST",
    body: formData,
  });

  if (!res.ok) throw new Error("Failed to upload post");
  return await res.text();
}

// Get posts for a room
export async function getPostsLocal(code) {
  const res = await fetch(`${API_BASE}/posts`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ code }),
  });

  if (!res.ok) throw new Error("Failed to fetch posts");
  return await res.json();
}

// Create an announcement
export async function createAnnouncementLocal({
  id,
  code,
  title,
  description,
}) {
  const res = await fetch(`${API_BASE}/announce`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ id, code, title, description }),
  });

  if (!res.ok) throw new Error("Failed to create announcement");
  return await res.text();
}

// Get announcements for a room
export async function getAnnouncementsLocal(code) {
  const res = await fetch(`${API_BASE}/getAnnouncements`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ code }),
  });

  if (!res.ok) throw new Error("Failed to fetch announcements");
  return await res.json();
}

// Get joined rooms
export async function getJoinedRoomsLocal() {
  const res = await fetch(`${API_BASE}/roomsof`);
  if (!res.ok) throw new Error("Failed to get joined rooms");
  console.log(res);
  return await res.json();
}

export async function getRoomById(roomId) {
  try {
    const response = await fetch(
      `${API_BASE}/room?id=${encodeURIComponent(roomId)}`
    );
    if (!response.ok) {
      throw new Error(`Error ${response.status}: ${await response.text()}`);
    }
    const room = await response.json();
    return room; // { id, title, description, teacher }
  } catch (err) {
    console.error("Failed to fetch room:", err);
    return null;
  }
}
