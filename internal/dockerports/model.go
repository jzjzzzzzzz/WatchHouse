package dockerports

import "time"

const (
	MaxInspectBytes = 16 << 20
	MaxContainers   = 1024
)

type Binding struct {
	ContainerID   string `json:"container_id"`
	ContainerName string `json:"container_name"`
	NetworkMode   string `json:"network_mode"`
	Protocol      string `json:"protocol"`
	ContainerPort uint16 `json:"container_port"`
	HostIP        string `json:"host_ip"`
	HostPort      uint16 `json:"host_port"`
}

type Report struct {
	SchemaVersion         int       `json:"schema_version"`
	ObservedAt            time.Time `json:"observed_at"`
	ContainerCount        int       `json:"container_count"`
	PublishedBindingCount int       `json:"published_binding_count"`
	Bindings              []Binding `json:"bindings"`
}
