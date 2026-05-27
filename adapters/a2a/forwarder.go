package a2a

import (
	"context"
	"sync"
)

// Forwarder sends an allowed A2A task to the downstream agent.
type Forwarder interface {
	ForwardTask(ctx context.Context, task TaskEnvelope) (*TaskResponse, error)
}

// MemoryForwarder is a deterministic test/example forwarder.
type MemoryForwarder struct {
	mu       sync.Mutex
	Tasks    []TaskEnvelope
	Response *TaskResponse
	Err      error
}

func (f *MemoryForwarder) ForwardTask(_ context.Context, task TaskEnvelope) (*TaskResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Tasks = append(f.Tasks, task)
	if f.Err != nil {
		return nil, f.Err
	}
	if f.Response != nil {
		return cloneTaskResponse(f.Response), nil
	}
	return &TaskResponse{
		TaskID:    task.TaskID,
		ContextID: task.ContextID,
		Status:    StatusAllowed,
		Output:    map[string]any{"forwarded": true},
	}, nil
}

func (f *MemoryForwarder) Snapshot() []TaskEnvelope {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]TaskEnvelope, len(f.Tasks))
	copy(out, f.Tasks)
	return out
}

func cloneTaskResponse(resp *TaskResponse) *TaskResponse {
	if resp == nil {
		return nil
	}
	out := *resp
	if resp.Output != nil {
		out.Output = map[string]any{}
		for k, v := range resp.Output {
			out.Output[k] = v
		}
	}
	if resp.Artifacts != nil {
		out.Artifacts = append([]Artifact(nil), resp.Artifacts...)
	}
	return &out
}
