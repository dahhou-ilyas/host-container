package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/go-connections/nat"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/moby/moby/client"
)

type ContainerInfo struct {
	ContainerID string `json:"container_id"`
	ProjectID   string `json:"project_id"`
	ProjectName string `json:"project_name"`
	FolderPath  string `json:"folder_path"`
	Port        string `json:"port,omitempty"`
	Status      string `json:"status"`
	UserId      string `json:"userId"`
}

type ContainerManager struct {
	client              *client.Client
	basePath            string
	repo *ContainerRepo
	userRepo *UserRepo
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


func NewContainerManager(basePath string,pool *pgxpool.Pool) (*ContainerManager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	if err := os.MkdirAll(basePath, os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to create base path: %w", err)
	}

	// dans cette partie je doit initier le db pool et le passé dans le container manager
	return &ContainerManager{
		client:     cli,
		basePath:   basePath,
		repo: NewContainerRepo(pool),
		userRepo: NewUserRepo(pool),
	}, nil
}

func (cm *ContainerManager) CreateContainer(ctx context.Context, project Project, imageName string) (*ContainerInfo, error) {
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

	folderPath := filepath.Join(cm.basePath, project.Name,project.ID)

	log.Printf("%s", folderPath)

	if err := os.MkdirAll(folderPath, os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to create project folder: %w", err)
	}

	if err := cm.pullImageIfNeeded(ctx, imageName); err != nil {
		return nil, fmt.Errorf("failed to pull image: %w", err)
	}

	containerConfig := &container.Config{
		Image:      imageName,
		Tty:        true,
		WorkingDir: "/workspace",
		Cmd:        []string{"/bin/sh"}, // Shell par défaut
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
			Memory:   512 * 1024 * 1024, // 512 MB
			NanoCPUs: 1000000000,        // 1 CPU
		},
		AutoRemove: false,
	}

	resp, err := cm.client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	if err := cm.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		cm.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	info := &ContainerInfo{
		ContainerID: resp.ID,
		ProjectID:   project.ID,
		ProjectName: project.Name,
		FolderPath:  folderPath,
		Status:      "running",
		UserId:      project.UserId,
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

	folderPath := filepath.Join(cm.basePath, project.Name, project.ID)
	if err := os.MkdirAll(folderPath, os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to create project folder: %w", err)
	}

	if err := cm.pullImageIfNeeded(ctx, imageName); err != nil {
		return nil, fmt.Errorf("failed to pull image: %w", err)
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
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	if err := cm.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		cm.client.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		return nil, fmt.Errorf("failed to start container: %w", err)
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

	info := &ContainerInfo{
		ContainerID: resp.ID,
		ProjectID:   project.ID,
		ProjectName: project.Name,
		FolderPath:  folderPath,
		Port:        assignedPort,
		Status:      "running",
		UserId:      project.UserId,
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

func (cm *ContainerManager) ExecCommand(ctx context.Context, projectID string, cmd []string) (string, error) {
	info, err := cm.repo.GetContainerByID(ctx,cm.repo.GetDB(),projectID)



	if err != nil {
		return "", fmt.Errorf("container not found for project %s", projectID)
	}

	execConfig := container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	}

	execID, err := cm.client.ContainerExecCreate(ctx, info.ContainerID, execConfig)
	if err != nil {
		return "", fmt.Errorf("failed to create exec: %w", err)
	}

	resp, err := cm.client.ContainerExecAttach(ctx, execID.ID, container.ExecStartOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to attach exec: %w", err)
	}
	defer resp.Close()

	output, err := io.ReadAll(resp.Reader)
	if err != nil {
		return "", fmt.Errorf("failed to read output: %w", err)
	}

	return cleanOutput(string(output)), nil
}

func (cm *ContainerManager) StopContainer(ctx context.Context, projectID string) error {
	tx, err := cm.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	info, err := cm.repo.GetContainerByID(ctx,cm.repo.GetDB(),projectID)
	if err  != nil{
		return fmt.Errorf("container not found for project %s", projectID)
	}

	timeout := 10
	if err := cm.client.ContainerStop(ctx, info.ContainerID, container.StopOptions{Timeout: &timeout}); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	info.Status = "stopped"

	
	_ , err = cm.repo.UpdateContainer(ctx,tx,info.ProjectID,info)
	if err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	return nil
}

func (cm *ContainerManager) StartContainer(ctx context.Context, projectID string) error {
	tx, err := cm.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	info, err := cm.repo.GetContainerByID(ctx,cm.repo.GetDB(),projectID)
	if err != nil {
		return fmt.Errorf("container not found for project %s", projectID)
	}

	if err := cm.client.ContainerStart(ctx, info.ContainerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	info.Status = "running"

	_ , err = cm.repo.UpdateContainer(ctx,tx,info.ProjectID,info)
	if err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
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

	info, err := cm.repo.GetContainerByID(ctx,tx,projectID)
	if err != nil{
		return fmt.Errorf("container not found for project %s", projectID)
	}

	timeout := 5
	cm.client.ContainerStop(ctx, info.ContainerID, container.StopOptions{Timeout: &timeout})

	if err := cm.client.ContainerRemove(ctx, info.ContainerID, container.RemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	if removeFolder {
		if err := os.RemoveAll(info.FolderPath); err != nil {
			return fmt.Errorf("failed to remove folder: %w", err)
		}
	}
	err = cm.repo.DeleteContainer(ctx,tx,projectID)

	if err!=nil {
		return err
	}

	if err = tx.Commit(ctx); err != nil {
		return err
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