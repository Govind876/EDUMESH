package main

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/ascii85"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/klauspost/compress/zstd"
	_ "modernc.org/sqlite"
)

const (
	dbName              = "local_user.db"
	udpPort             = 9999
	defaultHTTPPort     = ":8080"
	broadcastInt        = 30 * time.Second
	syncInt             = 1 * time.Minute
	uploadDir           = "./uploads"
	compressedDir       = "./uploads/.compressed"
	chunkTempDir        = "./uploads/.chunks"
	nodeIDFile          = "node_id.txt"
	sharedKeyFile       = "shared.key"
	chunkSize           = 1024 * 1024
	vcdSessionTTL       = 10 * time.Minute
	speedTestSize       = 200 * 1024
	vcdArtifactDir      = "./uploads/vcd"
	janitorInt          = 24 * time.Hour
	storageSoftLimitPct = 90
	smsPrefix           = "SchoolSync:"
	smsPartTextLimit    = 130
	smsRawChunkSize     = 96
)

var mu sync.Mutex
var peerMu sync.Mutex
var peers = map[string]*Peer{}
var uploadSessionMu sync.Mutex
var uploadSessions = map[string]*UploadSession{}
var nodeID string
var sharedKey []byte
var discoveryURL string
var advertiseHost string
var httpPort string
var vcdShareMu sync.Mutex
var vcdShare *VCDShareSession

//go:embed frontend/build/**
var embeddedFS embed.FS

type Peer struct {
	ID       string    `json:"id"`
	IP       string    `json:"ip"`
	Host     string    `json:"host"`
	Port     int       `json:"port"`
	LastSeen time.Time `json:"lastSeen"`
}

type Room struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Teacher     string `json:"teacher"`
}

type Post struct {
	ID      string `json:"id"`
	Code    string `json:"code"`
	Content string `json:"content"`
	FileURL string `json:"fileUrl"`
}

type FileMeta struct {
	ID             string `json:"id"`
	FileName       string `json:"fileName"`
	Size           int64  `json:"size"`
	RawSize        int64  `json:"rawSize"`
	CompressedSize int64  `json:"compressedSize"`
	Compression    string `json:"compression"`
	SHA256         string `json:"sha256"`
	Path           string `json:"-"`
	CompressedPath string `json:"-"`
	IsCached       bool   `json:"isCached"`
}

type UploadSession struct {
	ID         string
	PostID     string
	Code       string
	Content    string
	FileName   string
	FileSize   int64
	TotalChunk int
	FileSHA256 string
	Received   map[int]string
	CreatedAt  time.Time
}

type Announcement struct {
	ID          string `json:"id"`
	Code        string `json:"code"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type JoinRequest struct {
	ID        string `json:"id"`
	RoomID    string `json:"roomId"`
	Student   string `json:"student"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
}

type Member struct {
	RoomID  string `json:"roomId"`
	Student string `json:"student"`
}

type Assignment struct {
	ID          string `json:"id"`
	Code        string `json:"code"`
	Title       string `json:"title"`
	Description string `json:"description"`
	CreatedBy   string `json:"createdBy"`
	CreatedAt   string `json:"createdAt"`
}

type AssignmentSubmission struct {
	ID           string          `json:"id"`
	AssignmentID string          `json:"assignmentId"`
	Code         string          `json:"code"`
	Student      string          `json:"student"`
	Answer       string          `json:"answer"`
	Status       string          `json:"status"`
	FileName     string          `json:"fileName"`
	FileType     string          `json:"fileType"`
	FileSize     int64           `json:"fileSize"`
	FileSHA256   string          `json:"fileSha256"`
	Chunks       json.RawMessage `json:"chunks"`
	SubmittedAt  string          `json:"submittedAt"`
}

