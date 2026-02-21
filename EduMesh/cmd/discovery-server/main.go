package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type announce struct {
	ClassroomID string `json:"classroomId"`
	NodeID      string `json:"nodeId"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Role        string `json:"role"`
}

type peer struct {
	ClassroomID string    `json:"classroomId"`
	NodeID      string    `json:"nodeId"`
	Host        string    `json:"host"`
	Port        int       `json:"port"`
	Role        string    `json:"role"`
	LastSeen    time.Time `json:"lastSeen"`
}

var mu sync.Mutex
var store = map[string]map[string]*peer{}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "7070"
	}

	go cleanupLoop()

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	http.HandleFunc("/announce", handleAnnounce)
	http.HandleFunc("/classroom/", handleClassroom)

	log.Println("Discovery server on :" + port)
	log.Fatal(http.ListenAndServe(":"+port, enableCORS(http.DefaultServeMux)))
}

func enableCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func handleAnnounce(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Invalid method", http.StatusMethodNotAllowed)
		return
	}
	var a announce
	if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	if a.ClassroomID == "" || a.NodeID == "" || a.Host == "" || a.Port == 0 {
		http.Error(w, "Missing fields", http.StatusBadRequest)
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if _, ok := store[a.ClassroomID]; !ok {
		store[a.ClassroomID] = map[string]*peer{}
	}
	store[a.ClassroomID][a.NodeID] = &peer{
		ClassroomID: a.ClassroomID,
		NodeID:      a.NodeID,
		Host:        a.Host,
		Port:        a.Port,
		Role:        a.Role,
		LastSeen:    time.Now(),
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func handleClassroom(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/classroom/")
	if id == "" {
		http.Error(w, "Missing classroom id", http.StatusBadRequest)
		return
	}
	mu.Lock()
	defer mu.Unlock()
	list := []peer{}
	for _, p := range store[id] {
		list = append(list, *p)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

func cleanupLoop() {
	t := time.NewTicker(30 * time.Second)
	for range t.C {
		expire := time.Now().Add(-2 * time.Minute)
		mu.Lock()
		for classID, m := range store {
			for nodeID, p := range m {
				if p.LastSeen.Before(expire) {
					delete(m, nodeID)
				}
			}
			if len(m) == 0 {
				delete(store, classID)
			}
		}
		mu.Unlock()
	}
}
