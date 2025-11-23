package models

import (
	"time"

	"github.com/google/uuid"
)

type Environment struct {
	ID             uuid.UUID              `json:"id"`
	VolumeName     string                 `json:"volumeName"`
	MainModule     string                 `json:"mainModule"`
	CreatedAt      time.Time              `json:"createdAt"`
	LastExecutedAt *time.Time             `json:"lastExecutedAt,omitempty"`
	ExecutionCount int                    `json:"executionCount"`
	Status         string                 `json:"status"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	TTLSeconds     int                    `json:"ttlSeconds"`
}

type SetupRequest struct {
	MainModule  string            `json:"mainModule"`
	Modules     map[string]string `json:"modules"`
	Permissions *Permissions      `json:"permissions,omitempty"`
	TTLSeconds  int               `json:"ttlSeconds,omitempty"`
}

type ExecuteRequest struct {
	Data   interface{}     `json:"data,omitempty"`
	Env    map[string]string `json:"env,omitempty"`
	Limits *ResourceLimits `json:"limits,omitempty"`
}

type Permissions struct {
	AllowNet    interface{} `json:"allowNet,omitempty"`
	AllowRead   interface{} `json:"allowRead,omitempty"`
	AllowWrite  interface{} `json:"allowWrite,omitempty"`
	AllowEnv    interface{} `json:"allowEnv,omitempty"`
	AllowRun    interface{} `json:"allowRun,omitempty"`
	AllowFfi    bool        `json:"allowFfi,omitempty"`
	AllowHrtime bool        `json:"allowHrtime,omitempty"`
}

type ResourceLimits struct {
	TimeoutMs int `json:"timeoutMs"`
	MemoryMb  int `json:"memoryMb"`
}

type ExecutionResponse struct {
	ID         uuid.UUID `json:"id"`
	ExitCode   int       `json:"exitCode"`
	Stdout     string    `json:"stdout"`
	Stderr     string    `json:"stderr"`
	DurationMs int64     `json:"durationMs"`
}