type ChunkOwner struct {
	FileID     string `json:"fileId"`
	ChunkIndex int    `json:"chunkIndex"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	UpdatedAt  string `json:"updatedAt"`
}

type SMSPrepareRequest struct {
	FileID    string `json:"fileId"`
	FileName  string `json:"fileName"`
	Text      string `json:"text"`
	Target    string `json:"target"`
	Compress  bool   `json:"compress"`
	UseBase64 bool   `json:"useBase64"`
}

type SMSPrepareResponse struct {
	TransferID  string   `json:"transferId"`
	FileName    string   `json:"fileName"`
	Parts       int      `json:"parts"`
	Encoding    string   `json:"encoding"`
	Compression string   `json:"compression"`
	Messages    []string `json:"messages"`
}

type ExportPayload struct {
	NodeID        string                 `json:"nodeId"`
	Rooms         []Room                 `json:"rooms"`
	Posts         []Post                 `json:"posts"`
	Announcements []Announcement         `json:"announcements"`
	Files         []FileMeta             `json:"files"`
	JoinRequests  []JoinRequest          `json:"joinRequests"`
	Members       []Member               `json:"members"`
	Assignments   []Assignment           `json:"assignments"`
	Submissions   []AssignmentSubmission `json:"submissions"`
}

type VCDStartRequest struct {
	SSID            string `json:"ssid"`
	Password        string `json:"password"`
	ServerIP        string `json:"serverIp"`
	ArtifactPath    string `json:"artifactPath"`
	EnableBluetooth bool   `json:"enableBluetoothFallback"`
}

type VCDShareSession struct {
	Token           string       `json:"token"`
	ExecutableName  string       `json:"executableName"`
	ExecutableSize  int64        `json:"executableSize"`
	ServerIP        string       `json:"serverIp"`
	Port            int          `json:"port"`
	SSID            string       `json:"ssid"`
	Password        string       `json:"password"`
	DownloadURL     string       `json:"downloadUrl"`
	QRPayload       string       `json:"qrPayload"`
	ExpiresAt       time.Time    `json:"expiresAt"`
	EnableBluetooth bool         `json:"enableBluetoothFallback"`
	server          *http.Server `json:"-"`
	listener        net.Listener `json:"-"`
	stopTimer       *time.Timer  `json:"-"`
}

func main() {
	nodeID = loadOrCreateNodeID()
	sharedKey = loadSharedKey()
	discoveryURL = strings.TrimSpace(os.Getenv("RDE_DISCOVERY_URL"))
	advertiseHost = strings.TrimSpace(os.Getenv("RDE_ADVERTISE_HOST"))
	httpPort = resolveHTTPPort()
	os.MkdirAll(uploadDir, os.ModePerm)
	os.MkdirAll(compressedDir, os.ModePerm)
	os.MkdirAll(chunkTempDir, os.ModePerm)
	os.MkdirAll(vcdArtifactDir, os.ModePerm)
	db := initDB()
	defer db.Close()
	go startStorageJanitor(db)

	go startUDPListener(db)
	go func() {
		for {
			broadcastPresence()
			time.Sleep(broadcastInt)
		}
	}()
	go func() {
		for {
			syncAllPeers(db)
			time.Sleep(syncInt)
		}
	}()
	if discoveryURL != "" {
		if advertiseHost == "" {
			advertiseHost = getLocalIP()
		}
		go func() {
			for {
				announceDiscovery(db)
				time.Sleep(30 * time.Second)
			}
		}()
	}

	http.HandleFunc("/join_room", func(w http.ResponseWriter, r *http.Request) {
		handleJoinRoom(w, r, db)
	})
	http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		handleUploadPost(w, r, db)
	})
	http.HandleFunc("/upload/start", func(w http.ResponseWriter, r *http.Request) {
		handleUploadStart(w, r)
	})
	http.HandleFunc("/upload/chunk", handleUploadChunk)
	http.HandleFunc("/upload/finish", func(w http.ResponseWriter, r *http.Request) {
		handleUploadFinish(w, r, db)
	})
	http.HandleFunc("/posts", func(w http.ResponseWriter, r *http.Request) {
		handleGetPosts(w, r, db)
	})
	http.HandleFunc("/announce", func(w http.ResponseWriter, r *http.Request) {
		handleAnnounce(w, r, db)
	})
	http.HandleFunc("/getAnnouncements", func(w http.ResponseWriter, r *http.Request) {
		handleGetAnnouncements(w, r, db)
	})
	http.HandleFunc("/roomsof", func(w http.ResponseWriter, r *http.Request) {
		handleGetJoinedRooms(w, r, db)
	})
	http.HandleFunc("/room", func(w http.ResponseWriter, r *http.Request) {
		handleGetRoomByID(w, r, db)
	})
	http.HandleFunc("/request_join", func(w http.ResponseWriter, r *http.Request) {
		handleRequestJoin(w, r, db)
	})
	http.HandleFunc("/join_requests", func(w http.ResponseWriter, r *http.Request) {
		handleGetJoinRequests(w, r, db)
	})
	http.HandleFunc("/approve_join", func(w http.ResponseWriter, r *http.Request) {
		handleApproveJoin(w, r, db)
	})
	http.HandleFunc("/membership", func(w http.ResponseWriter, r *http.Request) {
		handleMembership(w, r, db)
	})
	http.HandleFunc("/members/list", func(w http.ResponseWriter, r *http.Request) {
		handleListMembers(w, r, db)
	})
	http.HandleFunc("/room_stats", func(w http.ResponseWriter, r *http.Request) {
		handleRoomStats(w, r, db)
	})
	http.HandleFunc("/classroom/delete", func(w http.ResponseWriter, r *http.Request) {
		handleDeleteClassroom(w, r, db)
	})
	http.HandleFunc("/assignments/create", func(w http.ResponseWriter, r *http.Request) {
		handleCreateAssignment(w, r, db)
	})
	http.HandleFunc("/assignments/list", func(w http.ResponseWriter, r *http.Request) {
		handleListAssignments(w, r, db)
	})
	http.HandleFunc("/assignments/submit", func(w http.ResponseWriter, r *http.Request) {
		handleSubmitAssignment(w, r, db)
	})
	http.HandleFunc("/assignments/submissions", func(w http.ResponseWriter, r *http.Request) {
		handleListSubmissions(w, r, db)
	})
	http.HandleFunc("/downloads/status", func(w http.ResponseWriter, r *http.Request) {
		handleDownloadStatus(w, r, db)
	})
	http.HandleFunc("/chunks/announce", func(w http.ResponseWriter, r *http.Request) {
		handleChunkAnnounce(w, r, db)
	})
	http.HandleFunc("/chunks/owners", func(w http.ResponseWriter, r *http.Request) {
		handleChunkOwners(w, r, db)
	})
	http.HandleFunc("/sms/prepare", func(w http.ResponseWriter, r *http.Request) {
		handleSMSPrepare(w, r, db)
	})
	http.HandleFunc("/sms/ingest", func(w http.ResponseWriter, r *http.Request) {
		handleSMSIngest(w, r, db)
	})
	http.HandleFunc("/sms/status", func(w http.ResponseWriter, r *http.Request) {
		handleSMSStatus(w, r, db)
	})
	http.HandleFunc("/sms/reassemble", func(w http.ResponseWriter, r *http.Request) {
		handleSMSReassemble(w, r, db)
	})
	http.HandleFunc("/vcd/start", handleVCDStart)
	http.HandleFunc("/vcd/upload-artifact", handleVCDUploadArtifact)
	http.HandleFunc("/vcd/status", handleVCDStatus)
	http.HandleFunc("/vcd/stop", handleVCDStop)
	http.HandleFunc("/health", handleHealth)
	http.HandleFunc("/network/info", handleNetworkInfo)
	http.HandleFunc("/speed-test", handleSpeedTest)
	http.HandleFunc("/export", func(w http.ResponseWriter, r *http.Request) {
		handleExport(w, r, db)
	})
	http.HandleFunc("/export_enc", func(w http.ResponseWriter, r *http.Request) {
		handleExportEncrypted(w, r, db)
	})
	http.HandleFunc("/peers", handlePeers)
	http.HandleFunc("/files/", func(w http.ResponseWriter, r *http.Request) {
		handleFiles(w, r, db)
	})
	http.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(uploadDir))))
	if sub, err := fs.Sub(embeddedFS, "frontend/build"); err == nil {
		http.Handle("/", spaHandler(sub))
	} else {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK - backend running. Try /roomsof, /peers, /health or /room?id=..."))
		})
	}
	log.Println("Server listening on", httpPort)
	log.Fatal(http.ListenAndServe(httpPort, enableCORS(http.DefaultServeMux)))
}

func enableCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Adjust these headers as needed for your frontend
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-RDE-ENC")

		// Handle preflight request
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		h.ServeHTTP(w, r)
	})
}

func spaHandler(content fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			http.NotFound(w, r)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		f, err := content.Open(path)
		if err != nil {
			// SPA fallback
			f, err = content.Open("index.html")
			if err != nil {
				http.NotFound(w, r)
				return
			}
			defer f.Close()
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			io.Copy(w, f)
			return
		}
		defer f.Close()
		if ctype := mime.TypeByExtension(filepath.Ext(path)); ctype != "" {
			w.Header().Set("Content-Type", ctype)
		}
		io.Copy(w, f)
	})
}

func loadOrCreateNodeID() string {
	if b, err := os.ReadFile(nodeIDFile); err == nil {
		id := strings.TrimSpace(string(b))
		if id != "" {
			return id
		}
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("node-%d", time.Now().UnixNano())
	}
	id := hex.EncodeToString(buf)
	_ = os.WriteFile(nodeIDFile, []byte(id), 0o600)
	return id
}

func loadSharedKey() []byte {
	if v := strings.TrimSpace(os.Getenv("RDE_SHARED_KEY")); v != "" {
		sum := sha256.Sum256([]byte(v))
		return sum[:]
	}
	if b, err := os.ReadFile(sharedKeyFile); err == nil {
		v := strings.TrimSpace(string(b))
		if v != "" {
			sum := sha256.Sum256([]byte(v))
			return sum[:]
		}
	}
	defaultKey := "change-me-shared-key"
	_ = os.WriteFile(sharedKeyFile, []byte(defaultKey), 0o600)
	sum := sha256.Sum256([]byte(defaultKey))
	return sum[:]
}

func resolveHTTPPort() string {
	raw := strings.TrimSpace(os.Getenv("RDE_HTTP_PORT"))
	if raw == "" {
		return defaultHTTPPort
	}
	if strings.HasPrefix(raw, ":") {
		return raw
	}
	if _, err := strconv.Atoi(raw); err == nil {
		return ":" + raw
	}
	return defaultHTTPPort
}

func encryptJSON(payload []byte) (string, string, error) {
	if len(sharedKey) == 0 {
		return "", "", fmt.Errorf("shared key missing")
	}
	block, err := aes.NewCipher(sharedKey)
	if err != nil {
		return "", "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", "", err
	}
	ciphertext := gcm.Seal(nil, nonce, payload, nil)
	return base64.StdEncoding.EncodeToString(nonce), base64.StdEncoding.EncodeToString(ciphertext), nil
}

func decryptJSON(nonceB64, dataB64 string) ([]byte, error) {
	if len(sharedKey) == 0 {
		return nil, fmt.Errorf("shared key missing")
	}
	nonce, err := base64.StdEncoding.DecodeString(nonceB64)
	if err != nil {
		return nil, err
	}
	data, err := base64.StdEncoding.DecodeString(dataB64)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(sharedKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, nonce, data, nil)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"nodeId": nodeID,
		"time":   time.Now().Format(time.RFC3339),
	})
}

func handleNetworkInfo(w http.ResponseWriter, r *http.Request) {
	host := strings.TrimSpace(advertiseHost)
	if host == "" {
		host = getLocalIP()
	}
	if host == "" {
		host = "localhost"
	}
	port := strings.TrimPrefix(httpPort, ":")
	if strings.TrimSpace(port) == "" {
		port = "8080"
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"host": host,
		"port": port,
	})
}

func handleSpeedTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.Itoa(speedTestSize))
	if r.Method == http.MethodHead {
		return
	}
	buf := make([]byte, speedTestSize)
	for i := range buf {
		buf[i] = byte((i * 31) % 251)
	}
	w.Write(buf)
}

func handlePeers(w http.ResponseWriter, r *http.Request) {
	peerMu.Lock()
	defer peerMu.Unlock()
	list := make([]*Peer, 0, len(peers))
	for _, p := range peers {
		list = append(list, p)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

func handleVCDStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req VCDStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	artifactPath := strings.TrimSpace(req.ArtifactPath)
	if artifactPath == "" {
		http.Error(w, "Artifact path is required.", http.StatusBadRequest)
		return
	}
	artifactPath = filepath.Clean(artifactPath)

	info, err := os.Stat(artifactPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "Artifact file not found. Check the artifact path.", http.StatusBadRequest)
			return
		}
		http.Error(w, "Unable to read artifact metadata", http.StatusInternalServerError)
		return
	}
	if info.IsDir() {
		http.Error(w, "Artifact path is invalid", http.StatusBadRequest)
		return
	}
	if info.Size() <= 0 {
		http.Error(w, "Artifact file is empty", http.StatusBadRequest)
		return
	}

	serverIP := strings.TrimSpace(req.ServerIP)
	if serverIP == "" {
		serverIP = getLocalIP()
	}
	token := makeRandomToken()
	artifactName := filepath.Base(artifactPath)
	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(artifactName)))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	mux := http.NewServeMux()
	serveArtifact := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", artifactName))
		http.ServeFile(w, r, artifactPath)
	}
	mux.HandleFunc("/download", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("token") != token {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		serveArtifact(w, r)
	})
	mux.HandleFunc("/download/", func(w http.ResponseWriter, r *http.Request) {
		pathToken := strings.TrimPrefix(r.URL.Path, "/download/")
		if strings.TrimSpace(pathToken) != token {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		serveArtifact(w, r)
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		http.Error(w, "Unable to start VCD server", http.StatusInternalServerError)
		return
	}
	port := 0
	if tcpAddr, ok := ln.Addr().(*net.TCPAddr); ok {
		port = tcpAddr.Port
	}
	if port == 0 {
		_ = ln.Close()
		http.Error(w, "Unable to resolve VCD port", http.StatusInternalServerError)
		return
	}

	srv := &http.Server{
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	downloadURL := fmt.Sprintf("http://%s:%d/download/%s", serverIP, port, token)
	qrPayload := buildVCDQRPayload(downloadURL, serverIP, port, req.SSID, req.Password, req.EnableBluetooth)
	session := &VCDShareSession{
		Token:           token,
		ExecutableName:  artifactName,
		ExecutableSize:  info.Size(),
		ServerIP:        serverIP,
		Port:            port,
		SSID:            strings.TrimSpace(req.SSID),
		Password:        strings.TrimSpace(req.Password),
		DownloadURL:     downloadURL,
		QRPayload:       qrPayload,
		ExpiresAt:       time.Now().Add(vcdSessionTTL),
		EnableBluetooth: req.EnableBluetooth,
		server:          srv,
		listener:        ln,
	}

	vcdShareMu.Lock()
	old := vcdShare
	vcdShare = session
	vcdShareMu.Unlock()
	if old != nil {
		stopVCDSession(old)
	}

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("VCD server error: %v", err)
		}
	}()
	session.stopTimer = time.AfterFunc(vcdSessionTTL, func() {
		vcdShareMu.Lock()
		current := vcdShare
		if current != nil && current.Token == session.Token {
			vcdShare = nil
		}
		vcdShareMu.Unlock()
		stopVCDSession(session)
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(session)
}

func handleVCDUploadArtifact(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseMultipartForm(700 << 20); err != nil {
		http.Error(w, "Invalid upload form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("artifact")
	if err != nil {
		http.Error(w, "artifact file is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	originalName := strings.TrimSpace(filepath.Base(header.Filename))
	if originalName == "" {
		http.Error(w, "Invalid file name", http.StatusBadRequest)
		return
	}

	if err := os.MkdirAll(vcdArtifactDir, os.ModePerm); err != nil {
		http.Error(w, "Unable to prepare upload directory", http.StatusInternalServerError)
		return
	}

	safeName := sanitizeArtifactName(strings.TrimSuffix(originalName, filepath.Ext(originalName)))
	if safeName == "" {
		safeName = "artifact"
	}
	ext := strings.ToLower(filepath.Ext(originalName))
	if strings.TrimSpace(ext) == "" {
		ext = ".bin"
	}
	fileName := fmt.Sprintf("%d-%s-%s%s", time.Now().Unix(), makeRandomToken()[:8], safeName, ext)
	dstPath := filepath.Join(vcdArtifactDir, fileName)

	dst, err := os.Create(dstPath)
	if err != nil {
		http.Error(w, "Unable to store artifact", http.StatusInternalServerError)
		return
	}
	size, copyErr := io.Copy(dst, file)
	closeErr := dst.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(dstPath)
		http.Error(w, "Unable to store artifact", http.StatusInternalServerError)
		return
	}
	if size <= 0 {
		_ = os.Remove(dstPath)
		http.Error(w, "Uploaded artifact is empty", http.StatusBadRequest)
		return
	}

	absPath, err := filepath.Abs(dstPath)
	if err != nil {
		absPath = dstPath
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"artifactPath": absPath,
		"fileName":     originalName,
		"size":         size,
	})
}

func sanitizeArtifactName(name string) string {
	clean := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, strings.TrimSpace(name))
	clean = strings.Trim(clean, "-_")
	if clean == "" {
		return ""
	}
	return clean
}

func handleVCDStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	vcdShareMu.Lock()
	defer vcdShareMu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if vcdShare == nil {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"active": false})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"active":  true,
		"session": vcdShare,
	})
}

func handleVCDStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	vcdShareMu.Lock()
	session := vcdShare
	vcdShare = nil
	vcdShareMu.Unlock()
	if session != nil {
		stopVCDSession(session)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"stopped": true})
}

func stopVCDSession(session *VCDShareSession) {
	if session == nil {
		return
	}
	if session.stopTimer != nil {
		session.stopTimer.Stop()
	}
	if session.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = session.server.Shutdown(ctx)
	}
	if session.listener != nil {
		_ = session.listener.Close()
	}
}

func makeRandomToken() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

func buildVCDQRPayload(downloadURL, ip string, port int, ssid, password string, bluetooth bool) string {
	payload := map[string]interface{}{
		"type":        "schoolsync-vcd",
		"downloadUrl": downloadURL,
		"serverIp":    ip,
		"port":        port,
		"ssid":        strings.TrimSpace(ssid),
		"password":    strings.TrimSpace(password),
		"bluetoothFallback": map[string]interface{}{
			"enabled": bluetooth,
			"mode":    "OPP",
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return downloadURL
	}
	return string(raw)
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "127.0.0.1"
}

func handleFiles(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/files/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	fileID := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	meta, err := getFileMeta(db, fileID)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	switch action {
	case "manifest":
		_ = touchFileAccess(db, meta.ID, nil)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":             meta.ID,
			"fileName":       meta.FileName,
			"size":           meta.Size,
			"rawSize":        meta.RawSize,
			"compressedSize": meta.CompressedSize,
			"compression":    meta.Compression,
			"sha256":         meta.SHA256,
			"chunkSize":      chunkSize,
		})
		return
	case "chunk":
		indexStr := r.URL.Query().Get("index")
		if indexStr == "" {
			http.Error(w, "Missing index", http.StatusBadRequest)
			return
		}
		index, err := strconv.Atoi(indexStr)
		if err != nil || index < 0 {
			http.Error(w, "Invalid index", http.StatusBadRequest)
			return
		}
		enc := r.URL.Query().Get("enc") == "1"
		if enc && len(sharedKey) == 0 {
			http.Error(w, "Shared key not configured", http.StatusBadRequest)
			return
		}
		if err := ensureFilePresent(db, meta); err != nil {
			http.Error(w, "File is not cached locally", http.StatusServiceUnavailable)
			return
		}
		f, err := os.Open(meta.Path)
		if err != nil {
			http.Error(w, "File open error", http.StatusInternalServerError)
			return
		}
		defer f.Close()
		offset := int64(index) * chunkSize
		if offset >= meta.Size {
			http.Error(w, "Index out of range", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			http.Error(w, "Seek error", http.StatusInternalServerError)
			return
		}
		limit := int64(chunkSize)
		if offset+limit > meta.Size {
			limit = meta.Size - offset
		}
		buf := make([]byte, limit)
		if _, err := io.ReadFull(f, buf); err != nil && err != io.EOF {
			http.Error(w, "Read error", http.StatusInternalServerError)
			return
		}
		chunkHash := sha256.Sum256(buf)
		w.Header().Set("X-Chunk-SHA256", hex.EncodeToString(chunkHash[:]))
		_ = touchFileAccess(db, meta.ID, &index)
		if enc {
			nonce, data, err := encryptJSON(buf)
			if err != nil {
				http.Error(w, "Encrypt error", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"nonce": nonce,
				"data":  data,
			})
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(buf)
		return
	case "range":
		startStr := r.URL.Query().Get("start")
		sizeStr := r.URL.Query().Get("size")
		if startStr == "" || sizeStr == "" {
			http.Error(w, "Missing start or size", http.StatusBadRequest)
			return
		}
		start, err := strconv.ParseInt(startStr, 10, 64)
		if err != nil || start < 0 {
			http.Error(w, "Invalid start", http.StatusBadRequest)
			return
		}
		reqSize, err := strconv.ParseInt(sizeStr, 10, 64)
		if err != nil || reqSize <= 0 {
			http.Error(w, "Invalid size", http.StatusBadRequest)
			return
		}
		// Keep per-request memory bounded while supporting adaptive chunk sizes.
		if reqSize > 2*1024*1024 {
			reqSize = 2 * 1024 * 1024
		}
		if start >= meta.Size {
			http.Error(w, "Start out of range", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		if err := ensureFilePresent(db, meta); err != nil {
			http.Error(w, "File is not cached locally", http.StatusServiceUnavailable)
			return
		}
		available := meta.Size - start
		if reqSize > available {
			reqSize = available
		}

		f, err := os.Open(meta.Path)
		if err != nil {
			http.Error(w, "File open error", http.StatusInternalServerError)
			return
		}
		defer f.Close()
		if _, err := f.Seek(start, io.SeekStart); err != nil {
			http.Error(w, "Seek error", http.StatusInternalServerError)
			return
		}
		buf := make([]byte, reqSize)
		if _, err := io.ReadFull(f, buf); err != nil && err != io.EOF {
			http.Error(w, "Read error", http.StatusInternalServerError)
			return
		}
		chunkHash := sha256.Sum256(buf)
		w.Header().Set("X-Chunk-SHA256", hex.EncodeToString(chunkHash[:]))
		w.Header().Set("X-Range-Start", strconv.FormatInt(start, 10))
		w.Header().Set("X-Range-End", strconv.FormatInt(start+int64(len(buf)), 10))
		w.Header().Set("Content-Type", "application/octet-stream")
		chunkIndex := int(start / chunkSize)
		_ = touchFileAccess(db, meta.ID, &chunkIndex)
		w.Write(buf)
		return
	case "download":
		if err := ensureFilePresent(db, meta); err != nil {
			http.Error(w, "File is not cached locally", http.StatusServiceUnavailable)
			return
		}
		_ = touchFileAccess(db, meta.ID, nil)
		http.ServeFile(w, r, meta.Path)
		return
	default:
		http.Redirect(w, r, "/files/"+fileID+"/download", http.StatusFound)
		return
	}
}

func getFileMeta(db *sql.DB, fileID string) (*FileMeta, error) {
	row := db.QueryRow("SELECT id, filename, size, sha256, path, raw_size, compressed_size, compressed_path, compression, is_cached FROM files WHERE id = ?", fileID)
	var id, name, sha, path, compressedPath, compression string
	var size, rawSize, compressedSize int64
	var isCached int
	if err := row.Scan(&id, &name, &size, &sha, &path, &rawSize, &compressedSize, &compressedPath, &compression, &isCached); err != nil {
		return nil, err
	}
	if rawSize <= 0 {
		rawSize = size
	}
	if compressedSize <= 0 {
		compressedSize = size
	}
	if strings.TrimSpace(compression) == "" {
		compression = "none"
	}
	return &FileMeta{
		ID:             id,
		FileName:       name,
		Size:           size,
		RawSize:        rawSize,
		CompressedSize: compressedSize,
		Compression:    compression,
		SHA256:         sha,
		Path:           path,
		CompressedPath: compressedPath,
		IsCached:       isCached == 1,
	}, nil
}

func handleExport(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	payload := buildExportPayload(db)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payload)
}

func handleExportEncrypted(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if len(sharedKey) == 0 {
		http.Error(w, "Shared key not configured", http.StatusBadRequest)
		return
	}
	payload := buildExportPayload(db)
	raw, _ := json.Marshal(payload)
	nonce, data, err := encryptJSON(raw)
	if err != nil {
		http.Error(w, "Encrypt error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"nonce": nonce,
		"data":  data,
	})
}

func buildExportPayload(db *sql.DB) ExportPayload {
	rooms := []Room{}
	posts := []Post{}
	anns := []Announcement{}
	files := []FileMeta{}
	reqs := []JoinRequest{}
	members := []Member{}
	assignments := []Assignment{}
	submissions := []AssignmentSubmission{}

	rows, err := db.Query("SELECT id, title, description, teacher FROM rooms")
	if err == nil {
		for rows.Next() {
			var r Room
			rows.Scan(&r.ID, &r.Title, &r.Description, &r.Teacher)
			rooms = append(rooms, r)
		}
		rows.Close()
	}

	rows, err = db.Query("SELECT id, code, content, file_url FROM posts")
	if err == nil {
		for rows.Next() {
			var p Post
			rows.Scan(&p.ID, &p.Code, &p.Content, &p.FileURL)
			posts = append(posts, p)
		}
		rows.Close()
	}

	rows, err = db.Query("SELECT id, code, title, description FROM announcements")
	if err == nil {
		for rows.Next() {
			var a Announcement
			rows.Scan(&a.ID, &a.Code, &a.Title, &a.Description)
			anns = append(anns, a)
		}
		rows.Close()
	}
	rows, err = db.Query("SELECT id, filename, size, sha256 FROM files")
	if err == nil {
		for rows.Next() {
			var f FileMeta
			rows.Scan(&f.ID, &f.FileName, &f.Size, &f.SHA256)
			files = append(files, f)
		}
		rows.Close()
	}
	rows, err = db.Query("SELECT id, room_id, student, status, created_at FROM join_requests")
	if err == nil {
		for rows.Next() {
			var jr JoinRequest
			rows.Scan(&jr.ID, &jr.RoomID, &jr.Student, &jr.Status, &jr.CreatedAt)
			reqs = append(reqs, jr)
		}
		rows.Close()
	}
	rows, err = db.Query("SELECT room_id, student FROM members")
	if err == nil {
		for rows.Next() {
			var m Member
			rows.Scan(&m.RoomID, &m.Student)
			members = append(members, m)
		}
		rows.Close()
	}
	rows, err = db.Query("SELECT id, code, title, description, created_by, created_at FROM assignments")
	if err == nil {
		for rows.Next() {
			var a Assignment
			rows.Scan(&a.ID, &a.Code, &a.Title, &a.Description, &a.CreatedBy, &a.CreatedAt)
			assignments = append(assignments, a)
		}
		rows.Close()
	}
	rows, err = db.Query("SELECT id, assignment_id, code, student, answer, status, file_name, file_type, file_size, file_sha256, chunks, submitted_at FROM assignment_submissions")
	if err == nil {
		for rows.Next() {
			var s AssignmentSubmission
			var chunks string
			rows.Scan(&s.ID, &s.AssignmentID, &s.Code, &s.Student, &s.Answer, &s.Status, &s.FileName, &s.FileType, &s.FileSize, &s.FileSHA256, &chunks, &s.SubmittedAt)
			if strings.TrimSpace(chunks) == "" {
				chunks = "[]"
			}
			s.Chunks = json.RawMessage(chunks)
			submissions = append(submissions, s)
		}
		rows.Close()
	}

	return ExportPayload{
		NodeID:        nodeID,
		Rooms:         rooms,
		Posts:         posts,
		Announcements: anns,
		Files:         files,
		JoinRequests:  reqs,
		Members:       members,
		Assignments:   assignments,
		Submissions:   submissions,
	}
}

// --- DB Setup & Models ---

func initDB() *sql.DB {
	db, err := sql.Open("sqlite", dbName)
	if err != nil {
		log.Fatal(err)
	}
	// Reduce SQLITE_BUSY errors under concurrent read/write traffic.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.Exec("PRAGMA busy_timeout = 5000;"); err != nil {
		log.Printf("failed to set sqlite busy_timeout: %v", err)
	}
	if _, err := db.Exec("PRAGMA journal_mode = WAL;"); err != nil {
		log.Printf("failed to set sqlite WAL mode: %v", err)
	}
	if _, err := db.Exec("PRAGMA synchronous = NORMAL;"); err != nil {
		log.Printf("failed to set sqlite synchronous mode: %v", err)
	}

	create := func(query string) {
		if _, err := db.Exec(query); err != nil {
			log.Fatal(err)
		}
	}
	bestEffort := func(query string) {
		if _, err := db.Exec(query); err != nil {
			msg := strings.ToLower(strings.TrimSpace(err.Error()))
			if strings.Contains(msg, "duplicate column name") || strings.Contains(msg, "already exists") {
				return
			}
			log.Printf("schema update skipped: %v", err)
		}
	}

	create(`CREATE TABLE IF NOT EXISTS rooms (
		id TEXT PRIMARY KEY,
		title TEXT,
		description TEXT,
		teacher TEXT
	);`)
	create(`CREATE TABLE IF NOT EXISTS posts (
		id TEXT PRIMARY KEY,
		code TEXT,
		content TEXT,
		file_url TEXT
	);`)
	create(`CREATE TABLE IF NOT EXISTS announcements (
		id TEXT PRIMARY KEY,
		code TEXT,
		title TEXT,
		description TEXT
	);`)
	create(`CREATE TABLE IF NOT EXISTS files (
		id TEXT PRIMARY KEY,
		filename TEXT,
		size INTEGER,
		sha256 TEXT,
		path TEXT
	);`)
	bestEffort("ALTER TABLE files ADD COLUMN raw_size INTEGER DEFAULT 0;")
	bestEffort("ALTER TABLE files ADD COLUMN compressed_size INTEGER DEFAULT 0;")
	bestEffort("ALTER TABLE files ADD COLUMN compressed_path TEXT DEFAULT '';")
	bestEffort("ALTER TABLE files ADD COLUMN compression TEXT DEFAULT 'none';")
	bestEffort("ALTER TABLE files ADD COLUMN last_accessed TEXT DEFAULT '';")
	bestEffort("ALTER TABLE files ADD COLUMN is_cached INTEGER DEFAULT 1;")
	create(`CREATE TABLE IF NOT EXISTS join_requests (
		id TEXT PRIMARY KEY,
		room_id TEXT,
		student TEXT,
		status TEXT,
		created_at TEXT
	);`)
	create(`CREATE TABLE IF NOT EXISTS members (
		room_id TEXT,
		student TEXT,
		PRIMARY KEY (room_id, student)
	);`)
	create(`CREATE TABLE IF NOT EXISTS assignments (
		id TEXT PRIMARY KEY,
		code TEXT,
		title TEXT,
		description TEXT,
		created_by TEXT,
		created_at TEXT
	);`)
	create(`CREATE TABLE IF NOT EXISTS assignment_submissions (
		id TEXT PRIMARY KEY,
		assignment_id TEXT,
		code TEXT,
		student TEXT,
		answer TEXT,
		status TEXT,
		file_name TEXT,
		file_type TEXT,
		file_size INTEGER,
		file_sha256 TEXT,
		chunks TEXT,
		submitted_at TEXT
	);`)
	create(`CREATE TABLE IF NOT EXISTS file_download_sessions (
		file_id TEXT PRIMARY KEY,
		file_name TEXT,
		file_size INTEGER,
		file_sha256 TEXT,
		chunk_size INTEGER,
		total_chunks INTEGER,
		completed_chunks INTEGER,
		status TEXT,
		temp_path TEXT,
		last_error TEXT,
		updated_at TEXT
	);`)
	create(`CREATE TABLE IF NOT EXISTS file_download_chunks (
		file_id TEXT,
		chunk_index INTEGER,
		status TEXT,
		chunk_hash TEXT,
		source_peer TEXT,
		updated_at TEXT,
		PRIMARY KEY (file_id, chunk_index)
	);`)
	create(`CREATE TABLE IF NOT EXISTS chunk_owners (
		file_id TEXT,
		chunk_index INTEGER,
		owner_host TEXT,
		owner_port INTEGER,
		updated_at TEXT,
		PRIMARY KEY (file_id, chunk_index, owner_host, owner_port)
	);`)
	create(`CREATE TABLE IF NOT EXISTS storage_chunks (
		file_id TEXT,
		chunk_index INTEGER,
		offset_bytes INTEGER,
		chunk_size INTEGER,
		chunk_sha256 TEXT,
		last_accessed TEXT,
		is_present INTEGER DEFAULT 1,
		PRIMARY KEY (file_id, chunk_index)
	);`)
	create(`CREATE TABLE IF NOT EXISTS sms_transfers (
		transfer_id TEXT PRIMARY KEY,
		file_name TEXT,
		file_sha256 TEXT,
		total_parts INTEGER,
		received_parts INTEGER,
		encoding TEXT,
		compression TEXT,
		status TEXT,
		source TEXT,
		target TEXT,
		output_path TEXT,
		created_at TEXT,
		updated_at TEXT
	);`)
	create(`CREATE TABLE IF NOT EXISTS sms_parts (
		transfer_id TEXT,
		part_index INTEGER,
		total_parts INTEGER,
		payload TEXT,
		received_at TEXT,
		PRIMARY KEY (transfer_id, part_index)
	);`)

	return db
}

// --- Handlers ---

func handleJoinRoom(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != "POST" {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Teacher     string `json:"teacher"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}
	mu.Lock()
	defer mu.Unlock()
	_, err := db.Exec("INSERT OR IGNORE INTO rooms (id, title, description, teacher) VALUES (?, ?, ?, ?)",
		body.ID, body.Title, body.Description, body.Teacher)
	if err != nil {
		http.Error(w, "Failed to join room", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Joined room"))
}

func handleUploadPost(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != "POST" {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseMultipartForm(10 << 20) // 10 MB
	if err != nil {
		http.Error(w, fmt.Sprintf("Error parsing form: %v", err), http.StatusBadRequest)
		return
	}

	id := r.FormValue("id")
	var exists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM posts WHERE id = ?)", id).Scan(&exists)

	if exists {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Post uploaded successfully"))
		return
	}
	code := r.FormValue("code")
	content := r.FormValue("content")

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, fmt.Sprintf("Error getting file: %v", err), http.StatusBadRequest)
		return
	}
	defer file.Close()

	filename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), header.Filename)
	filepath := filepath.Join(uploadDir, filename)
	fileURL := "/uploads/" + filename

	dst, err := os.Create(filepath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error saving file: %v", err), http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(dst, hasher), file)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error writing file: %v", err), http.StatusInternalServerError)
		return
	}
	fileID := hex.EncodeToString(hasher.Sum(nil))
	fileURL = "/files/" + fileID + "/download"

	mu.Lock()
	defer mu.Unlock()
	_, _ = db.Exec("INSERT OR IGNORE INTO files (id, filename, size, sha256, path) VALUES (?, ?, ?, ?, ?)",
		fileID, header.Filename, written, fileID, filepath)
	_, err = db.Exec("INSERT OR IGNORE INTO posts (id, code, content, file_url) VALUES (?, ?, ?, ?)", id, code, content, fileURL)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error storing post in db: %v", err), http.StatusInternalServerError)
		return
	}
	totalLocalChunks := int((written + int64(chunkSize) - 1) / int64(chunkSize))
	localHost, localPort := localAdvertiseHostPort()
	for i := 0; i < totalLocalChunks; i++ {
		_ = upsertChunkOwner(db, fileID, i, localHost, localPort)
	}
	_ = indexFileChunks(db, fileID, filepath, written, chunkSize)
	go buildCompressedArtifact(db, fileID, filepath, written)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Post uploaded successfully"))
}

func handleUploadStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID         string `json:"id"`
		Code       string `json:"code"`
		Content    string `json:"content"`
		FileName   string `json:"fileName"`
		FileSize   int64  `json:"fileSize"`
		TotalChunk int    `json:"totalChunks"`
		FileSHA256 string `json:"fileSha256"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}
	if req.Code == "" || req.Content == "" || req.FileName == "" || req.FileSize <= 0 {
		http.Error(w, "Missing fields", http.StatusBadRequest)
		return
	}
	if req.ID == "" {
		req.ID = makeID()
	}
	if req.TotalChunk <= 0 {
		req.TotalChunk = int((req.FileSize + chunkSize - 1) / chunkSize)
	}
	uploadID := makeID()
	session := &UploadSession{
		ID:         uploadID,
		PostID:     req.ID,
		Code:       req.Code,
		Content:    req.Content,
		FileName:   sanitizeFileName(req.FileName),
		FileSize:   req.FileSize,
		TotalChunk: req.TotalChunk,
		FileSHA256: strings.ToLower(strings.TrimSpace(req.FileSHA256)),
		Received:   map[int]string{},
		CreatedAt:  time.Now(),
	}

	uploadSessionMu.Lock()
	uploadSessions[uploadID] = session
	uploadSessionMu.Unlock()

	_ = os.MkdirAll(filepath.Join(chunkTempDir, uploadID), os.ModePerm)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"uploadId":   uploadID,
		"chunkSize":  chunkSize,
		"totalChunk": req.TotalChunk,
	})
}

func handleUploadChunk(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		http.Error(w, "Invalid multipart payload", http.StatusBadRequest)
		return
	}
	uploadID := strings.TrimSpace(r.FormValue("uploadId"))
	indexStr := strings.TrimSpace(r.FormValue("index"))
	checksum := strings.ToLower(strings.TrimSpace(r.FormValue("checksum")))
	if uploadID == "" || indexStr == "" {
		http.Error(w, "Missing fields", http.StatusBadRequest)
		return
	}
	index, err := strconv.Atoi(indexStr)
	if err != nil || index < 0 {
		http.Error(w, "Invalid chunk index", http.StatusBadRequest)
		return
	}

	uploadSessionMu.Lock()
	session, ok := uploadSessions[uploadID]
	uploadSessionMu.Unlock()
	if !ok {
		http.Error(w, "Unknown upload session", http.StatusNotFound)
		return
	}
	if index >= session.TotalChunk {
		http.Error(w, "Chunk index out of range", http.StatusBadRequest)
		return
	}

	chunkFile, _, err := r.FormFile("chunk")
	if err != nil {
		http.Error(w, "Missing chunk file", http.StatusBadRequest)
		return
	}
	defer chunkFile.Close()

	data, err := io.ReadAll(chunkFile)
	if err != nil {
		http.Error(w, "Failed to read chunk", http.StatusInternalServerError)
		return
	}
	sum := sha256.Sum256(data)
	actual := hex.EncodeToString(sum[:])
	if checksum != "" && actual != checksum {
		http.Error(w, "Checksum mismatch", http.StatusBadRequest)
		return
	}

	chunkDir := filepath.Join(chunkTempDir, uploadID)
	if err := os.MkdirAll(chunkDir, os.ModePerm); err != nil {
		http.Error(w, "Failed to create chunk dir", http.StatusInternalServerError)
		return
	}
	chunkPath := filepath.Join(chunkDir, fmt.Sprintf("%06d.part", index))
	if err := os.WriteFile(chunkPath, data, 0o600); err != nil {
		http.Error(w, "Failed to store chunk", http.StatusInternalServerError)
		return
	}

	uploadSessionMu.Lock()
	session.Received[index] = actual
	uploadSessionMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"accepted": true,
		"index":    index,
		"checksum": actual,
	})
}

func handleUploadFinish(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != "POST" {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		UploadID string `json:"uploadId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}
	req.UploadID = strings.TrimSpace(req.UploadID)
	if req.UploadID == "" {
		http.Error(w, "Missing uploadId", http.StatusBadRequest)
		return
	}

	uploadSessionMu.Lock()
	session, ok := uploadSessions[req.UploadID]
	uploadSessionMu.Unlock()
	if !ok {
		http.Error(w, "Unknown upload session", http.StatusNotFound)
		return
	}

	for i := 0; i < session.TotalChunk; i++ {
		if _, ok := session.Received[i]; !ok {
			http.Error(w, "Missing chunks", http.StatusBadRequest)
			return
		}
	}

	finalName := fmt.Sprintf("%d_%s", time.Now().UnixNano(), session.FileName)
	finalPath := filepath.Join(uploadDir, finalName)
	out, err := os.Create(finalPath)
	if err != nil {
		http.Error(w, "Failed to create final file", http.StatusInternalServerError)
		return
	}

	hasher := sha256.New()
	written := int64(0)
	chunkDir := filepath.Join(chunkTempDir, req.UploadID)
	for i := 0; i < session.TotalChunk; i++ {
		chunkPath := filepath.Join(chunkDir, fmt.Sprintf("%06d.part", i))
		data, err := os.ReadFile(chunkPath)
		if err != nil {
			out.Close()
			_ = os.Remove(finalPath)
			http.Error(w, "Failed to read chunk during assembly", http.StatusInternalServerError)
			return
		}
		n, err := out.Write(data)
		if err != nil {
			out.Close()
			_ = os.Remove(finalPath)
			http.Error(w, "Failed to write final file", http.StatusInternalServerError)
			return
		}
		written += int64(n)
		_, _ = hasher.Write(data)
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(finalPath)
		http.Error(w, "Failed to close file", http.StatusInternalServerError)
		return
	}

	fileID := hex.EncodeToString(hasher.Sum(nil))
	if session.FileSHA256 != "" && session.FileSHA256 != fileID {
		_ = os.Remove(finalPath)
		http.Error(w, "File checksum mismatch", http.StatusBadRequest)
		return
	}

	fileURL := "/files/" + fileID + "/download"
	mu.Lock()
	_, _ = db.Exec("INSERT OR IGNORE INTO files (id, filename, size, sha256, path) VALUES (?, ?, ?, ?, ?)",
		fileID, session.FileName, written, fileID, finalPath)
	_, err = db.Exec("INSERT OR IGNORE INTO posts (id, code, content, file_url) VALUES (?, ?, ?, ?)",
		session.PostID, session.Code, session.Content, fileURL)
	mu.Unlock()
	if err != nil {
		http.Error(w, "Failed to store upload record", http.StatusInternalServerError)
		return
	}

	uploadSessionMu.Lock()
	delete(uploadSessions, req.UploadID)
	uploadSessionMu.Unlock()
	_ = os.RemoveAll(chunkDir)
	localHost, localPort := localAdvertiseHostPort()
	for i := 0; i < session.TotalChunk; i++ {
		_ = upsertChunkOwner(db, fileID, i, localHost, localPort)
	}
	_ = indexFileChunks(db, fileID, finalPath, written, chunkSize)
	go buildCompressedArtifact(db, fileID, finalPath, written)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Post uploaded successfully",
		"fileId":  fileID,
		"fileUrl": fileURL,
	})
}

