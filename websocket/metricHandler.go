package websocket

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/dahhou-ilyas/host-container/service"
	"github.com/dahhou-ilyas/host-container/utils"
	"github.com/docker/docker/api/types/container"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type MetricHandler struct {
	containerManager *service.ContainerManager
}

func NewMetricHandler(containerManager *service.ContainerManager) (*MetricHandler, error) {
	return &MetricHandler{containerManager: containerManager}, nil
}

func (mh *MetricHandler) WsHandler(w http.ResponseWriter, r *http.Request) {

	containerID := r.URL.Query().Get("containerId")

	if containerID == "" {
		utils.RespondError(w, "containerId requis", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("Error upgrading:", err)
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				cancel()
				return
			}
		}
	}()

	decoder , ioReader , err := mh.containerManager.GetMetricOfContainer(ctx,containerID,conn);


	if err != nil {
		return
	}

	defer ioReader.Close()



	for {
        var v container.StatsResponse

        if err := decoder.Decode(&v); err != nil {
            break
        }

        snapshot := buildSnapshot(v)


        if err := conn.WriteJSON(snapshot); err != nil {
            break
        }
    }
}

type MetricsSnapshot struct {
	Timestamp   time.Time   `json:"timestamp"`
	CPU         float64 `json:"cpu"`        
	MemoryUsed  uint64  `json:"memoryUsed"` 
	MemoryTotal uint64  `json:"memoryTotal"`
	NetworkRx   uint64  `json:"networkRx"`  
	NetworkTx   uint64  `json:"networkTx"`  
	BlockRead   uint64  `json:"blockRead"`  
	BlockWrite  uint64  `json:"blockWrite"` 
}

func buildSnapshot(current container.StatsResponse) MetricsSnapshot {

	Timestamp := current.Read;

	//Δcontainer
	deltaTotalUsage := current.CPUStats.CPUUsage.TotalUsage - current.PreCPUStats.CPUUsage.TotalUsage
	//Δsystem
	deltaSystemCpuUsage := current.CPUStats.SystemUsage - current.PreCPUStats.SystemUsage

	CPUpercentage := (deltaTotalUsage/deltaSystemCpuUsage) * uint64(current.CPUStats.OnlineCPUs) * 100


	MemoryTotal := current.MemoryStats.Limit;
	MemoryUsage := current.MemoryStats.Usage - current.MemoryStats.Stats["inactive_file"]


	return MetricsSnapshot{Timestamp: Timestamp,CPU: float64(CPUpercentage),MemoryUsed: MemoryUsage,MemoryTotal: MemoryTotal}


}

