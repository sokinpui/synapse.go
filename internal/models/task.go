package models

import "github.com/sokinpui/sllmi-go/v2"

type GenerationTask struct {
	TaskID    string        `json:"task_id"`
	Prompt    string        `json:"prompt"`
	ModelCode string        `json:"model_code"`
	Stream    bool          `json:"stream"`
	Config    *sllmi.Config `json:"config,omitempty"`
	Images    [][]byte      `json:"images,omitempty"`
}