func makeID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

func handleGetPosts(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	var req struct {
		Code string `json:"code"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	rows, err := db.Query("SELECT id, content, file_url FROM posts WHERE code = ?", req.Code)
	if err != nil {
		http.Error(w, "Failed to fetch posts", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	posts := make([]map[string]string, 0)
	for rows.Next() {
		var id, content, file string
		rows.Scan(&id, &content, &file)
		posts = append(posts, map[string]string{
			"id":      id,
			"content": content,
			"fileUrl": file,
		})
	}
	json.NewEncoder(w).Encode(posts)
}

func handleAnnounce(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	var a struct {
		ID          string `json:"id"`
		Code        string `json:"code"`
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	json.NewDecoder(r.Body).Decode(&a)
	mu.Lock()
	defer mu.Unlock()
	_, err := db.Exec("INSERT OR IGNORE INTO announcements (id, code, title, description) VALUES (?, ?, ?, ?)",
		a.ID, a.Code, a.Title, a.Description)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error storing an in db: %v", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte("Announcement created"))
}

func handleGetAnnouncements(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	var req struct {
		Code string `json:"code"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	rows, err := db.Query("SELECT id, title, description FROM announcements WHERE code = ?", req.Code)
	if err != nil {
		http.Error(w, "Failed to fetch announcements", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	result := make([]map[string]string, 0)
	for rows.Next() {
		var id, title, desc string
		rows.Scan(&id, &title, &desc)
		result = append(result, map[string]string{
			"id":          id,
			"title":       title,
			"description": desc,
		})
		print(id, title, desc)
	}
	json.NewEncoder(w).Encode(result)
}

func handleRequestJoin(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != "POST" {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID      string `json:"id"`
		RoomID  string `json:"roomId"`
		Student string `json:"student"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}
	if req.ID == "" || req.RoomID == "" || req.Student == "" {
		http.Error(w, "Missing fields", http.StatusBadRequest)
		return
	}
	mu.Lock()
	defer mu.Unlock()
	_, _ = db.Exec("INSERT OR IGNORE INTO join_requests (id, room_id, student, status, created_at) VALUES (?, ?, ?, ?, ?)",
		req.ID, req.RoomID, req.Student, "pending", time.Now().Format(time.RFC3339))
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Requested"))
}

func handleGetJoinRequests(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	roomID := r.URL.Query().Get("roomId")
	if roomID == "" {
		http.Error(w, "Missing roomId", http.StatusBadRequest)
		return
	}
	rows, err := db.Query("SELECT id, room_id, student, status, created_at FROM join_requests WHERE room_id = ? AND status = 'pending'", roomID)
	if err != nil {
		http.Error(w, "Failed to fetch requests", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	out := make([]JoinRequest, 0)
	for rows.Next() {
		var jr JoinRequest
		rows.Scan(&jr.ID, &jr.RoomID, &jr.Student, &jr.Status, &jr.CreatedAt)
		out = append(out, jr)
	}
	json.NewEncoder(w).Encode(out)
}

func handleApproveJoin(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != "POST" {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID      string `json:"id"`
		RoomID  string `json:"roomId"`
		Student string `json:"student"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if req.ID != "" {
		_, _ = db.Exec("UPDATE join_requests SET status = 'approved' WHERE id = ?", req.ID)
		row := db.QueryRow("SELECT room_id, student FROM join_requests WHERE id = ?", req.ID)
		var roomID, student string
		if err := row.Scan(&roomID, &student); err == nil {
			_, _ = db.Exec("INSERT OR IGNORE INTO members (room_id, student) VALUES (?, ?)", roomID, student)
		}
	} else if req.RoomID != "" && req.Student != "" {
		_, _ = db.Exec("INSERT OR IGNORE INTO members (room_id, student) VALUES (?, ?)", req.RoomID, req.Student)
		_, _ = db.Exec("UPDATE join_requests SET status = 'approved' WHERE room_id = ? AND student = ?", req.RoomID, req.Student)
	} else {
		http.Error(w, "Missing fields", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Approved"))
}

func handleMembership(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	roomID := r.URL.Query().Get("roomId")
	student := r.URL.Query().Get("student")
	if roomID == "" || student == "" {
		http.Error(w, "Missing fields", http.StatusBadRequest)
		return
	}
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM members WHERE room_id = ? AND student = ?)", roomID, student).Scan(&exists)
	if err != nil {
		http.Error(w, "Error checking membership", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"approved": exists})
}

func handleListMembers(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}
	roomID := strings.TrimSpace(r.URL.Query().Get("roomId"))
	if r.Method == http.MethodPost {
		var req struct {
			RoomID string `json:"roomId"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil && strings.TrimSpace(req.RoomID) != "" {
			roomID = strings.TrimSpace(req.RoomID)
		}
	}
	if roomID == "" {
		http.Error(w, "Missing roomId", http.StatusBadRequest)
		return
	}
	rows, err := db.Query("SELECT room_id, student FROM members WHERE room_id = ? ORDER BY student ASC", roomID)
	if err != nil {
		http.Error(w, "Failed to fetch members", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	list := make([]Member, 0)
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.RoomID, &m.Student); err != nil {
			continue
		}
		list = append(list, m)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"roomId":    roomID,
		"members":   list,
		"count":     len(list),
		"updatedAt": time.Now().Format(time.RFC3339),
	})
}

func handleRoomStats(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	roomID := r.URL.Query().Get("roomId")
	var classroomCount int
	if err := db.QueryRow("SELECT COUNT(1) FROM rooms").Scan(&classroomCount); err != nil {
		http.Error(w, "Failed to read classroom stats", http.StatusInternalServerError)
		return
	}

	memberCount := 0
	pendingCount := 0
	if roomID != "" {
		_ = db.QueryRow("SELECT COUNT(1) FROM members WHERE room_id = ?", roomID).Scan(&memberCount)
		_ = db.QueryRow("SELECT COUNT(1) FROM join_requests WHERE room_id = ? AND status = 'pending'", roomID).Scan(&pendingCount)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]int{
		"classroomCount": classroomCount,
		"memberCount":    memberCount,
		"pendingCount":   pendingCount,
	})
}

func handleDeleteClassroom(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		RoomID string `json:"roomId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}
	roomID := strings.TrimSpace(req.RoomID)
	if roomID == "" {
		http.Error(w, "Missing roomId", http.StatusBadRequest)
		return
	}

	mu.Lock()
	defer mu.Unlock()

	tx, err := db.Begin()
	if err != nil {
		http.Error(w, "Failed to start delete transaction", http.StatusInternalServerError)
		return
	}
	rollback := func() {
		_ = tx.Rollback()
	}

	if _, err := tx.Exec("DELETE FROM assignment_submissions WHERE code = ?", roomID); err != nil {
		rollback()
		http.Error(w, "Failed to delete submissions", http.StatusInternalServerError)
		return
	}
	if _, err := tx.Exec("DELETE FROM assignments WHERE code = ?", roomID); err != nil {
		rollback()
		http.Error(w, "Failed to delete assignments", http.StatusInternalServerError)
		return
	}
	if _, err := tx.Exec("DELETE FROM posts WHERE code = ?", roomID); err != nil {
		rollback()
		http.Error(w, "Failed to delete posts", http.StatusInternalServerError)
		return
	}
	if _, err := tx.Exec("DELETE FROM announcements WHERE code = ?", roomID); err != nil {
		rollback()
		http.Error(w, "Failed to delete announcements", http.StatusInternalServerError)
		return
	}
	if _, err := tx.Exec("DELETE FROM join_requests WHERE room_id = ?", roomID); err != nil {
		rollback()
		http.Error(w, "Failed to delete join requests", http.StatusInternalServerError)
		return
	}
	if _, err := tx.Exec("DELETE FROM members WHERE room_id = ?", roomID); err != nil {
		rollback()
		http.Error(w, "Failed to delete members", http.StatusInternalServerError)
		return
	}
	if _, err := tx.Exec("DELETE FROM rooms WHERE id = ?", roomID); err != nil {
		rollback()
		http.Error(w, "Failed to delete classroom", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, "Failed to commit classroom deletion", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"deleted": true,
		"roomId":  roomID,
	})
}

func handleCreateAssignment(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != "POST" {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}
	var req Assignment
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	req.Code = strings.TrimSpace(req.Code)
	req.Title = strings.TrimSpace(req.Title)
	if req.ID == "" {
		req.ID = makeID()
	}
	if req.Code == "" || req.Title == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.CreatedAt) == "" {
		req.CreatedAt = time.Now().Format(time.RFC3339)
	}
	mu.Lock()
	_, err := db.Exec("INSERT OR IGNORE INTO assignments (id, code, title, description, created_by, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		req.ID, req.Code, req.Title, req.Description, req.CreatedBy, req.CreatedAt)
	mu.Unlock()
	if err != nil {
		http.Error(w, "Failed to create assignment", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(req)
}

func handleListAssignments(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != "POST" {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}
	rows, err := db.Query("SELECT id, code, title, description, created_by, created_at FROM assignments WHERE code = ? ORDER BY created_at DESC", strings.TrimSpace(req.Code))
	if err != nil {
		http.Error(w, "Failed to fetch assignments", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	list := make([]Assignment, 0)
	for rows.Next() {
		var a Assignment
		rows.Scan(&a.ID, &a.Code, &a.Title, &a.Description, &a.CreatedBy, &a.CreatedAt)
		list = append(list, a)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

func handleSubmitAssignment(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != "POST" {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}
	var req AssignmentSubmission
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	req.AssignmentID = strings.TrimSpace(req.AssignmentID)
	req.Code = strings.TrimSpace(req.Code)
	req.Student = strings.TrimSpace(req.Student)
	if req.ID == "" {
		req.ID = makeID()
	}
	if req.AssignmentID == "" || req.Code == "" || req.Student == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Status) == "" {
		req.Status = "Submitted"
	}
	if strings.TrimSpace(req.SubmittedAt) == "" {
		req.SubmittedAt = time.Now().Format(time.RFC3339)
	}
	chunksRaw := "[]"
	if len(req.Chunks) > 0 {
		chunksRaw = string(req.Chunks)
	}
	mu.Lock()
	_, err := db.Exec(`INSERT OR IGNORE INTO assignment_submissions
		(id, assignment_id, code, student, answer, status, file_name, file_type, file_size, file_sha256, chunks, submitted_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		req.ID, req.AssignmentID, req.Code, req.Student, req.Answer, req.Status, req.FileName, req.FileType, req.FileSize, req.FileSHA256, chunksRaw, req.SubmittedAt)
	mu.Unlock()
	if err != nil {
		http.Error(w, "Failed to submit assignment", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "id": req.ID})
}

func handleListSubmissions(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != "POST" {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Code         string `json:"code"`
		AssignmentID string `json:"assignmentId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}
	var (
		rows *sql.Rows
		err  error
	)
	code := strings.TrimSpace(req.Code)
	assignmentID := strings.TrimSpace(req.AssignmentID)
	if assignmentID != "" {
		rows, err = db.Query(`SELECT id, assignment_id, code, student, answer, status, file_name, file_type, file_size, file_sha256, chunks, submitted_at
			FROM assignment_submissions WHERE code = ? AND assignment_id = ? ORDER BY submitted_at DESC`, code, assignmentID)
	} else {
		rows, err = db.Query(`SELECT id, assignment_id, code, student, answer, status, file_name, file_type, file_size, file_sha256, chunks, submitted_at
			FROM assignment_submissions WHERE code = ? ORDER BY submitted_at DESC`, code)
	}
	if err != nil {
		http.Error(w, "Failed to fetch submissions", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	list := make([]AssignmentSubmission, 0)
	for rows.Next() {
		var s AssignmentSubmission
		var chunks string
		rows.Scan(&s.ID, &s.AssignmentID, &s.Code, &s.Student, &s.Answer, &s.Status, &s.FileName, &s.FileType, &s.FileSize, &s.FileSHA256, &chunks, &s.SubmittedAt)
		if strings.TrimSpace(chunks) == "" {
			chunks = "[]"
		}
		s.Chunks = json.RawMessage(chunks)
		list = append(list, s)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

func handleDownloadStatus(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	fileID := strings.TrimSpace(r.URL.Query().Get("fileId"))
	if fileID == "" {
		http.Error(w, "Missing fileId", http.StatusBadRequest)
		return
	}
	var sess struct {
		FileID          string `json:"fileId"`
		FileName        string `json:"fileName"`
		FileSize        int64  `json:"fileSize"`
		FileSHA256      string `json:"fileSha256"`
		ChunkSize       int64  `json:"chunkSize"`
		TotalChunks     int    `json:"totalChunks"`
		CompletedChunks int    `json:"completedChunks"`
		Status          string `json:"status"`
		LastError       string `json:"lastError"`
		UpdatedAt       string `json:"updatedAt"`
	}
	err := db.QueryRow(`SELECT file_id, file_name, file_size, file_sha256, chunk_size, total_chunks, completed_chunks, status, last_error, updated_at
		FROM file_download_sessions WHERE file_id = ?`, fileID).
		Scan(&sess.FileID, &sess.FileName, &sess.FileSize, &sess.FileSHA256, &sess.ChunkSize, &sess.TotalChunks, &sess.CompletedChunks, &sess.Status, &sess.LastError, &sess.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Download state not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to query status", http.StatusInternalServerError)
		return
	}

	var inProgress, incomplete, notDownloaded int
	_ = db.QueryRow("SELECT COUNT(1) FROM file_download_chunks WHERE file_id = ? AND status = 'in_progress'", fileID).Scan(&inProgress)
	_ = db.QueryRow("SELECT COUNT(1) FROM file_download_chunks WHERE file_id = ? AND status = 'incomplete'", fileID).Scan(&incomplete)
	_ = db.QueryRow("SELECT COUNT(1) FROM file_download_chunks WHERE file_id = ? AND status = 'not_downloaded'", fileID).Scan(&notDownloaded)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"session":       sess,
		"inProgress":    inProgress,
		"incomplete":    incomplete,
		"notDownloaded": notDownloaded,
	})
}

func handleChunkAnnounce(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		FileID     string `json:"fileId"`
		ChunkIndex int    `json:"chunkIndex"`
		Host       string `json:"host"`
		Port       int    `json:"port"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}
	req.FileID = strings.TrimSpace(req.FileID)
	req.Host = strings.TrimSpace(req.Host)
	if req.FileID == "" || req.ChunkIndex < 0 {
		http.Error(w, "Missing fileId or chunkIndex", http.StatusBadRequest)
		return
	}
	if req.Host == "" {
		req.Host = getLocalIP()
	}
	if req.Port == 0 {
		req.Port = 8080
	}
	if err := upsertChunkOwner(db, req.FileID, req.ChunkIndex, req.Host, req.Port); err != nil {
		http.Error(w, "Failed to store chunk owner", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

func handleChunkOwners(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}
	fileID := strings.TrimSpace(r.URL.Query().Get("fileId"))
	chunkIndexStr := strings.TrimSpace(r.URL.Query().Get("chunkIndex"))
	if r.Method == http.MethodPost {
		var req struct {
			FileID     string `json:"fileId"`
			ChunkIndex int    `json:"chunkIndex"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			if strings.TrimSpace(req.FileID) != "" {
				fileID = strings.TrimSpace(req.FileID)
			}
			if req.ChunkIndex >= 0 {
				chunkIndexStr = strconv.Itoa(req.ChunkIndex)
			}
		}
	}
	if fileID == "" || chunkIndexStr == "" {
		http.Error(w, "Missing fileId or chunkIndex", http.StatusBadRequest)
		return
	}
	chunkIndex, err := strconv.Atoi(chunkIndexStr)
	if err != nil || chunkIndex < 0 {
		http.Error(w, "Invalid chunkIndex", http.StatusBadRequest)
		return
	}
	owners, err := listChunkOwners(db, fileID, chunkIndex)
	if err != nil {
		http.Error(w, "Failed to list chunk owners", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"fileId":     fileID,
		"chunkIndex": chunkIndex,
		"owners":     owners,
	})
}

func handleSMSPrepare(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}
	var req SMSPrepareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	var (
		raw      []byte
		fileName string
	)
	if strings.TrimSpace(req.FileID) != "" {
		meta, err := getFileMeta(db, strings.TrimSpace(req.FileID))
		if err != nil {
			http.Error(w, "Unknown fileId", http.StatusNotFound)
			return
		}
		if err := ensureFilePresent(db, meta); err != nil {
			http.Error(w, "File not available for SMS preparation", http.StatusServiceUnavailable)
			return
		}
		data, err := os.ReadFile(meta.Path)
		if err != nil {
			http.Error(w, "Failed to read file", http.StatusInternalServerError)
			return
		}
		raw = data
		fileName = meta.FileName
	} else {
		text := strings.TrimSpace(req.Text)
		if text == "" {
			http.Error(w, "Provide fileId or text", http.StatusBadRequest)
			return
		}
		raw = []byte(text)
		fileName = strings.TrimSpace(req.FileName)
		if fileName == "" {
			fileName = "sms-resource.txt"
		}
	}
	if strings.TrimSpace(req.FileName) != "" {
		fileName = sanitizeFileName(req.FileName)
	}

	compressed, compression := raw, "none"
	if req.Compress || strings.TrimSpace(req.FileID) != "" {
		var err error
		compressed, err = compressSMSPayload(raw)
		if err == nil {
			compression = "zstd"
		}
	}
	encoding := "base85"
	if req.UseBase64 {
		encoding = "base64"
	}
	sum := sha256.Sum256(raw)
	fileSHA := hex.EncodeToString(sum[:])
	transferID := makeSMSTransferID()
	messages, err := buildSMSMessages(transferID, fileName, fileSHA, compression, encoding, compressed)
	if err != nil {
		http.Error(w, "Failed to build SMS chunks", http.StatusInternalServerError)
		return
	}

	now := time.Now().Format(time.RFC3339)
	_, _ = db.Exec(`INSERT INTO sms_transfers
		(transfer_id, file_name, file_sha256, total_parts, received_parts, encoding, compression, status, source, target, output_path, created_at, updated_at)
		VALUES (?, ?, ?, ?, 0, ?, ?, 'prepared', ?, ?, '', ?, ?)
		ON CONFLICT(transfer_id) DO UPDATE SET
			file_name=excluded.file_name,
			file_sha256=excluded.file_sha256,
			total_parts=excluded.total_parts,
			encoding=excluded.encoding,
			compression=excluded.compression,
			status=excluded.status,
			source=excluded.source,
			target=excluded.target,
			updated_at=excluded.updated_at`,
		transferID, fileName, fileSHA, len(messages)-1, encoding, compression, nodeID, strings.TrimSpace(req.Target), now, now)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(SMSPrepareResponse{
		TransferID:  transferID,
		FileName:    fileName,
		Parts:       len(messages) - 1,
		Encoding:    encoding,
		Compression: compression,
		Messages:    messages,
	})
}

func handleSMSIngest(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		From    string `json:"from"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}
	message := strings.TrimSpace(req.Message)
	if !strings.HasPrefix(message, smsPrefix) {
		http.Error(w, "Not a SchoolSync SMS", http.StatusBadRequest)
		return
	}
	body := strings.TrimPrefix(message, smsPrefix)
	parts := strings.SplitN(body, "|", 2)
	if len(parts) < 2 {
		http.Error(w, "Invalid SMS packet", http.StatusBadRequest)
		return
	}

	now := time.Now().Format(time.RFC3339)
	switch strings.TrimSpace(parts[0]) {
	case "M":
		metaParts := strings.Split(body, "|")
		parts = metaParts
		if len(parts) < 8 {
			http.Error(w, "Invalid SMS meta packet", http.StatusBadRequest)
			return
		}
		transferID := strings.TrimSpace(parts[1])
		total, err := strconv.Atoi(strings.TrimSpace(parts[6]))
		if err != nil || total <= 0 {
			http.Error(w, "Invalid total parts", http.StatusBadRequest)
			return
		}
		fileNameRaw, err := decodeBase64String(parts[2])
		if err != nil {
			http.Error(w, "Invalid fileName encoding", http.StatusBadRequest)
			return
		}
		_, _ = db.Exec(`INSERT INTO sms_transfers
			(transfer_id, file_name, file_sha256, total_parts, received_parts, encoding, compression, status, source, target, output_path, created_at, updated_at)
			VALUES (?, ?, ?, ?, 0, ?, ?, 'receiving', ?, ?, '', ?, ?)
			ON CONFLICT(transfer_id) DO UPDATE SET
				file_name=excluded.file_name,
				file_sha256=excluded.file_sha256,
				total_parts=excluded.total_parts,
				encoding=excluded.encoding,
				compression=excluded.compression,
				status='receiving',
				source=excluded.source,
				updated_at=excluded.updated_at`,
			transferID, sanitizeFileName(fileNameRaw), strings.TrimSpace(parts[3]), total, strings.TrimSpace(parts[5]), strings.TrimSpace(parts[4]), strings.TrimSpace(req.From), nodeID, now, now)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "transferId": transferID, "packet": "meta"})
		return
	case "D":
		dataParts := strings.SplitN(body, "|", 6)
		parts = dataParts
		if len(parts) < 6 {
			http.Error(w, "Invalid SMS data packet", http.StatusBadRequest)
			return
		}
		transferID := strings.TrimSpace(parts[1])
		partIndex, err := strconv.Atoi(strings.TrimSpace(parts[2]))
		if err != nil || partIndex <= 0 {
			http.Error(w, "Invalid part index", http.StatusBadRequest)
			return
		}
		total, err := strconv.Atoi(strings.TrimSpace(parts[3]))
		if err != nil || total <= 0 {
			http.Error(w, "Invalid total", http.StatusBadRequest)
			return
		}
		payload := strings.TrimSpace(parts[5])
		_, _ = db.Exec(`INSERT INTO sms_parts (transfer_id, part_index, total_parts, payload, received_at)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(transfer_id, part_index) DO UPDATE SET
				payload=excluded.payload,
				received_at=excluded.received_at`,
			transferID, partIndex, total, payload, now)
		_, _ = db.Exec(`INSERT OR IGNORE INTO sms_transfers
			(transfer_id, file_name, file_sha256, total_parts, received_parts, encoding, compression, status, source, target, output_path, created_at, updated_at)
			VALUES (?, '', '', ?, 0, ?, 'unknown', 'receiving', ?, ?, '', ?, ?)`,
			transferID, total, strings.TrimSpace(parts[4]), strings.TrimSpace(req.From), nodeID, now, now)

		var received int
		_ = db.QueryRow("SELECT COUNT(1) FROM sms_parts WHERE transfer_id = ?", transferID).Scan(&received)
		status := "receiving"
		if received >= total {
			status = "ready"
		}
		_, _ = db.Exec(`UPDATE sms_transfers
			SET total_parts = CASE WHEN total_parts <= 0 THEN ? ELSE total_parts END,
				received_parts = ?,
				status = ?,
				updated_at = ?
			WHERE transfer_id = ?`, total, received, status, now, transferID)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":            true,
			"transferId":    transferID,
			"partIndex":     partIndex,
			"receivedParts": received,
			"totalParts":    total,
			"status":        status,
		})
		return
	default:
		http.Error(w, "Unknown SMS packet type", http.StatusBadRequest)
		return
	}
}

func handleSMSStatus(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}
	transferID := strings.TrimSpace(r.URL.Query().Get("transferId"))
	if transferID == "" {
		http.Error(w, "Missing transferId", http.StatusBadRequest)
		return
	}
	var row struct {
		TransferID    string `json:"transferId"`
		FileName      string `json:"fileName"`
		FileSHA256    string `json:"fileSha256"`
		TotalParts    int    `json:"totalParts"`
		ReceivedParts int    `json:"receivedParts"`
		Encoding      string `json:"encoding"`
		Compression   string `json:"compression"`
		Status        string `json:"status"`
		OutputPath    string `json:"outputPath"`
		UpdatedAt     string `json:"updatedAt"`
	}
	err := db.QueryRow(`SELECT transfer_id, file_name, file_sha256, total_parts, received_parts, encoding, compression, status, output_path, updated_at
		FROM sms_transfers WHERE transfer_id = ?`, transferID).
		Scan(&row.TransferID, &row.FileName, &row.FileSHA256, &row.TotalParts, &row.ReceivedParts, &row.Encoding, &row.Compression, &row.Status, &row.OutputPath, &row.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Transfer not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to query transfer", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(row)
}

func handleSMSReassemble(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		TransferID string `json:"transferId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}
	transferID := strings.TrimSpace(req.TransferID)
	if transferID == "" {
		http.Error(w, "Missing transferId", http.StatusBadRequest)
		return
	}

	var fileName, fileSHA, encoding, compression, status string
	var totalParts int
	err := db.QueryRow(`SELECT file_name, file_sha256, total_parts, encoding, compression, status
		FROM sms_transfers WHERE transfer_id = ?`, transferID).
		Scan(&fileName, &fileSHA, &totalParts, &encoding, &compression, &status)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Transfer not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to query transfer", http.StatusInternalServerError)
		return
	}
	if totalParts <= 0 {
		http.Error(w, "Transfer metadata incomplete", http.StatusBadRequest)
		return
	}

	rows, err := db.Query(`SELECT part_index, payload FROM sms_parts
		WHERE transfer_id = ?
		ORDER BY part_index ASC`, transferID)
	if err != nil {
		http.Error(w, "Failed to query parts", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	chunks := make([][]byte, 0, totalParts)
	for rows.Next() {
		var partIndex int
		var payload string
		if err := rows.Scan(&partIndex, &payload); err != nil {
			continue
		}
		raw, err := decodeSMSChunkPayload(payload, encoding)
		if err != nil {
			http.Error(w, "Failed to decode part payload", http.StatusBadRequest)
			return
		}
		chunks = append(chunks, raw)
	}
	if len(chunks) != totalParts {
		http.Error(w, "Transfer is incomplete", http.StatusConflict)
		return
	}

	joined := bytes.Join(chunks, nil)
	finalBytes := joined
	if strings.EqualFold(strings.TrimSpace(compression), "zstd") {
		decoded, err := decompressSMSPayload(joined)
		if err != nil {
			http.Error(w, "Failed to decompress transfer", http.StatusBadRequest)
			return
		}
		finalBytes = decoded
	}
	sum := sha256.Sum256(finalBytes)
	actualSHA := hex.EncodeToString(sum[:])
	if strings.TrimSpace(fileSHA) != "" && !strings.EqualFold(fileSHA, actualSHA) {
		http.Error(w, "Integrity check failed", http.StatusBadRequest)
		return
	}

	if fileName == "" {
		fileName = "sms-resource.bin"
	}
	outName := fmt.Sprintf("sms_%s_%s", transferID, sanitizeFileName(fileName))
	outPath := filepath.Join(uploadDir, outName)
	if err := os.WriteFile(outPath, finalBytes, 0o600); err != nil {
		http.Error(w, "Failed to write reconstructed file", http.StatusInternalServerError)
		return
	}

	now := time.Now().Format(time.RFC3339)
	_, _ = db.Exec(`UPDATE sms_transfers
		SET received_parts = ?, status = 'completed', output_path = ?, updated_at = ?
		WHERE transfer_id = ?`, totalParts, outPath, now, transferID)
	_, _ = db.Exec(`INSERT OR IGNORE INTO files (id, filename, size, sha256, path)
		VALUES (?, ?, ?, ?, ?)`, actualSHA, fileName, len(finalBytes), actualSHA, outPath)
	_ = indexFileChunks(db, actualSHA, outPath, int64(len(finalBytes)), chunkSize)
	go buildCompressedArtifact(db, actualSHA, outPath, int64(len(finalBytes)))

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"message":      "Resource reassembled",
		"transferId":   transferID,
		"fileId":       actualSHA,
		"fileName":     fileName,
		"size":         len(finalBytes),
		"outputPath":   outPath,
		"statusBefore": status,
	})
}

func makeSMSTransferID() string {
	raw := makeID()
	if len(raw) <= 12 {
		return raw
	}
	return raw[:12]
}

func buildSMSMessages(transferID, fileName, fileSHA, compression, encoding string, payload []byte) ([]string, error) {
	if strings.TrimSpace(transferID) == "" || len(payload) == 0 {
		return nil, fmt.Errorf("invalid sms build input")
	}
	if encoding != "base85" && encoding != "base64" {
		return nil, fmt.Errorf("unsupported encoding")
	}
	if len(fileName) > 32 {
		fileName = fileName[:32]
	}
	total := int((len(payload) + smsRawChunkSize - 1) / smsRawChunkSize)
	if total <= 0 {
		total = 1
	}

	nameB64 := base64.StdEncoding.EncodeToString([]byte(fileName))
	meta := fmt.Sprintf("%sM|%s|%s|%s|%s|%s|%d",
		smsPrefix, transferID, nameB64, fileSHA, compression, encoding, total)
	if len(meta) > 160 {
		return nil, fmt.Errorf("sms meta exceeds 160 chars")
	}
	out := []string{meta}

	for i := 0; i < total; i++ {
		start := i * smsRawChunkSize
		end := start + smsRawChunkSize
		if end > len(payload) {
			end = len(payload)
		}
		part := payload[start:end]
		encoded := encodeSMSChunkPayload(part, encoding)
		if len(encoded) > smsPartTextLimit {
			return nil, fmt.Errorf("encoded SMS chunk too large: part=%d chars=%d", i+1, len(encoded))
		}
		msg := fmt.Sprintf("%sD|%s|%d|%d|%s|%s", smsPrefix, transferID, i+1, total, encoding, encoded)
		if len(msg) > 160 {
			return nil, fmt.Errorf("sms exceeds 160 chars at part=%d", i+1)
		}
		out = append(out, msg)
	}
	return out, nil
}

func encodeSMSChunkPayload(raw []byte, encoding string) string {
	if encoding == "base64" {
		return base64.StdEncoding.EncodeToString(raw)
	}
	dst := make([]byte, ascii85.MaxEncodedLen(len(raw)))
	n := ascii85.Encode(dst, raw)
	return string(dst[:n])
}

func decodeSMSChunkPayload(encoded, encoding string) ([]byte, error) {
	if encoding == "base64" {
		return base64.StdEncoding.DecodeString(encoded)
	}
	dst := make([]byte, len(encoded))
	n, _, err := ascii85.Decode(dst, []byte(encoded), true)
	if err != nil {
		return nil, err
	}
	return dst[:n], nil
}

func decodeBase64String(raw string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func compressSMSPayload(raw []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw, err := zstd.NewWriter(&buf, zstd.WithEncoderLevel(zstd.SpeedBetterCompression))
	if err != nil {
		return nil, err
	}
	if _, err := zw.Write(raw); err != nil {
		_ = zw.Close()
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decompressSMSPayload(raw []byte) ([]byte, error) {
	zr, err := zstd.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	return io.ReadAll(zr)
}

func upsertChunkOwner(db *sql.DB, fileID string, chunkIndex int, host string, port int) error {
	fileID = strings.TrimSpace(fileID)
	host = strings.TrimSpace(host)
	if fileID == "" || chunkIndex < 0 || host == "" {
		return fmt.Errorf("invalid chunk owner input")
	}
	if port == 0 {
		port = 8080
	}
	_, err := db.Exec(`INSERT INTO chunk_owners (file_id, chunk_index, owner_host, owner_port, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(file_id, chunk_index, owner_host, owner_port) DO UPDATE SET
		updated_at=excluded.updated_at`,
		fileID, chunkIndex, host, port, time.Now().Format(time.RFC3339))
	return err
}

func listChunkOwners(db *sql.DB, fileID string, chunkIndex int) ([]ChunkOwner, error) {
	rows, err := db.Query(`SELECT file_id, chunk_index, owner_host, owner_port, updated_at
		FROM chunk_owners
		WHERE file_id = ? AND chunk_index = ?
		ORDER BY updated_at DESC`, fileID, chunkIndex)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	owners := make([]ChunkOwner, 0)
	for rows.Next() {
		var o ChunkOwner
		if err := rows.Scan(&o.FileID, &o.ChunkIndex, &o.Host, &o.Port, &o.UpdatedAt); err != nil {
			continue
		}
		owners = append(owners, o)
	}
	return owners, nil
}

func localAdvertiseHostPort() (string, int) {
	host := strings.TrimSpace(advertiseHost)
	if host == "" {
		host = getLocalIP()
	}
	return host, 8080
}

func hostPortFromBase(base string) (string, int, bool) {
	u, err := url.Parse(strings.TrimSpace(base))
	if err != nil || u == nil {
		return "", 0, false
	}
	host := strings.TrimSpace(u.Hostname())
	if host == "" {
		return "", 0, false
	}
	port := 8080
	if p := strings.TrimSpace(u.Port()); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			port = v
		}
	}
	return host, port, true
}

func prioritizeChunkCandidates(db *sql.DB, fileID string, chunkIndex int, fallback []string) []string {
	owners, err := listChunkOwners(db, fileID, chunkIndex)
	if err != nil || len(owners) == 0 {
		return fallback
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(owners)+len(fallback))
	add := func(base string) {
		base = strings.TrimSuffix(strings.TrimSpace(base), "/")
		if base == "" {
			return
		}
		if _, ok := seen[base]; ok {
			return
		}
		seen[base] = struct{}{}
		out = append(out, base)
	}
	for _, o := range owners {
		if strings.TrimSpace(o.Host) == "" {
			continue
		}
		port := o.Port
		if port == 0 {
			port = 8080
		}
		add(fmt.Sprintf("http://%s:%d", o.Host, port))
	}
	for _, base := range fallback {
		add(base)
	}
	return out
}

func handleGetJoinedRooms(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	rows, err := db.Query("SELECT id, title, description, teacher FROM rooms")
	if err != nil {
		http.Error(w, "Error fetching rooms"+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	rooms := make([]map[string]string, 0)
	for rows.Next() {
		var id, title, desc, teacher string
		rows.Scan(&id, &title, &desc, &teacher)
		rooms = append(rooms, map[string]string{
			"id":          id,
			"title":       title,
			"description": desc,
			"teacher":     teacher,
		})
	}
	json.NewEncoder(w).Encode(rooms)
}

// --- UDP Sharing ---

func broadcastPresence() {
	payload, _ := json.Marshal(map[string]interface{}{
		"nodeId": nodeID,
		"port":   strings.TrimPrefix(httpPort, ":"),
	})

	broadcastAddr := &net.UDPAddr{IP: net.IPv4bcast, Port: udpPort}
	conn, err := net.DialUDP("udp", nil, broadcastAddr)
	if err != nil {
		log.Println("Broadcast error:", err)
		return
	}
	defer conn.Close()
	conn.Write(payload)
}

func startUDPListener(db *sql.DB) {
	addr := &net.UDPAddr{IP: net.IPv4zero, Port: udpPort}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	buf := make([]byte, 2048)
	for {
		n, remoteAddr, _ := conn.ReadFromUDP(buf)
		go handleUDPMessage(buf[:n], remoteAddr, db)
	}
}

func handleUDPMessage(data []byte, remote *net.UDPAddr, db *sql.DB) {
	var msg struct {
		NodeID string `json:"nodeId"`
		Port   string `json:"port"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return
	}
	if msg.NodeID == "" || msg.NodeID == nodeID {
		return
	}
	port := 0
	if msg.Port != "" {
		fmt.Sscanf(msg.Port, "%d", &port)
	}
	if port == 0 {
		port = 8080
	}

	upsertPeer(remote.IP.String(), "", msg.NodeID, port)
	go syncFromPeer(db, remote.IP.String(), port)
}

func upsertPeer(ip, host, id string, port int) {
	peerMu.Lock()
	defer peerMu.Unlock()
	p, ok := peers[ip]
	if !ok {
		p = &Peer{}
		peers[ip] = p
	}
	p.ID = id
	p.IP = ip
	if host != "" {
		p.Host = host
	}
	p.Port = port
	p.LastSeen = time.Now()
}

func syncAllPeers(db *sql.DB) {
	peerMu.Lock()
	list := make([]*Peer, 0, len(peers))
	for _, p := range peers {
		list = append(list, p)
	}
	peerMu.Unlock()

	for _, p := range list {
		host := p.IP
		if p.Host != "" {
			host = p.Host
		}
		syncFromPeer(db, host, p.Port)
	}

	if discoveryURL != "" {
		syncFromDiscovery(db)
	}
}

func syncFromPeer(db *sql.DB, ip string, port int) {
	base := fmt.Sprintf("http://%s:%d", ip, port)
	payload, err := fetchExportPayload(base)
	if err != nil {
		return
	}
	if payload.NodeID == nodeID {
		return
	}

	syncFilesFromPeer(db, base, payload.Files)

	mu.Lock()
	for _, r := range payload.Rooms {
		db.Exec("INSERT OR IGNORE INTO rooms (id, title, description, teacher) VALUES (?, ?, ?, ?)",
			r.ID, r.Title, r.Description, r.Teacher)
	}
	mu.Unlock()

	for _, p := range payload.Posts {
		var exists bool
		_ = db.QueryRow("SELECT EXISTS(SELECT 1 FROM posts WHERE id = ?)", p.ID).Scan(&exists)
		if exists {
			continue
		}
		fileURL := localFileURLFromRemote(p.FileURL)
		mu.Lock()
		db.Exec("INSERT OR IGNORE INTO posts (id, code, content, file_url) VALUES (?, ?, ?, ?)",
			p.ID, p.Code, p.Content, fileURL)
		mu.Unlock()
	}

	mu.Lock()
	for _, a := range payload.Announcements {
		db.Exec("INSERT OR IGNORE INTO announcements (id, code, title, description) VALUES (?, ?, ?, ?)",
			a.ID, a.Code, a.Title, a.Description)
	}
	mu.Unlock()

	mu.Lock()
	for _, jr := range payload.JoinRequests {
		db.Exec("INSERT OR IGNORE INTO join_requests (id, room_id, student, status, created_at) VALUES (?, ?, ?, ?, ?)",
			jr.ID, jr.RoomID, jr.Student, jr.Status, jr.CreatedAt)
		if jr.Status == "approved" {
			db.Exec("INSERT OR IGNORE INTO members (room_id, student) VALUES (?, ?)", jr.RoomID, jr.Student)
		}
	}
	for _, m := range payload.Members {
		db.Exec("INSERT OR IGNORE INTO members (room_id, student) VALUES (?, ?)", m.RoomID, m.Student)
	}
	for _, a := range payload.Assignments {
		db.Exec("INSERT OR IGNORE INTO assignments (id, code, title, description, created_by, created_at) VALUES (?, ?, ?, ?, ?, ?)",
			a.ID, a.Code, a.Title, a.Description, a.CreatedBy, a.CreatedAt)
	}
	for _, s := range payload.Submissions {
		chunks := "[]"
		if len(s.Chunks) > 0 {
			chunks = string(s.Chunks)
		}
		db.Exec(`INSERT OR IGNORE INTO assignment_submissions
			(id, assignment_id, code, student, answer, status, file_name, file_type, file_size, file_sha256, chunks, submitted_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			s.ID, s.AssignmentID, s.Code, s.Student, s.Answer, s.Status, s.FileName, s.FileType, s.FileSize, s.FileSHA256, chunks, s.SubmittedAt)
	}
	mu.Unlock()
}

func fetchExportPayload(base string) (ExportPayload, error) {
	if len(sharedKey) > 0 {
		resp, err := http.Get(base + "/export_enc")
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			var enc struct {
				Nonce string `json:"nonce"`
				Data  string `json:"data"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&enc); err == nil {
				raw, err := decryptJSON(enc.Nonce, enc.Data)
				if err == nil {
					var payload ExportPayload
					if err := json.Unmarshal(raw, &payload); err == nil {
						return payload, nil
					}
				}
			}
		}
	}

	resp, err := http.Get(base + "/export")
	if err != nil {
		return ExportPayload{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ExportPayload{}, fmt.Errorf("export failed")
	}
	var payload ExportPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return ExportPayload{}, err
	}
	return payload, nil
}

func syncFilesFromPeer(db *sql.DB, base string, files []FileMeta) {
	for _, f := range files {
		var exists bool
		_ = db.QueryRow("SELECT EXISTS(SELECT 1 FROM files WHERE id = ?)", f.ID).Scan(&exists)
		if exists {
			continue
		}
		localPath, err := downloadFileChunks(db, base, f)
		if err != nil {
			continue
		}
		mu.Lock()
		db.Exec("INSERT OR IGNORE INTO files (id, filename, size, sha256, path) VALUES (?, ?, ?, ?, ?)",
			f.ID, f.FileName, f.Size, f.SHA256, localPath)
		mu.Unlock()
		_ = indexFileChunks(db, f.ID, localPath, f.Size, chunkSize)
	}
}

func localFileURLFromRemote(fileURL string) string {
	if fileURL == "" {
		return ""
	}
	if strings.HasPrefix(fileURL, "/files/") {
		parts := strings.Split(strings.TrimPrefix(fileURL, "/files/"), "/")
		if len(parts) > 0 && parts[0] != "" {
			return "/files/" + parts[0] + "/download"
		}
	}
	if strings.HasPrefix(fileURL, "/uploads/") {
		return fileURL
	}
	return fileURL
}

func downloadFileChunks(db *sql.DB, base string, meta FileMeta) (string, error) {
	candidates := peerBases(base)
	man, err := fetchManifestFromPeers(candidates, meta.ID)
	if err != nil {
		return "", err
	}

	total := int((man.Size + man.ChunkSize - 1) / man.ChunkSize)
	chunkTmpPath := filepath.Join(chunkTempDir, fmt.Sprintf("%s.resume", meta.ID))
	if err := ensureDownloadSession(db, meta.ID, man, chunkTmpPath, total); err != nil {
		return "", err
	}

	out, err := os.OpenFile(chunkTmpPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return "", err
	}
	defer out.Close()
	if err := out.Truncate(man.Size); err != nil {
		return "", err
	}

	if err := revalidateCompletedChunks(db, out, meta.ID, man); err != nil {
		_ = setDownloadSessionStatus(db, meta.ID, "failed", err.Error())
		return "", err
	}
	if err := setDownloadSessionStatus(db, meta.ID, "in_progress", ""); err != nil {
		return "", err
	}

	for i := 0; i < total; i++ {
		completed, err := isChunkCompleted(db, meta.ID, i)
		if err != nil {
			_ = setDownloadSessionStatus(db, meta.ID, "failed", err.Error())
			return "", err
		}
		if completed {
			continue
		}

		_ = upsertChunkStatus(db, meta.ID, i, "in_progress", "", "")
		chunkCandidates := prioritizeChunkCandidates(db, meta.ID, i, candidates)
		chunk, source, chunkHash, err := fetchChunkFromPeers(chunkCandidates, meta.ID, i)
		if err != nil {
			_ = upsertChunkStatus(db, meta.ID, i, "incomplete", "", "")
			_ = setDownloadSessionStatus(db, meta.ID, "in_progress", err.Error())
			return "", err
		}
		offset := int64(i) * man.ChunkSize
		if _, err := out.WriteAt(chunk, offset); err != nil {
			_ = upsertChunkStatus(db, meta.ID, i, "incomplete", "", source)
			_ = setDownloadSessionStatus(db, meta.ID, "failed", err.Error())
			return "", err
		}
		if err := upsertChunkStatus(db, meta.ID, i, "completed", chunkHash, source); err != nil {
			_ = setDownloadSessionStatus(db, meta.ID, "failed", err.Error())
			return "", err
		}
		if h, p, ok := hostPortFromBase(source); ok {
			_ = upsertChunkOwner(db, meta.ID, i, h, p)
		}
		if h, p := localAdvertiseHostPort(); h != "" {
			_ = upsertChunkOwner(db, meta.ID, i, h, p)
		}
		if err := updateCompletedChunks(db, meta.ID); err != nil {
			_ = setDownloadSessionStatus(db, meta.ID, "failed", err.Error())
			return "", err
		}
	}

	if _, err := out.Seek(0, io.SeekStart); err != nil {
		_ = setDownloadSessionStatus(db, meta.ID, "failed", err.Error())
		return "", err
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, out); err != nil {
		_ = setDownloadSessionStatus(db, meta.ID, "failed", err.Error())
		return "", err
	}
	sum := hex.EncodeToString(hasher.Sum(nil))
	if man.SHA256 != "" && sum != man.SHA256 {
		_ = setDownloadSessionStatus(db, meta.ID, "failed", "final file hash mismatch")
		return "", fmt.Errorf("hash mismatch")
	}
	finalName := meta.ID + "_" + sanitizeFileName(meta.FileName)
	finalPath := filepath.Join(uploadDir, finalName)
	if err := os.Rename(chunkTmpPath, finalPath); err != nil {
		_ = setDownloadSessionStatus(db, meta.ID, "failed", err.Error())
		return "", err
	}
	_ = setDownloadSessionStatus(db, meta.ID, "completed", "")
	return finalPath, nil
}

type fileManifest struct {
	ID        string `json:"id"`
	FileName  string `json:"fileName"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256"`
	ChunkSize int64  `json:"chunkSize"`
}

func peerBases(primary string) []string {
	seen := map[string]struct{}{}
	list := []string{}
	add := func(base string) {
		base = strings.TrimSuffix(strings.TrimSpace(base), "/")
		if base == "" {
			return
		}
		if _, ok := seen[base]; ok {
			return
		}
		seen[base] = struct{}{}
		list = append(list, base)
	}
	add(primary)

	peerMu.Lock()
	snapshot := make([]*Peer, 0, len(peers))
	for _, p := range peers {
		snapshot = append(snapshot, p)
	}
	peerMu.Unlock()
	for _, p := range snapshot {
		host := strings.TrimSpace(p.Host)
		if host == "" {
			host = strings.TrimSpace(p.IP)
		}
		if host == "" {
			continue
		}
		port := p.Port
		if port == 0 {
			port = 8080
		}
		add(fmt.Sprintf("http://%s:%d", host, port))
	}
	return list
}

func fetchManifestFromPeers(candidates []string, fileID string) (fileManifest, error) {
	for _, base := range candidates {
		url := fmt.Sprintf("%s/files/%s/manifest", base, fileID)
		resp, err := http.Get(url)
		if err != nil {
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			continue
		}
		var man fileManifest
		err = json.NewDecoder(resp.Body).Decode(&man)
		resp.Body.Close()
		if err != nil || man.ID == "" {
			continue
		}
		if man.ChunkSize <= 0 {
			man.ChunkSize = chunkSize
		}
		return man, nil
	}
	return fileManifest{}, fmt.Errorf("manifest failed from peers")
}

func fetchChunkFromPeers(candidates []string, fileID string, index int) ([]byte, string, string, error) {
	var lastErr error
	for _, base := range candidates {
		chunkURL := fmt.Sprintf("%s/files/%s/chunk?index=%d", base, fileID, index)
		if len(sharedKey) > 0 {
			chunkURL += "&enc=1"
		}
		resp, err := http.Get(chunkURL)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			lastErr = fmt.Errorf("chunk status %d from %s", resp.StatusCode, base)
			continue
		}

		expectedHash := strings.ToLower(strings.TrimSpace(resp.Header.Get("X-Chunk-SHA256")))
		var chunk []byte
		if len(sharedKey) > 0 {
			var enc struct {
				Nonce string `json:"nonce"`
				Data  string `json:"data"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&enc); err != nil {
				resp.Body.Close()
				lastErr = err
				continue
			}
			resp.Body.Close()
			chunk, err = decryptJSON(enc.Nonce, enc.Data)
			if err != nil {
				lastErr = err
				continue
			}
		} else {
			chunk, err = io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				lastErr = err
				continue
			}
		}

		sum := sha256.Sum256(chunk)
		actual := hex.EncodeToString(sum[:])
		if expectedHash != "" {
			if actual != expectedHash {
				lastErr = fmt.Errorf("chunk checksum mismatch from %s", base)
				continue
			}
		}
		return chunk, base, actual, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("chunk not available")
	}
	return nil, "", "", lastErr
}

func ensureDownloadSession(db *sql.DB, fileID string, man fileManifest, tempPath string, total int) error {
	now := time.Now().Format(time.RFC3339)
	_, err := db.Exec(`INSERT INTO file_download_sessions
		(file_id, file_name, file_size, file_sha256, chunk_size, total_chunks, completed_chunks, status, temp_path, last_error, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, 0, 'in_progress', ?, '', ?)
		ON CONFLICT(file_id) DO UPDATE SET
			file_name=excluded.file_name,
			file_size=excluded.file_size,
			file_sha256=excluded.file_sha256,
			chunk_size=excluded.chunk_size,
			total_chunks=excluded.total_chunks,
			temp_path=excluded.temp_path,
			updated_at=excluded.updated_at`,
		fileID, man.FileName, man.Size, man.SHA256, man.ChunkSize, total, tempPath, now)
	if err != nil {
		return err
	}

	for i := 0; i < total; i++ {
		_, err := db.Exec(`INSERT OR IGNORE INTO file_download_chunks
			(file_id, chunk_index, status, chunk_hash, source_peer, updated_at)
			VALUES (?, ?, 'not_downloaded', '', '', ?)`, fileID, i, now)
		if err != nil {
			return err
		}
	}
	return updateCompletedChunks(db, fileID)
}

func setDownloadSessionStatus(db *sql.DB, fileID, status, lastErr string) error {
	_, err := db.Exec("UPDATE file_download_sessions SET status = ?, last_error = ?, updated_at = ? WHERE file_id = ?",
		status, lastErr, time.Now().Format(time.RFC3339), fileID)
	return err
}

func upsertChunkStatus(db *sql.DB, fileID string, index int, status, hash, source string) error {
	_, err := db.Exec(`INSERT INTO file_download_chunks
		(file_id, chunk_index, status, chunk_hash, source_peer, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(file_id, chunk_index) DO UPDATE SET
			status=excluded.status,
			chunk_hash=excluded.chunk_hash,
			source_peer=excluded.source_peer,
			updated_at=excluded.updated_at`,
		fileID, index, status, hash, source, time.Now().Format(time.RFC3339))
	return err
}

func isChunkCompleted(db *sql.DB, fileID string, index int) (bool, error) {
	var status string
	err := db.QueryRow("SELECT status FROM file_download_chunks WHERE file_id = ? AND chunk_index = ?", fileID, index).Scan(&status)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return status == "completed", nil
}

func updateCompletedChunks(db *sql.DB, fileID string) error {
	var completed int
	if err := db.QueryRow("SELECT COUNT(1) FROM file_download_chunks WHERE file_id = ? AND status = 'completed'", fileID).Scan(&completed); err != nil {
		return err
	}
	_, err := db.Exec("UPDATE file_download_sessions SET completed_chunks = ?, updated_at = ? WHERE file_id = ?",
		completed, time.Now().Format(time.RFC3339), fileID)
	return err
}

func revalidateCompletedChunks(db *sql.DB, f *os.File, fileID string, man fileManifest) error {
	rows, err := db.Query("SELECT chunk_index, chunk_hash FROM file_download_chunks WHERE file_id = ? AND status = 'completed' ORDER BY chunk_index", fileID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var index int
		var expectedHash string
		if err := rows.Scan(&index, &expectedHash); err != nil {
			return err
		}
		offset := int64(index) * man.ChunkSize
		size := man.ChunkSize
		if offset+size > man.Size {
			size = man.Size - offset
		}
		if size <= 0 {
			_ = upsertChunkStatus(db, fileID, index, "incomplete", "", "")
			continue
		}
		buf := make([]byte, size)
		n, err := f.ReadAt(buf, offset)
		if err != nil && err != io.EOF {
			_ = upsertChunkStatus(db, fileID, index, "incomplete", "", "")
			continue
		}
		if int64(n) != size {
			_ = upsertChunkStatus(db, fileID, index, "incomplete", "", "")
			continue
		}
		sum := sha256.Sum256(buf)
		actual := hex.EncodeToString(sum[:])
		if expectedHash == "" || actual != expectedHash {
			_ = upsertChunkStatus(db, fileID, index, "incomplete", "", "")
		}
	}
	return updateCompletedChunks(db, fileID)
}

func startStorageJanitor(db *sql.DB) {
	runStorageJanitor(db)
	ticker := time.NewTicker(janitorInt)
	defer ticker.Stop()
	for range ticker.C {
		runStorageJanitor(db)
	}
}

func runStorageJanitor(db *sql.DB) {
	used, budget := currentStorageUsage()
	if budget <= 0 || used <= 0 {
		return
	}
	usagePct := int((used * 100) / budget)
	if usagePct < storageSoftLimitPct {
		return
	}

	for usagePct > 80 {
		var fileID, path, compressedPath string
		err := db.QueryRow(`SELECT id, path, compressed_path FROM files
			WHERE is_cached = 1 AND TRIM(path) <> ''
			ORDER BY CASE WHEN TRIM(last_accessed) = '' THEN '1970-01-01T00:00:00Z' ELSE last_accessed END ASC
			LIMIT 1`).Scan(&fileID, &path, &compressedPath)
		if err != nil {
			return
		}

		if strings.TrimSpace(path) != "" {
			_ = os.Remove(path)
		}
		if strings.TrimSpace(compressedPath) != "" {
			_ = os.Remove(compressedPath)
		}
		now := time.Now().Format(time.RFC3339)
		_, _ = db.Exec("UPDATE files SET path = '', compressed_path = '', is_cached = 0, last_accessed = ? WHERE id = ?", now, fileID)
		_, _ = db.Exec("UPDATE storage_chunks SET is_present = 0, last_accessed = ? WHERE file_id = ?", now, fileID)

		used, budget = currentStorageUsage()
		if budget <= 0 || used <= 0 {
			return
		}
		usagePct = int((used * 100) / budget)
	}
}

func currentStorageUsage() (int64, int64) {
	budget := int64(5 * 1024 * 1024 * 1024) // 5GB default device budget
	if raw := strings.TrimSpace(os.Getenv("RDE_STORAGE_BUDGET_BYTES")); raw != "" {
		if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil && parsed > 0 {
			budget = parsed
		}
	}

	var used int64
	_ = filepath.Walk(uploadDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		used += info.Size()
		return nil
	})
	return used, budget
}

func ensureFilePresent(db *sql.DB, meta *FileMeta) error {
	if meta == nil {
		return fmt.Errorf("missing file metadata")
	}
	if strings.TrimSpace(meta.Path) != "" {
		if _, err := os.Stat(meta.Path); err == nil {
			return nil
		}
	}
	localPath, err := downloadFileChunks(db, "", *meta)
	if err != nil {
		return err
	}
	now := time.Now().Format(time.RFC3339)
	_, _ = db.Exec("UPDATE files SET path = ?, is_cached = 1, last_accessed = ? WHERE id = ?", localPath, now, meta.ID)
	meta.Path = localPath
	meta.IsCached = true
	if info, err := os.Stat(localPath); err == nil {
		meta.Size = info.Size()
	}
	_ = indexFileChunks(db, meta.ID, localPath, meta.Size, chunkSize)
	return nil
}

func touchFileAccess(db *sql.DB, fileID string, chunkIndex *int) error {
	now := time.Now().Format(time.RFC3339)
	if _, err := db.Exec("UPDATE files SET last_accessed = ? WHERE id = ?", now, fileID); err != nil {
		return err
	}
	if chunkIndex != nil && *chunkIndex >= 0 {
		_, _ = db.Exec("UPDATE storage_chunks SET last_accessed = ? WHERE file_id = ? AND chunk_index = ?", now, fileID, *chunkIndex)
	}
	return nil
}

func indexFileChunks(db *sql.DB, fileID, path string, rawSize int64, chunkBytes int64) error {
	fileID = strings.TrimSpace(fileID)
	path = strings.TrimSpace(path)
	if fileID == "" || path == "" {
		return fmt.Errorf("invalid indexing input")
	}
	if chunkBytes <= 0 {
		chunkBytes = chunkSize
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	now := time.Now().Format(time.RFC3339)
	offset := int64(0)
	index := 0
	buf := make([]byte, chunkBytes)
	for {
		n, readErr := io.ReadFull(f, buf)
		if readErr == io.EOF {
			break
		}
		if readErr != nil && readErr != io.ErrUnexpectedEOF {
			return readErr
		}
		chunk := buf[:n]
		sum := sha256.Sum256(chunk)
		_, err := db.Exec(`INSERT INTO storage_chunks
			(file_id, chunk_index, offset_bytes, chunk_size, chunk_sha256, last_accessed, is_present)
			VALUES (?, ?, ?, ?, ?, ?, 1)
			ON CONFLICT(file_id, chunk_index) DO UPDATE SET
				offset_bytes=excluded.offset_bytes,
				chunk_size=excluded.chunk_size,
				chunk_sha256=excluded.chunk_sha256,
				last_accessed=excluded.last_accessed,
				is_present=1`,
			fileID, index, offset, n, hex.EncodeToString(sum[:]), now)
		if err != nil {
			return err
		}
		offset += int64(n)
		index++
		if readErr == io.ErrUnexpectedEOF {
			break
		}
	}
	if rawSize <= 0 {
		rawSize = offset
	}
	_, _ = db.Exec(`UPDATE files
		SET size = CASE WHEN size <= 0 THEN ? ELSE size END,
			raw_size = CASE WHEN raw_size <= 0 THEN ? ELSE raw_size END,
			compressed_size = CASE WHEN compressed_size <= 0 THEN ? ELSE compressed_size END,
			compression = CASE WHEN TRIM(compression) = '' THEN 'none' ELSE compression END,
			last_accessed = ?,
			is_cached = 1
		WHERE id = ?`, rawSize, rawSize, rawSize, now, fileID)
	return nil
}

func buildCompressedArtifact(db *sql.DB, fileID, srcPath string, rawSize int64) {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("RDE_ENABLE_STORAGE_COMPRESSION")), "false") {
		return
	}
	if strings.TrimSpace(srcPath) == "" || strings.TrimSpace(fileID) == "" {
		return
	}
	if err := os.MkdirAll(compressedDir, os.ModePerm); err != nil {
		return
	}
	src, err := os.Open(srcPath)
	if err != nil {
		return
	}
	defer src.Close()

	dstPath := filepath.Join(compressedDir, fileID+".sz")
	dst, err := os.Create(dstPath)
	if err != nil {
		return
	}
	zw, err := zstd.NewWriter(dst, zstd.WithEncoderLevel(zstd.SpeedBetterCompression))
	if err != nil {
		_ = dst.Close()
		return
	}
	if _, err := io.Copy(zw, src); err != nil {
		_ = zw.Close()
		_ = dst.Close()
		_ = os.Remove(dstPath)
		return
	}
	if err := zw.Close(); err != nil {
		_ = dst.Close()
		_ = os.Remove(dstPath)
		return
	}
	if err := dst.Close(); err != nil {
		_ = os.Remove(dstPath)
		return
	}

	info, err := os.Stat(dstPath)
	if err != nil {
		return
	}
	now := time.Now().Format(time.RFC3339)
	if rawSize <= 0 {
		if srcInfo, err := os.Stat(srcPath); err == nil {
			rawSize = srcInfo.Size()
		}
	}
	if rawSize > 0 && info.Size() >= rawSize {
		_ = os.Remove(dstPath)
		_, _ = db.Exec(`UPDATE files SET raw_size = ?, compressed_size = ?, compressed_path = '', compression = 'none', last_accessed = ? WHERE id = ?`,
			rawSize, rawSize, now, fileID)
		return
	}
	_, _ = db.Exec(`UPDATE files SET raw_size = ?, compressed_size = ?, compressed_path = ?, compression = 'sz-zstd', last_accessed = ? WHERE id = ?`,
		rawSize, info.Size(), dstPath, now, fileID)
}

func sanitizeFileName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "file.bin"
	}
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "/", "_")
	return name
}

type discoveryAnnounce struct {
	ClassroomID string `json:"classroomId"`
	NodeID      string `json:"nodeId"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Role        string `json:"role"`
}

type discoveryPeer struct {
	ClassroomID string `json:"classroomId"`
	NodeID      string `json:"nodeId"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Role        string `json:"role"`
	LastSeen    string `json:"lastSeen"`
}

func announceDiscovery(db *sql.DB) {
	if discoveryURL == "" || advertiseHost == "" {
		return
	}
	rows, err := db.Query("SELECT id FROM rooms")
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		rows.Scan(&id)
		payload := discoveryAnnounce{
			ClassroomID: id,
			NodeID:      nodeID,
			Host:        advertiseHost,
			Port:        mustPort(httpPort),
			Role:        "peer",
		}
		body, _ := json.Marshal(payload)
		http.Post(discoveryURL+"/announce", "application/json", strings.NewReader(string(body)))
	}
}

func syncFromDiscovery(db *sql.DB) {
	if discoveryURL == "" {
		return
	}
	rows, err := db.Query("SELECT id FROM rooms")
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		rows.Scan(&id)
		url := discoveryURL + "/classroom/" + id
		resp, err := http.Get(url)
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil {
				resp.Body.Close()
			}
			continue
		}
		var list []discoveryPeer
		if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
			resp.Body.Close()
			continue
		}
		resp.Body.Close()
		for _, p := range list {
			if p.NodeID == nodeID {
				continue
			}
			upsertPeer(p.Host, p.Host, p.NodeID, p.Port)
			syncFromPeer(db, p.Host, p.Port)
		}
	}
}

func mustPort(port string) int {
	p, _ := strconv.Atoi(strings.TrimPrefix(port, ":"))
	if p == 0 {
		return 8080
	}
	return p
}
func handleGetRoomByID(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing id parameter", http.StatusBadRequest)
		return
	}
	row := db.QueryRow("SELECT id, title, description, teacher FROM rooms WHERE id = ?", id)
	var room struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Teacher     string `json:"teacher"`
	}
	err := row.Scan(&room.ID, &room.Title, &room.Description, &room.Teacher)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Room not found", http.StatusNotFound)
		} else {
			http.Error(w, "Database error", http.StatusInternalServerError)
		}
		return
	}
	json.NewEncoder(w).Encode(room)
}
