package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/moby/moby/client"
)

type ContainerInfo struct {
	ContainerID string     `json:"container_id"`
	ProjectID   string     `json:"project_id"`
	ProjectName string     `json:"project_name"`
	ImageName   string     `json:"image_name"`
	FolderPath  string     `json:"folder_path"`
	Port        string     `json:"port,omitempty"`
	Status      string     `json:"status"`
	UserId      string     `json:"userId"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CreatedAt   *time.Time `json:"created_at,omitempty"`
}

type ContainerManager struct {
	client          *client.Client
	basePath        string
	repo            *ContainerRepo
	userRepo        *UserRepo
	autoStopTimeout time.Duration
	stopChan        chan struct{}
}

const DEFAULT_MAX_CONTAINERS_PER_USER = 10

var ErrLimitContainer = errors.New("you reached the number limit of containers")


func getMaxContainersPerUser() int {
	maxContainers := DEFAULT_MAX_CONTAINERS_PER_USER
	if envMax := os.Getenv("MAX_CONTAINERS_PER_USER"); envMax != "" {
		if parsed, err := strconv.Atoi(envMax); err == nil && parsed > 0 {
			maxContainers = parsed
		}
	}
	return maxContainers
}


func NewContainerManager(basePath string, pool *pgxpool.Pool, autoStopTimeout time.Duration) (*ContainerManager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	if err := os.MkdirAll(basePath, os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to create base path: %w", err)
	}

	return &ContainerManager{
		client:          cli,
		basePath:        basePath,
		repo:            NewContainerRepo(pool),
		userRepo:        NewUserRepo(pool),
		autoStopTimeout: autoStopTimeout,
		stopChan:        make(chan struct{}),
	}, nil
}

func (cm *ContainerManager) CreateContainer(ctx context.Context, project Project, imageName string) (_ *ContainerInfo, err error) {
	tx, err := cm.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	_, containers, err := cm.userRepo.GetContainersForUserByID(ctx, tx, project.UserId)

	if err != nil {
		return nil, fmt.Errorf("failed to get the user and container: %w", err)
	}

	maxContainers := getMaxContainersPerUser()
	if len(containers) >= maxContainers {
		return nil, fmt.Errorf("container limit reached: you have %d containers (max: %d)", len(containers), maxContainers)
	}

	if err = cm.pullImageIfNeeded(ctx, imageName); err != nil {
		return nil, fmt.Errorf("failed to pull image: %w", err)
	}

	folderPath := filepath.Join(cm.basePath, project.Name, project.ID)
	if err = os.MkdirAll(folderPath, os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to create project folder: %w", err)
	}
	defer func() {
		if err != nil {
			os.RemoveAll(folderPath)
		}
	}()

	containerConfig := &container.Config{
		Image:      imageName,
		Tty:        true,
		WorkingDir: "/workspace",
		Cmd:        []string{"/bin/sh"},
	}

	hostConfig := &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: folderPath,
				Target: "/workspace",
			},
		},
		Resources: container.Resources{
			Memory:   512 * 1024 * 1024,
			NanoCPUs: 1000000000,
		},
		AutoRemove: false,
	}

	resp, err := cm.client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}
	defer func() {
		if err != nil {
			cm.client.ContainerStop(ctx, resp.ID, container.StopOptions{})
			cm.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		}
	}()

	if err = cm.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	if pkgErr := cm.installPackages(ctx, resp.ID, []string{"tree"}); pkgErr != nil {
		log.Printf("warning: failed to install packages in container %s: %v", resp.ID, pkgErr)
	}

	now := time.Now()
	info := &ContainerInfo{
		ContainerID: resp.ID,
		ProjectID:   project.ID,
		ProjectName: project.Name,
		ImageName:   imageName,
		FolderPath:  folderPath,
		Status:      "running",
		UserId:      project.UserId,
		StartedAt:   &now,
		CreatedAt:   &now,
	}

	id, err := cm.repo.CreateContainer(ctx, tx, *info)
	if err != nil {
		return nil, err
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}

	info.ProjectID = id
	return info, nil
}

func (cm *ContainerManager) CreateContainerWithPort(ctx context.Context, project Project, imageName string, containerPort string) (*ContainerInfo, error) {
	tx, err := cm.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	_, containers, err := cm.userRepo.GetContainersForUserByID(ctx, tx, project.UserId)

	if err != nil {
		return nil, fmt.Errorf("failed to get the user and container: %w", err)
	}

	maxContainers := getMaxContainersPerUser()
	if len(containers) >= maxContainers {
		return nil, fmt.Errorf("container limit reached: you have %d containers (max: %d)", len(containers), maxContainers)
	}

	

	if err := cm.pullImageIfNeeded(ctx, imageName); err != nil {
		return nil, fmt.Errorf("failed to pull image: %w", err)
	}

	folderPath := filepath.Join(cm.basePath, project.Name, project.ID)
	if err := os.MkdirAll(folderPath, os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to create project folder: %w", err)
	}

	exposedPort := nat.Port(containerPort + "/tcp")
	containerConfig := &container.Config{
		Image:        imageName,
		Tty:          true,
		WorkingDir:   "/workspace",
		ExposedPorts: nat.PortSet{exposedPort: struct{}{}},
	}

	hostConfig := &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: folderPath,
				Target: "/workspace",
			},
		},
		PortBindings: nat.PortMap{
			exposedPort: []nat.PortBinding{
				{HostIP: "0.0.0.0", HostPort: ""},
			},
		},
		Resources: container.Resources{
			Memory:   512 * 1024 * 1024,
			NanoCPUs: 1000000000,
		},
	}

	resp, err := cm.client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		removeFolderRollBack(folderPath)
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	if err := cm.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		cm.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		removeFolderRollBack(folderPath)
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	if err := cm.installPackages(ctx, resp.ID, []string{"tree"}); err != nil {
		log.Printf("warning: failed to install packages in container %s: %v", resp.ID, err)
	}

	// Récupérer le port assigné
	inspect, err := cm.client.ContainerInspect(ctx, resp.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	assignedPort := ""
	if bindings, ok := inspect.NetworkSettings.Ports[exposedPort]; ok && len(bindings) > 0 {
		assignedPort = bindings[0].HostPort
	}

	now := time.Now()
	info := &ContainerInfo{
		ContainerID: resp.ID,
		ProjectID:   project.ID,
		ProjectName: project.Name,
		ImageName:   imageName,
		FolderPath:  folderPath,
		Port:        assignedPort,
		Status:      "running",
		UserId:      project.UserId,
		StartedAt:   &now,
		CreatedAt:   &now,
	}

	id , err := cm.repo.CreateContainer(ctx,tx,*info)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	info.ProjectID = id

	return info, nil
}

func (cm *ContainerManager) ExecCommand(ctx context.Context, projectID string, cmd []string) (string, string, error) {
	info, err := cm.repo.GetContainerByID(ctx,cm.repo.GetDB(),projectID)



	if err != nil {
		return "", "", fmt.Errorf("container not found for project %s", projectID)
	}

	execConfig := container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	}

	execID, err := cm.client.ContainerExecCreate(ctx, info.ContainerID, execConfig)
	if err != nil {
		return "", "", fmt.Errorf("failed to create exec: %w", err)
	}

	resp, err := cm.client.ContainerExecAttach(ctx, execID.ID, container.ExecStartOptions{})
	if err != nil {
		return "", "", fmt.Errorf("failed to attach exec: %w", err)
	}
	defer resp.Close()

	var stdout, stderr bytes.Buffer
    _, err = stdcopy.StdCopy(&stdout, &stderr, resp.Reader)
    if err != nil {
        return "", "", fmt.Errorf("failed to read output: %w", err)
    }

    return stdout.String(), stderr.String(), nil

}

func (cm *ContainerManager) StopContainer(ctx context.Context, projectID string) (err error) {
	tx, err := cm.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	info, err := cm.repo.GetContainerByID(ctx, cm.repo.GetDB(), projectID)
	if err != nil {
		return fmt.Errorf("container not found for project %s", projectID)
	}

	timeout := 10
	if err = cm.client.ContainerStop(ctx, info.ContainerID, container.StopOptions{Timeout: &timeout}); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}
	// Compensation : si la DB échoue, on redémarre le container
	defer func() {
		if err != nil {
			if startErr := cm.client.ContainerStart(ctx, info.ContainerID, container.StartOptions{}); startErr != nil {
				log.Printf("rollback failed: could not restart container %s: %v", info.ContainerID, startErr)
			}
		}
	}()

	info.Status = "stopped"
	_, err = cm.repo.UpdateContainer(ctx, tx, info.ProjectID, info)
	if err != nil {
		return err
	}

	if err = tx.Commit(ctx); err != nil {
		return err
	}

	return nil
}

func (cm *ContainerManager) StartContainer(ctx context.Context, projectID string) (err error) {
	tx, err := cm.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	info, err := cm.repo.GetContainerByID(ctx, cm.repo.GetDB(), projectID)
	if err != nil {
		return fmt.Errorf("container not found for project %s", projectID)
	}

	if err = cm.client.ContainerStart(ctx, info.ContainerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}
	// Compensation : si la DB échoue, on re-stoppe le container
	defer func() {
		if err != nil {
			timeout := 10
			if stopErr := cm.client.ContainerStop(ctx, info.ContainerID, container.StopOptions{Timeout: &timeout}); stopErr != nil {
				log.Printf("rollback failed: could not stop container %s: %v", info.ContainerID, stopErr)
			}
		}
	}()

	now := time.Now()
	info.Status = "running"
	info.StartedAt = &now

	_, err = cm.repo.UpdateContainer(ctx, tx, info.ProjectID, info)
	if err != nil {
		return err
	}

	if err = tx.Commit(ctx); err != nil {
		return err
	}

	return nil
}

func (cm *ContainerManager) RemoveContainer(ctx context.Context, projectID string, removeFolder bool) error {
	tx, err := cm.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	info, err := cm.repo.GetContainerByID(ctx, tx, projectID)
	if err != nil {
		return fmt.Errorf("container not found for project %s", projectID)
	}

	// DB d'abord (réversible via rollback)
	err = cm.repo.DeleteContainer(ctx, tx, projectID)
	if err != nil {
		return err
	}

	if err = tx.Commit(ctx); err != nil {
		return err
	}

	// Après commit : cleanup Docker + filesystem (best-effort, loguer les erreurs)
	timeout := 5
	cm.client.ContainerStop(ctx, info.ContainerID, container.StopOptions{Timeout: &timeout})

	if err := cm.client.ContainerRemove(ctx, info.ContainerID, container.RemoveOptions{Force: true}); err != nil {
		log.Printf("WARNING: orphan container %s needs manual cleanup: %v", info.ContainerID, err)
	}

	if removeFolder {
		if err := os.RemoveAll(info.FolderPath); err != nil {
			log.Printf("WARNING: orphan folder %s needs manual cleanup: %v", info.FolderPath, err)
		}
	}

	return nil
}

func (cm *ContainerManager) GetContainerInfo(ctx context.Context ,projectID string) (*ContainerInfo, error) {

	info, err := cm.repo.GetContainerByID(ctx , cm.repo.GetDB() , projectID)
	if err != nil {
		return nil, fmt.Errorf("container not found for project %s", projectID)
	}

	return &info, nil
}


func (cm *ContainerManager) execInContainer(ctx context.Context, containerID string, cmd []string) error {
	execConfig := container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	}
	execID, err := cm.client.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return err
	}
	resp, err := cm.client.ContainerExecAttach(ctx, execID.ID, container.ExecStartOptions{})
	if err != nil {
		return err
	}
	defer resp.Close()
	_, err = io.Copy(io.Discard, resp.Reader)
	return err
}

func (cm *ContainerManager) installPackages(ctx context.Context, containerID string, packages []string) error {
	pkgList := strings.Join(packages, " ")

	// Detect package manager and install
	installCmds := []struct {
		check   []string
		install []string
	}{
		{
			check:   []string{"sh", "-c", "command -v apt-get"},
			install: []string{"sh", "-c", "apt-get update && apt-get install -y " + pkgList},
		},
		{
			check:   []string{"sh", "-c", "command -v apk"},
			install: []string{"sh", "-c", "apk add --no-cache " + pkgList},
		},
		{
			check:   []string{"sh", "-c", "command -v yum"},
			install: []string{"sh", "-c", "yum install -y " + pkgList},
		},
		{
			check:   []string{"sh", "-c", "command -v dnf"},
			install: []string{"sh", "-c", "dnf install -y " + pkgList},
		},
	}

	for _, ic := range installCmds {
		if err := cm.execInContainer(ctx, containerID, ic.check); err == nil {
			return cm.execInContainer(ctx, containerID, ic.install)
		}
	}

	return fmt.Errorf("no supported package manager found in container %s", containerID)
}

func (cm *ContainerManager) pullImageIfNeeded(ctx context.Context, imageName string) error {
	_, _, err := cm.client.ImageInspectWithRaw(ctx, imageName)
	if err == nil {
		return nil
	}

	reader, err := cm.client.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()

	_, err = io.Copy(io.Discard, reader)
	return err
}

func (cm *ContainerManager) StartAutoStopWatcher() {
	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		log.Printf("Auto-stop watcher started (timeout: %v)", cm.autoStopTimeout)
		for {
			select {
			case <-ticker.C:
				cm.checkAndStopExpiredContainers()
			case <-cm.stopChan:
				ticker.Stop()
				log.Println("Auto-stop watcher stopped")
				return
			}
		}
	}()
}

func (cm *ContainerManager) StopAutoStopWatcher() {
	close(cm.stopChan)
}

func (cm *ContainerManager) checkAndStopExpiredContainers() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	containers, err := cm.repo.GetRunningContainersOlderThan(ctx, cm.repo.GetDB(), cm.autoStopTimeout)
	if err != nil {
		log.Printf("Auto-stop watcher: failed to query expired containers: %v", err)
		return
	}

	for _, c := range containers {
		log.Printf("Auto-stop watcher: stopping container %s (project %s), running since %v", c.ContainerID, c.ProjectID, c.StartedAt)
		if err := cm.StopContainer(ctx, c.ProjectID); err != nil {
			log.Printf("Auto-stop watcher: failed to stop container %s: %v", c.ProjectID, err)
		} else {
			log.Printf("Auto-stop watcher: successfully stopped container %s", c.ProjectID)
		}
	}
}

func (cm *ContainerManager) GetMetricOfContainer(ctx context.Context, containerID string,conn *websocket.Conn) (*json.Decoder,io.ReadCloser,error) {
	stats, err := cm.client.ContainerStats(ctx, containerID, true)

	if err != nil {
		return nil,nil,fmt.Errorf("failed to GET METRIC OF THE CONTAINER: %w", err)
    }

	decoder := json.NewDecoder(stats.Body);

	return decoder,stats.Body,nil
}


func (cm *ContainerManager) Close() error {
	return cm.client.Close()
}



// I use this helpper later in futur
func (cm *ContainerManager) withTx(ctx context.Context, fn func(tx pgx.Tx) error) (err error) {
	tx, err := cm.repo.BeginTx(ctx)
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	if err = fn(tx); err != nil {
		return err
	}

	err = tx.Commit(ctx)
	return err
}


func cleanOutput(input string) string {
    return strings.Map(func(r rune) rune {
        if unicode.IsPrint(r) || r == '\n' || r == '\t' {
            return r
        }
        return -1
    }, input)
}

func removeFolderRollBack(folderPath string) error{
	if err := os.RemoveAll(folderPath); err != nil {
		return fmt.Errorf("failed to remove project folder: %w", err)
	}
	return nil
}