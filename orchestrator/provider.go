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
	Image       string            `json:"image"`
	Environment map[string]string `json:"environment"`
	Volumes     map[string]string `json:"volumes"`
	Ports       []PortBinding     `json:"ports"`
	Network     string            `json:"network,omitempty"`
	IP          string            `json:"ip,omitempty"`
	MemoryLimit    int64   `json:"memory_limit,omitempty"`     // Memory limit in bytes (0 = no limit)
	CPULimit       float64 `json:"cpu_limit,omitempty"`        // CPU limit (e.g. 0.5 = half a core, 2.0 = two cores, 0 = no limit)
	DiskIOReadBps  int64   `json:"disk_io_read_bps,omitempty"` // Disk read bytes/sec limit (0 = no limit)
	DiskIOWriteBps int64   `json:"disk_io_write_bps,omitempty"` // Disk write bytes/sec limit (0 = no limit)
	DiskSizeLimit  int64   `json:"disk_size_limit,omitempty"`  // Disk size limit in bytes (0 = no limit, requires overlay2+xfs)
	PidsLimit      int64   `json:"pids_limit,omitempty"`       // Max processes (0 = no limit)
	MemorySwap     int64   `json:"memory_swap,omitempty"`      // Memory+swap limit in bytes (set equal to MemoryLimit to disable swap, -1 = unlimited, 0 = unset)
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
