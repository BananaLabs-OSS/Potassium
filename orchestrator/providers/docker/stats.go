package docker

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
)

// ContainerStats represents a point-in-time resource usage snapshot.
type ContainerStats struct {
	ContainerID  string  `json:"container_id"`
	Name         string  `json:"name"`
	CPUPercent   float64 `json:"cpu_percent"`
	MemoryUsed   int64   `json:"memory_used"`
	MemoryLimit  int64   `json:"memory_limit"`
	NetRxBytes   int64   `json:"net_rx_bytes"`
	NetTxBytes   int64   `json:"net_tx_bytes"`
	DiskReadBytes  int64 `json:"disk_read_bytes"`
	DiskWriteBytes int64 `json:"disk_write_bytes"`
	Timestamp    int64   `json:"timestamp"`
}

// Stats returns a one-shot resource usage snapshot for a single container.
func (d *DockerProvider) Stats(ctx context.Context, id string) (*ContainerStats, error) {
	resp, err := d.client.ContainerStats(ctx, id, false)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var stats container.StatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, err
	}

	// Get container name
	info, err := d.client.ContainerInspect(ctx, id)
	name := id[:12]
	if err == nil {
		name = info.Name
	}

	// Calculate CPU percent
	cpuPercent := 0.0
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	sysDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)
	if sysDelta > 0 && cpuDelta > 0 {
		cpuPercent = (cpuDelta / sysDelta) * float64(len(stats.CPUStats.CPUUsage.PercpuUsage)) * 100.0
	}

	// Sum network I/O across all interfaces
	var netRx, netTx int64
	for _, v := range stats.Networks {
		netRx += int64(v.RxBytes)
		netTx += int64(v.TxBytes)
	}

	// Sum block I/O
	var diskRead, diskWrite int64
	for _, entry := range stats.BlkioStats.IoServiceBytesRecursive {
		switch entry.Op {
		case "read", "Read":
			diskRead += int64(entry.Value)
		case "write", "Write":
			diskWrite += int64(entry.Value)
		}
	}

	return &ContainerStats{
		ContainerID:    id,
		Name:           name,
		CPUPercent:     cpuPercent,
		MemoryUsed:     int64(stats.MemoryStats.Usage),
		MemoryLimit:    int64(stats.MemoryStats.Limit),
		NetRxBytes:     netRx,
		NetTxBytes:     netTx,
		DiskReadBytes:  diskRead,
		DiskWriteBytes: diskWrite,
		Timestamp:      time.Now().Unix(),
	}, nil
}

// StatsAll returns resource usage snapshots for all running containers.
// Uses bounded concurrency of 5 goroutines.
func (d *DockerProvider) StatsAll(ctx context.Context) ([]ContainerStats, error) {
	containers, err := d.client.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return nil, err
	}

	var (
		mu      sync.Mutex
		results []ContainerStats
		wg      sync.WaitGroup
		sem     = make(chan struct{}, 5)
	)

	for _, c := range containers {
		if c.State != "running" {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(id string) {
			defer wg.Done()
			defer func() { <-sem }()

			s, err := d.Stats(ctx, id)
			if err != nil {
				return
			}
			mu.Lock()
			results = append(results, *s)
			mu.Unlock()
		}(c.ID)
	}

	wg.Wait()
	return results, nil
}
