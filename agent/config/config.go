package config

import "time"

const (
	HeartbeatInterval    = 5 * time.Second
	BroadcastingInterval = 10 * time.Second
	ReconnectTimeout     = 10 * time.Second
)
