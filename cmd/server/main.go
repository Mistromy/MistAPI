package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"sync"
	"time"

	"charm.land/log/v2"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

//type ServerMsg struct {
//	Server string `json:"server"`
//	Status string `json:"status"`
//	Cpu    int    `json:"cpu"`
//	Uptime uint64 `json:"stats"`
//}

type NirupamaMsg struct {
	Messages    int      `json:"messages_tracked"`
	Servers     int      `json:"guild_count"`
	Members     int      `json:"user_count"`
	Uptime      *float32 `json:"uptime"`             // uptime percentage over the last 90 days
	EpochMS     int64    `json:"epoch_ms"`           // per-message — updates constantly, drives latency
	HeartbeatMS int64    `json:"heartbeat_epoch_ms"` // periodic "phone home" — drives online/offline
}

type uptimeResp struct {
	Aggregates struct {
		SuccessRate *float64 `json:"success_rate"`
	} `json:"aggregates"`
}

const publicPort = ":6767"
const privatePort = ":6769"
const nirupamaUptimeURL = "https://nirupama.cronitorstatus.com/api/incident-uptime?component=c32863b23756ef1b&time=90d"

var (
	mu     sync.Mutex
	latest NirupamaMsg
)

// the list of everyone currently connected to the public websocket,
// and the function that sends a copy of an update to all of them.

type Hub struct {
	mu      sync.Mutex
	clients map[*websocket.Conn]struct{}
}

func newHub() *Hub {
	return &Hub{clients: make(map[*websocket.Conn]struct{})}
}

func (h *Hub) add(c *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c] = struct{}{}
}

func (h *Hub) remove(c *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, c)
}

func (h *Hub) broadcast(msg NirupamaMsg) {
	h.mu.Lock()
	defer h.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for c := range h.clients {
		err := wsjson.Write(ctx, c, msg)
		if err != nil {
			fmt.Println("write:", err)
		}
	}
}

func fetchUptime(client *http.Client) (float32, bool) {
	resp, err := client.Get(nirupamaUptimeURL)
	if err != nil {
		fmt.Println("uptime fetch:", err)
		return 0, false
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		fmt.Println("uptime fetch status:", resp.StatusCode)
		return 0, false
	}

	var u uptimeResp
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		fmt.Println("uptime decode:", err)
		return 0, false
	}
	if u.Aggregates.SuccessRate == nil {
		fmt.Println("uptime: success_rate absent")
		return 0, false
	}
	return float32(*u.Aggregates.SuccessRate * 100), true
}

func pollUptime(hub *Hub) {
	client := &http.Client{Timeout: 10 * time.Second}
	fails := 0
	for {
		pct, ok := fetchUptime(client)
		mu.Lock()
		if ok {
			fails = 0
			latest.Uptime = &pct
		} else if fails++; fails >= 6 {
			latest.Uptime = nil // 30 min of silence — stop claiming a number
		}
		snapshot := latest
		mu.Unlock()
		hub.broadcast(snapshot)
		time.Sleep(5 * time.Minute)
	}
}

var allowedOrigins = map[string]bool{
	"https://mista.tech":          true,
	"https://nirupama.mista.tech": true,
}

func withCORS(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if o := r.Header.Get("Origin"); allowedOrigins[o] {
			w.Header().Set("Access-Control-Allow-Origin", o)
			w.Header().Add("Vary", "Origin")
		}
		h(w, r)
	}
}

func fmtUptime(u *float32) string {
	if u == nil {
		return "—"
	}
	return fmt.Sprintf("%.3f%%", *u)
}

func main() {
	debug := flag.Bool("debug", false, "enable debug logging")
	flag.Parse()
	if *debug {
		log.SetLevel(log.DebugLevel)
	}

	hub := newHub()
	publicMux := http.NewServeMux()
	privateMux := http.NewServeMux()

	go pollUptime(hub)

	publicMux.HandleFunc("/nirupama", withCORS(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mu.Lock()
		snapshot := latest
		mu.Unlock()
		err := json.NewEncoder(w).Encode(snapshot)
		if err != nil {
			return
		}
	}))

	publicMux.HandleFunc("/nirupama/live", withCORS(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true,
		})
		if err != nil {
			return
		}
		defer func(conn *websocket.Conn) {
			_ = conn.CloseNow()

		}(conn)

		hub.add(conn)
		defer hub.remove(conn)

		mu.Lock()
		snapshot := latest
		mu.Unlock()
		_ = wsjson.Write(r.Context(), conn, snapshot) // send what we already know, right away

		for { // this connection isn't sending anything — just watch for it closing
			if _, _, err := conn.Read(r.Context()); err != nil {
				return
			}
		}
	}))

	privateMux.HandleFunc("/nirupama/live", func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func(conn *websocket.Conn) {
			_ = conn.CloseNow()
		}(conn)

		for {
			var p NirupamaMsg
			if err := wsjson.Read(r.Context(), conn, &p); err != nil {
				return
			}
			mu.Lock()
			latest.Messages = p.Messages
			if p.EpochMS > 0 {
				latest.EpochMS = p.EpochMS
			}
			if p.HeartbeatMS > 0 {
				latest.HeartbeatMS = p.HeartbeatMS
			}
			if p.Servers > 0 {
				latest.Servers = p.Servers
			}
			if p.Members > 0 {
				latest.Members = p.Members
			}
			snapshot := latest
			mu.Unlock()
			log.Debug("Updated",
				"msgs", snapshot.Messages,
				"guilds", snapshot.Servers,
				"members", snapshot.Members,
				"uptime", fmtUptime(snapshot.Uptime))

			hub.broadcast(snapshot) // send the full snapshot to every connected client
		}
	})
	go func() {
		log.Info("Public API on", "port", publicPort)
		if err := http.ListenAndServe(publicPort, publicMux); err != nil {
			log.Error("Error starting public API:", err)
		}
	}()
	log.Info("Private API via tailscale on", "port", privatePort)
	if err := http.ListenAndServe(privatePort, privateMux); err != nil {
		log.Error("Error starting private API:", err)
	}
}
