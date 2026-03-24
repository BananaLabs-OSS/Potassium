package docker

import (
	"context"

	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
)

// ContainerEvent represents a Docker container lifecycle event.
type ContainerEvent struct {
	ContainerID string `json:"container_id"`
	Name        string `json:"name"`
	Action      string `json:"action"` // start, stop, die, restart, destroy
	Time        int64  `json:"time"`
}

// Events subscribes to Docker container lifecycle events and returns a channel.
// The channel closes when the context is cancelled or an error occurs.
func (d *DockerProvider) Events(ctx context.Context) (<-chan ContainerEvent, <-chan error) {
	eventCh := make(chan ContainerEvent, 32)
	errCh := make(chan error, 1)

	go func() {
		defer close(eventCh)
		defer close(errCh)

		msgs, errs := d.client.Events(ctx, events.ListOptions{
			Filters: filters.NewArgs(
				filters.Arg("type", "container"),
				filters.Arg("event", "start"),
				filters.Arg("event", "stop"),
				filters.Arg("event", "die"),
				filters.Arg("event", "restart"),
				filters.Arg("event", "destroy"),
			),
		})

		for {
			select {
			case msg, ok := <-msgs:
				if !ok {
					return
				}
				name := msg.Actor.Attributes["name"]
				eventCh <- ContainerEvent{
					ContainerID: msg.Actor.ID,
					Name:        name,
					Action:      string(msg.Action),
					Time:        msg.Time,
				}
			case err, ok := <-errs:
				if !ok {
					return
				}
				errCh <- err
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	return eventCh, errCh
}
