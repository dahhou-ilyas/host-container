package service

import (
	"context"
	"log/slog"
	"strconv"

	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/moby/moby/client"
)

// ContainerEventsService subscribes to the Docker daemon event stream and
// dispatches crash / OOM / health-check events to the RecoveryService.
// It reconnects automatically if the stream drops.
type ContainerEventsService struct {
	docker   *client.Client
	recovery *RecoveryService
}

func NewContainerEventsService(docker *client.Client, recovery *RecoveryService) *ContainerEventsService {
	return &ContainerEventsService{docker: docker, recovery: recovery}
}

// Start launches the event listener in a background goroutine.
// The goroutine respects ctx cancellation (used for graceful shutdown).
func (s *ContainerEventsService) Start(ctx context.Context) {
	go func() {
		slog.Info("docker events listener started")
		for {
			select {
			case <-ctx.Done():
				slog.Info("docker events listener stopped")
				return
			default:
				s.listen(ctx)
			}
		}
	}()
}

func (s *ContainerEventsService) listen(ctx context.Context) {
	// Only handle containers we created — ignore all other Docker activity on the daemon.
	f := filters.NewArgs()
	f.Add("type", "container")
	f.Add("label", "codedock.managed=true")

	msgCh, errCh := s.docker.Events(ctx, events.ListOptions{Filters: f})

	for {
		select {
		case <-ctx.Done():
			return
		case err := <-errCh:
			if err != nil && ctx.Err() == nil {
				slog.Error("docker events stream error, reconnecting", "err", err)
			}
			return // outer loop reconnects
		case msg := <-msgCh:
			s.dispatch(ctx, msg)
		}
	}
}

func (s *ContainerEventsService) dispatch(ctx context.Context, msg events.Message) {
	switch msg.Action {
	case "die":
		exitCode, _ := strconv.Atoi(msg.Actor.Attributes["exitCode"])
		if exitCode != 0 {
			// exitCode 0 = clean stop (user pressed Stop); non-zero = crash
			go s.recovery.HandleCrash(context.Background(), msg.Actor.ID, exitCode)
		}
	case "oom":
		// OOM event fires before "die" — mark the flag so the UI shows OOM badge
		go s.recovery.HandleOOM(context.Background(), msg.Actor.ID)
	case "health_status: healthy":
		go s.recovery.HandleHealthChange(context.Background(), msg.Actor.ID, "healthy")
	case "health_status: unhealthy":
		go s.recovery.HandleHealthChange(context.Background(), msg.Actor.ID, "unhealthy")
	case "health_status: starting":
		go s.recovery.HandleHealthChange(context.Background(), msg.Actor.ID, "starting")
	}
}
