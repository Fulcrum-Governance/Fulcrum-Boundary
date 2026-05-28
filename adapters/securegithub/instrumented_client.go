package securegithub

import (
	"context"
	"fmt"
	"sync"
)

type InstrumentedGitHubClient struct {
	Base GitHubClient

	mu            sync.Mutex
	readCalls     int
	mutationCalls int
	lastMutation  LiveFileMutationRequest
	lastRead      LiveIssueRequest
}

func NewInstrumentedGitHubClient(base GitHubClient) *InstrumentedGitHubClient {
	return &InstrumentedGitHubClient{Base: base}
}

func (c *InstrumentedGitHubClient) GetIssue(ctx context.Context, req LiveIssueRequest) (LiveIssue, error) {
	c.mu.Lock()
	c.readCalls++
	c.lastRead = req
	c.mu.Unlock()
	if c.Base == nil {
		return LiveIssue{}, fmt.Errorf("instrumented GitHub client has no base client")
	}
	return c.Base.GetIssue(ctx, req)
}

func (c *InstrumentedGitHubClient) CreateOrUpdateFile(ctx context.Context, req LiveFileMutationRequest) error {
	c.mu.Lock()
	c.mutationCalls++
	c.lastMutation = req
	c.mu.Unlock()
	if c.Base == nil {
		return fmt.Errorf("instrumented GitHub client has no base client")
	}
	return c.Base.CreateOrUpdateFile(ctx, req)
}

func (c *InstrumentedGitHubClient) ReadCalls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.readCalls
}

func (c *InstrumentedGitHubClient) MutationCalls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.mutationCalls
}

func (c *InstrumentedGitHubClient) MutationCalled() bool {
	return c.MutationCalls() > 0
}
