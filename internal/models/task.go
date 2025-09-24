package models

import "github.com/sokinpui/sllmi-go"

type GenerationTask struct {
	TaskID    string        `json:"task_id"`
	Prompt    string        `json:"prompt"`
	ModelCode string        `json:"model_code"`
	Stream    bool          `json:"stream"`
	Config    *sllmi.Config `json:"config,omitempty"`
}
