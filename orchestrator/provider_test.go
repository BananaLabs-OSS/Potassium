package orchestrator

import (
	"encoding/json"
	"testing"
)

func TestPortBinding_JSONRoundTrip(t *testing.T) {
	original := PortBinding{
		Host:      5521,
		Container: 25565,
		Protocol:  "tcp",
		Name:      "game",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded PortBinding
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded != original {
		t.Fatalf("roundtrip mismatch: got %+v, want %+v", decoded, original)
	}
}

func TestPortBinding_JSONOmitEmptyName(t *testing.T) {
	p := PortBinding{
		Host:      5521,
		Container: 25565,
		Protocol:  "tcp",
	}

	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	// Name should be omitted from JSON when empty
	var raw map[string]interface{}
	json.Unmarshal(data, &raw)

	if _, exists := raw["name"]; exists {
		t.Error("empty Name should be omitted from JSON")
	}
}

func TestPortBinding_JSONWithName(t *testing.T) {
	jsonStr := `{"host":5521,"container":25565,"protocol":"tcp","name":"game"}`

	var p PortBinding
	if err := json.Unmarshal([]byte(jsonStr), &p); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if p.Name != "game" {
		t.Errorf("Name = %q, want %q", p.Name, "game")
	}
	if p.Host != 5521 {
		t.Errorf("Host = %d, want 5521", p.Host)
	}
	if p.Container != 25565 {
		t.Errorf("Container = %d, want 25565", p.Container)
	}
	if p.Protocol != "tcp" {
		t.Errorf("Protocol = %q, want %q", p.Protocol, "tcp")
	}
}

func TestPortBinding_JSONWithoutName(t *testing.T) {
	jsonStr := `{"host":5521,"container":25565,"protocol":"tcp"}`

	var p PortBinding
	if err := json.Unmarshal([]byte(jsonStr), &p); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if p.Name != "" {
		t.Errorf("Name should be empty when not in JSON, got %q", p.Name)
	}
}

func TestAllocateRequest_JSONWithNamedPorts(t *testing.T) {
	req := AllocateRequest{
		Image: "test-server:latest",
		Ports: []PortBinding{
			{Host: 5521, Container: 25565, Protocol: "tcp", Name: "game"},
			{Host: 5522, Container: 19132, Protocol: "udp", Name: "bedrock"},
		},
		Environment: map[string]string{"MOTD": "test"},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded AllocateRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if len(decoded.Ports) != 2 {
		t.Fatalf("expected 2 ports, got %d", len(decoded.Ports))
	}
	if decoded.Ports[0].Name != "game" {
		t.Errorf("port[0].Name = %q, want %q", decoded.Ports[0].Name, "game")
	}
	if decoded.Ports[1].Name != "bedrock" {
		t.Errorf("port[1].Name = %q, want %q", decoded.Ports[1].Name, "bedrock")
	}
}
