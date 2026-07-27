package docker

import (
	"sync"
	"testing"

	"github.com/docker/docker/client"
)

func TestDockerProviderCloseIsNilSafeAndIdempotent(t *testing.T) {
	var nilProvider *DockerProvider
	if err := nilProvider.Close(); err != nil {
		t.Fatalf("nil Close: %v", err)
	}

	sdk, err := client.NewClientWithOpts(client.WithHost("tcp://127.0.0.1:2375"))
	if err != nil {
		t.Fatalf("new Docker SDK client: %v", err)
	}
	provider := &DockerProvider{client: sdk}

	var wait sync.WaitGroup
	for range 32 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if err := provider.Close(); err != nil {
				t.Errorf("concurrent Close: %v", err)
			}
		}()
	}
	wait.Wait()
	if err := provider.Close(); err != nil {
		t.Fatalf("replayed Close: %v", err)
	}
}
