package service

import (
	"context"
	"fmt"

	dockernetwork "github.com/docker/docker/api/types/network"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type NetworkInfo struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	DockerName string            `json:"docker_name"`
	CreatedAt  string            `json:"created_at"`
	Members    []ContainerMember `json:"members"`
}

type ContainerMember struct {
	ProjectID string `json:"project_id"`
	Name      string `json:"name"`
	Alias     string `json:"alias"`
	Status    string `json:"status"`
}

type NetworkManager struct {
	cm   *ContainerManager
	pool *pgxpool.Pool
}

func NewNetworkManager(cm *ContainerManager, pool *pgxpool.Pool) *NetworkManager {
	return &NetworkManager{cm: cm, pool: pool}
}

func (m *NetworkManager) CreateNetwork(ctx context.Context, userID, name string) (*NetworkInfo, error) {
	var maxNets, currentNets int
	if err := m.pool.QueryRow(ctx,
		`SELECT pl.max_networks FROM plans pl JOIN users u ON u.plan_id=pl.id WHERE u.id=$1::bigint`,
		userID).Scan(&maxNets); err != nil {
		return nil, fmt.Errorf("plan lookup failed: %w", err)
	}
	m.pool.QueryRow(ctx, `SELECT COUNT(*) FROM networks WHERE user_id=$1::bigint`, userID).Scan(&currentNets)
	if currentNets >= maxNets {
		return nil, fmt.Errorf("network limit reached: your plan allows %d network(s)", maxNets)
	}

	// Namespaced by userID to avoid collisions between users on the Docker daemon
	dockerName := fmt.Sprintf("codedock_%s_%s", userID, name)

	resp, err := m.cm.client.NetworkCreate(ctx, dockerName, dockernetwork.CreateOptions{
		Driver:     "bridge",
		Attachable: true,
		Labels:     map[string]string{"codedock.user_id": userID, "codedock.name": name},
	})
	if err != nil {
		return nil, fmt.Errorf("docker NetworkCreate failed: %w", err)
	}

	var netID, createdAt string
	err = m.pool.QueryRow(ctx,
		`INSERT INTO networks (user_id, name, docker_net_id, docker_net_name)
		 VALUES ($1::bigint, $2, $3, $4) RETURNING id::text, created_at::text`,
		userID, name, resp.ID, dockerName,
	).Scan(&netID, &createdAt)
	if err != nil {
		m.cm.client.NetworkRemove(ctx, resp.ID)
		return nil, fmt.Errorf("db insert failed: %w", err)
	}

	return &NetworkInfo{ID: netID, Name: name, DockerName: dockerName, CreatedAt: createdAt, Members: []ContainerMember{}}, nil
}

func (m *NetworkManager) ConnectContainer(ctx context.Context, userID, netID, projectID, alias string) error {
	var dockerNetID string
	if err := m.pool.QueryRow(ctx,
		`SELECT docker_net_id FROM networks WHERE id=$1::uuid AND user_id=$2::bigint`,
		netID, userID).Scan(&dockerNetID); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("network not found or access denied")
		}
		return fmt.Errorf("network lookup failed: %w", err)
	}

	// containers.id (BIGINT PK) is the "project_id" in the API.
	// Two users can have containers named "web-app"; project_id (BIGINT) is globally unique.
	var containerDockerID string
	var containerDBID int64
	if err := m.pool.QueryRow(ctx,
		`SELECT container_id, id FROM containers WHERE id=$1::bigint AND user_id=$2::bigint`,
		projectID, userID).Scan(&containerDockerID, &containerDBID); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("container not found or access denied")
		}
		return fmt.Errorf("container lookup failed: %w", err)
	}

	if err := m.cm.client.NetworkConnect(ctx, dockerNetID, containerDockerID, &dockernetwork.EndpointSettings{
		Aliases: []string{alias},
	}); err != nil {
		return fmt.Errorf("docker NetworkConnect failed: %w", err)
	}

	_, err := m.pool.Exec(ctx,
		`INSERT INTO container_networks (container_id, network_id, alias)
		 VALUES ($1, $2::uuid, $3)
		 ON CONFLICT (container_id, network_id) DO UPDATE SET alias=$3`,
		containerDBID, netID, alias)
	return err
}

