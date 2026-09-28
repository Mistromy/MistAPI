package nirupama

type NirupamaMsg struct {
	Messages    int      `json:"messages_tracked"`
	Servers     int      `json:"guild_count"`
	Members     int      `json:"user_count"`
	Uptime      *float32 `json:"uptime"`             // uptime percentage over the last 90 days
	EpochMS     int64    `json:"epoch_ms"`           // per-message — updates constantly, drives latency
	HeartbeatMS int64    `json:"heartbeat_epoch_ms"` // periodic "phone home" — drives online/offline
}
