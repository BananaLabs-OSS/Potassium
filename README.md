# Potassium

Core orchestration library for container management.

From [BananaLabs OSS](https://github.com/bananalabs-oss).

## Overview

Potassium provides:

- **Provider Interface**: Abstract container operations
- **Docker Provider**: Docker/Podman implementation
- **Registry**: In-memory server registry with filtering
- **Types**: Shared types for orchestration requests

## Installation

```bash
go get github.com/bananalabs-oss/potassium
```

## Usage

### Docker Provider

```go
import "github.com/bananalabs-oss/potassium/orchestrator/providers/docker"

provider, err := docker.New()
if err != nil {
    panic(err)
}

// Allocate container
server, err := provider.Allocate(ctx, orchestrator.AllocateRequest{
    Image: "localhost/hytale-server",
    Ports: []orchestrator.PortBinding{
        {Host: 5521, Container: 5521, Protocol: "udp"},
    },
    Environment: map[string]string{
        "SERVER_ID": "test-1",
    },
})

// List containers
servers, err := provider.List(ctx, nil)

// Deallocate
err = provider.Deallocate(ctx, server.ID)
```

### Overlay Network Mode

```go
server, err := provider.Allocate(ctx, orchestrator.AllocateRequest{
    Image:   "localhost/hytale-server",
    Network: "banananet",
    IP:      "10.99.0.10",
    Ports: []orchestrator.PortBinding{
        {Container: 5520, Protocol: "udp"},
    },
})
```

### Registry

```go
import "github.com/bananalabs-oss/potassium/registry"

reg, _ := registry.New()

// Register
reg.Register(registry.ServerInfo{
    ID:         "skywars-1",
    Type:       registry.TypeGame,
    Mode:       "skywars",
    Host:       "10.99.0.10",
    Port:       5520,
    MaxPlayers: 8,
})

// Query
servers := reg.List(&registry.ListFilter{
    Type:          registry.TypeGame,
    Mode:          "skywars",
    HasReadyMatch: true,
})

// Update
reg.Update("skywars-1", func(s *registry.ServerInfo) {
    s.Players = 4
})
```

### Peel Client

```go
import "github.com/bananalabs-oss/potassium/relay"

client := relay.NewClient("http://localhost:8080")

// Set route
err := client.SetRoute("192.168.1.50", "10.99.0.10:5520")

// Delete route
err := client.DeleteRoute("192.168.1.50")

// List routes
routes, err := client.ListRoutes()
```

### Binary Diffing

```go
import "github.com/bananalabs-oss/potassium/diff"

// bsdiff — pure Go, good for files under ~500MB
patch, err := diff.Generate("old_file.bin", "new_file.bin")
err = diff.Apply("old_file.bin", "patch.bsdiff", "new_file.bin")

// HDiffPatch — exec wrapper, handles multi-GB files with constant memory
err = diff.GenerateHDiff("old_file.bin", "new_file.bin", "patch.hdiff")
err = diff.ApplyHDiff("old_file.bin", "patch.hdiff", "new_file.bin")
```

HDiffPatch requires `hdiffz`/`hpatchz` binaries. Place them in one of:

- `bin/{os}/` next to the executable
- `bin/{os}/` in the working directory
- System PATH

Download from [HDiffPatch releases](https://github.com/sisong/HDiffPatch/releases).

### Manifest

```go
import "github.com/bananalabs-oss/potassium/manifest"

// Create manifest
m := manifest.New("1.0.0", "1.1.0")
m.AddFile(manifest.FileEntry{
Path:      "game.exe",
Action:    manifest.ActionPatch,
OldHash:   oldHash,
NewHash:   newHash,
PatchFile: "game.exe.patch",
Algorithm: "hdiff",
})
m.Save("manifest.json")

// Load manifest
m, err := manifest.Load("manifest.json")
```

## Types

### AllocateRequest

```go
type AllocateRequest struct {
    Image       string
    Ports       []PortBinding
    Environment map[string]string
    Network     string  // Overlay network name
    IP          string  // Static IP on network
}
```

### ServerInfo

```go
type ServerInfo struct {
    ID          string
    Type        ServerType  // "lobby" or "game"
    Mode        string
    Host        string
    Port        int
 WebhookPort int
    Players     int
    MaxPlayers  int
    Matches     map[string]MatchInfo
    Metadata    map[string]string
}
```

## License

MIT

