package registry

import (
	"errors"
	"sync"
)

type ServerType string

const (
	TypeLobby ServerType = "lobby"
	TypeGame  ServerType = "game"
)

type MatchStatus string

const (
	StatusReady    MatchStatus = "ready"
	StatusBusy     MatchStatus = "busy"
	StatusStarting MatchStatus = "starting"
)

type ServerInfo struct {
	ID          string     `json:"ID"`
	Type        ServerType `json:"Type"`
	Mode        string     `json:"Mode"`
	Host        string     `json:"Host"`
	Port        int        `json:"Port"`
	WebhookPort int        `json:"WebhookPort,omitempty"`

	// For lobby servers (no matches)
	Players    int `json:"Players"`
	MaxPlayers int `json:"MaxPlayers"`

	// For game servers
	Matches map[string]MatchInfo `json:"Matches"`

	Metadata map[string]string
}

type MatchInfo struct {
	Status  MatchStatus `json:"Status"`
	Need    int         `json:"Need"`
	Players []string    `json:"Players"`
}

type ListFilter struct {
	Type          ServerType // Filter by lobby/game
	Mode          string     // Filter by skywars/survival
	HasCapacity   bool       // has player space (for lobbies)
	HasReadyMatch bool       // has a ready match (for game servers)
}

type Registry struct {
	mu      sync.RWMutex // Protects the map
	servers map[string]ServerInfo
}

func New() (*Registry, error) {
	return &Registry{
		servers: make(map[string]ServerInfo),
	}, nil
}

func (r *Registry) Register(server ServerInfo) error {
	// Lock Registry
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if server ID is empty
	if server.ID == "" {
		return errors.New("Server ID required")
	}

	// Initialize Matches map if game server and nil
	if server.Type == TypeGame && server.Matches == nil {
		server.Matches = make(map[string]MatchInfo)
	}

	// Add server to the map
	r.servers[server.ID] = server
	return nil
}

func (r *Registry) Unregister(id string) {
	// Lock
	r.mu.Lock()
	defer r.mu.Unlock()

	// Delete from map
	delete(r.servers, id)
}

func (r *Registry) Get(id string) (ServerInfo, bool) {
	// RLock
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Get from map
	server, ok := r.servers[id]

	// Return server and whether it exists
	return server, ok
}

func (r *Registry) Update(id string, update func(info *ServerInfo)) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	server, ok := r.servers[id]
	if !ok {
		return errors.New("Server not found")
	}

	update(&server)
	r.servers[id] = server
	return nil
}

func (r *Registry) UpdateMatch(serverID string, matchID string, match MatchInfo) error {
	// Lock
	r.mu.Lock()
	defer r.mu.Unlock()

	// Get server
	server, ok := r.servers[serverID]
	if !ok {
		return errors.New("Server not found")
	}

	// Get match
	if server.Matches == nil {
		server.Matches = make(map[string]MatchInfo)
	}

	// Update and return match
	server.Matches[matchID] = match
	r.servers[serverID] = server
	return nil
}

func (r *Registry) List(filter *ListFilter) []ServerInfo {
	// RLock
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []ServerInfo

	for _, server := range r.servers {
		// No Filter?
		if filter == nil {
			for _, server := range r.servers {
				result = append(result, server)
			}
			return result
		}

		// Check Type filter
		if filter.Type != "" && server.Type != filter.Type {
			continue // Skip this server
		}

		// Check Mode filter
		if filter.Mode != "" && server.Mode != filter.Mode {
			continue // skip this server
		}

		// Check HasCapacity (for lobbies)
		if filter.HasCapacity && server.Players >= server.MaxPlayers {
			continue // full, skip
		}

		// Check HasReadyMatch (for game servers)
		if filter.HasReadyMatch {
			hasReady := false
			for _, match := range server.Matches {
				if match.Status == StatusReady {
					hasReady = true
					break
				}
			}
			if !hasReady {
				continue // no ready matches, skip
			}
		}

		result = append(result, server)
	}

	return result
}

func (r *Registry) FindReadyMatch(mode string) (ServerInfo, string, bool) {
	// RLock
	r.mu.RLock()
	defer r.mu.RLock()

	// Loop through matches, find one that's ready
	for _, server := range r.servers {
		// Only game servers with matching mode
		if server.Type != TypeGame || server.Mode != mode {
			continue
		}

		// Find a ready match
		for matchID, match := range server.Matches {
			if match.Status == StatusReady {
				return server, matchID, true // found it!
			}
		}
	}
	// Nothing found
	return ServerInfo{}, "", false
}

func (r *Registry) FindLobby() (ServerInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, server := range r.servers {
		if server.Type == TypeLobby && server.Players < server.MaxPlayers {
			return server, true
		}
	}

	return ServerInfo{}, false
}

func (r *Registry) RemoveMatch(serverID string, matchID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	server, ok := r.servers[serverID]
	if !ok {
		return errors.New("server not found")
	}

	delete(server.Matches, matchID)
	r.servers[serverID] = server
	return nil
}