func (m *NetworkManager) DisconnectContainer(ctx context.Context, userID, netID, projectID string) error {
	var dockerNetID string
	if err := m.pool.QueryRow(ctx,
		`SELECT docker_net_id FROM networks WHERE id=$1::uuid AND user_id=$2::bigint`,
		netID, userID).Scan(&dockerNetID); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("network not found or access denied")
		}
		return fmt.Errorf("network lookup failed: %w", err)
	}

	var containerDockerID string
	var containerDBID int64
	if err := m.pool.QueryRow(ctx,
		`SELECT container_id, id FROM containers WHERE id=$1::bigint AND user_id=$2::bigint`,
		projectID, userID).Scan(&containerDockerID, &containerDBID); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("container not found or access denied")
		}
		return fmt.Errorf("container lookup failed: %w", err)
	}

	if err := m.cm.client.NetworkDisconnect(ctx, dockerNetID, containerDockerID, false); err != nil {
		return fmt.Errorf("docker NetworkDisconnect failed: %w", err)
	}

	_, err := m.pool.Exec(ctx,
		`DELETE FROM container_networks WHERE container_id=$1 AND network_id=$2::uuid`,
		containerDBID, netID)
	return err
}

func (m *NetworkManager) DeleteNetwork(ctx context.Context, userID, netID string) error {
	var dockerNetID string
	if err := m.pool.QueryRow(ctx,
		`SELECT docker_net_id FROM networks WHERE id=$1::uuid AND user_id=$2::bigint`,
		netID, userID).Scan(&dockerNetID); err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("network not found or access denied")
		}
		return fmt.Errorf("network lookup failed: %w", err)
	}

	// Disconnect all remaining containers before removing (force=true handles stopped containers)
	if inspect, err := m.cm.client.NetworkInspect(ctx, dockerNetID, dockernetwork.InspectOptions{}); err == nil {
		for cid := range inspect.Containers {
			m.cm.client.NetworkDisconnect(ctx, dockerNetID, cid, true)
		}
	}

	if err := m.cm.client.NetworkRemove(ctx, dockerNetID); err != nil {
		return fmt.Errorf("docker NetworkRemove failed: %w", err)
	}

	// CASCADE on networks deletes container_networks automatically
	_, err := m.pool.Exec(ctx,
		`DELETE FROM networks WHERE id=$1::uuid AND user_id=$2::bigint`, netID, userID)
	return err
}

func (m *NetworkManager) ListNetworks(ctx context.Context, userID string) ([]NetworkInfo, error) {
	rows, err := m.pool.Query(ctx, `
		SELECT n.id::text, n.name, n.docker_net_name, n.created_at::text,
		       cn.alias, c.id::text, c.project_name, c.status
		FROM networks n
		LEFT JOIN container_networks cn ON cn.network_id = n.id
		LEFT JOIN containers c ON c.id = cn.container_id
		WHERE n.user_id = $1::bigint
		ORDER BY n.created_at DESC, c.id ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	netMap := make(map[string]*NetworkInfo)
	var order []string

	for rows.Next() {
		var nid, name, dockerName, createdAt string
		var alias, projID, projName, status *string
		if err := rows.Scan(&nid, &name, &dockerName, &createdAt, &alias, &projID, &projName, &status); err != nil {
			return nil, err
		}
		if _, ok := netMap[nid]; !ok {
			netMap[nid] = &NetworkInfo{
				ID:         nid,
				Name:       name,
				DockerName: dockerName,
				CreatedAt:  createdAt,
				Members:    []ContainerMember{},
			}
			order = append(order, nid)
		}
		if alias != nil && projID != nil {
			netMap[nid].Members = append(netMap[nid].Members, ContainerMember{
				ProjectID: *projID,
				Name:      *projName,
				Alias:     *alias,
				Status:    *status,
			})
		}
	}

	result := make([]NetworkInfo, 0, len(order))
	for _, id := range order {
		result = append(result, *netMap[id])
	}
	return result, nil
}
