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
	Name      string `json:"name,omitempty" yaml:"name,omitempty"`
	Range     string `json:"range,omitempty" yaml:"range,omitempty"` // e.g. "25565-25599" — allocate host port from this range
}

type AllocateRequest struct {
	Image       string            `json:"image" yaml:"image"`
	Environment map[string]string `json:"environment" yaml:"environment"`
	Volumes     map[string]string `json:"volumes" yaml:"volumes"`
	Ports       []PortBinding     `json:"ports" yaml:"ports"`
	Network     string            `json:"network,omitempty" yaml:"network,omitempty"`
	IP          string            `json:"ip,omitempty" yaml:"ip,omitempty"`
	MemoryLimit    int64   `json:"memory_limit,omitempty" yaml:"memory_limit,omitempty"`
	CPULimit       float64 `json:"cpu_limit,omitempty" yaml:"cpu_limit,omitempty"`
	DiskIOReadBps  int64   `json:"disk_io_read_bps,omitempty" yaml:"disk_io_read_bps,omitempty"`
	DiskIOWriteBps int64   `json:"disk_io_write_bps,omitempty" yaml:"disk_io_write_bps,omitempty"`
	DiskSizeLimit  int64   `json:"disk_size_limit,omitempty" yaml:"disk_size_limit,omitempty"`
	PidsLimit      int64   `json:"pids_limit,omitempty" yaml:"pids_limit,omitempty"`
	MemorySwap     int64   `json:"memory_swap,omitempty" yaml:"memory_swap,omitempty"`
}

type Provider interface {
	List(ctx context.Context, filter map[string]string) ([]Server, error)
	Get(ctx context.Context, id string) (*Server, error)
	Allocate(ctx context.Context, req AllocateRequest) (*Server, error)
	Deallocate(ctx context.Context, id string) error
	Restart(ctx context.Context, id string) error
	Exec(ctx context.Context, id string, cmd []string) (string, error)
	Logs(ctx context.Context, id string, tail int) (string, error)
}
