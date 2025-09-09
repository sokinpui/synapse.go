package models

import "sllmi-go"

// GenerationTask represents a task to be processed by a worker.
type GenerationTask struct {
	TaskID    string          `json:"task_id"`
	Prompt    string          `json:"prompt"`
	ModelCode string          `json:"model_code"`
	Stream    bool            `json:"stream"`
	Config    *sllmigo.Config `json:"config,omitempty"`
}
