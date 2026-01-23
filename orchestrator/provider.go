package orchestrator

import "context"

type ServerStatus string

const (
	StatusRunning ServerStatus = "running"
	StatusStopped ServerStatus = "stopped"
	StatusError   ServerStatus = "error"
)

type Server struct {
	ID     string         `json:"id"`
	Name   string         `json:"name"`
	Status ServerStatus   `json:"status"`
	IP     string         `json:"ip"`
	Ports  map[string]int `json:"port"`
}

type PortBinding struct {
	Host      int    `json:"host"`
	Container int    `json:"container"`
	Protocol  string `json:"protocol"`
}

type AllocateRequest struct {
	Image       string            `json:"image"`
	Environment map[string]string `json:"environment"`
	Volumes     map[string]string `json:"volumes"`
	Ports       []PortBinding     `json:"ports"`
	Network     string            `json:"network,omitempty"`
	IP          string            `json:"ip,omitempty"`
}

type Provider interface {
	List(ctx context.Context, filter map[string]string) ([]Server, error)
	Get(ctx context.Context, id string) (*Server, error)
	Allocate(ctx context.Context, req AllocateRequest) (*Server, error)
	Deallocate(ctx context.Context, id string) error
}
