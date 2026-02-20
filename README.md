# Potassium

Shared library for the BananaKit ecosystem.

From [BananaLabs OSS](https://github.com/bananalabs-oss).

## Overview

Potassium provides:

- **Middleware**: JWT auth, service-to-service auth, standard error responses
- **Config**: Environment variable helpers and CLI flag resolution
- **Provider Interface**: Abstract container operations
- **Docker Provider**: Docker/Podman implementation
- **Registry**: In-memory server registry with filtering
- **Types**: Shared types for orchestration requests

## Installation

```bash
go get github.com/bananalabs-oss/potassium
```

## Usage

### Config

```go
import "github.com/bananalabs-oss/potassium/config"

// Required env var (fatal if missing)
jwtSecret := config.RequireEnv("JWT_SECRET")

// Env var with fallback
dbURL := config.EnvOrDefault("DATABASE_URL", "sqlite://app.db")
port := config.EnvOrDefaultInt("PORT", 8080)

// CLI flag > env var > default resolution
host := config.Resolve(cliHost, os.Getenv("HOST"), "0.0.0.0")
workers := config.ResolveInt(cliWorkers, getEnvInt("WORKERS"), 4)
```

### Middleware

```go
import potassium "github.com/bananalabs-oss/potassium/middleware"

// JWT auth (sets account_id and session_id in context)
router.Use(potassium.JWTAuth(potassium.JWTConfig{
    Secret: []byte(jwtSecret),
}))

// Service-to-service auth (X-Service-Token header)
router.Use(potassium.ServiceAuth(serviceSecret))

// Standard error response
c.JSON(http.StatusNotFound, potassium.ErrorResponse{
    Error:   "not_found",
    Message: "Resource not found",
})
```

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

