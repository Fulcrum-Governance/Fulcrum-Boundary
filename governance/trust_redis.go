package governance

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// RedisKV is the minimal key/value store RedisTrustBackend depends on. The
// in-repo RESPRedisClient implements it over a raw RESP connection; tests and
// alternate deployments can substitute any type with the same methods (for
// example, a go-redis adapter).
type RedisKV interface {
	// Get returns the value for key, or "" if the key is absent.
	Get(ctx context.Context, key string) (string, error)
	// Set writes value for key with the given TTL.
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	// Del removes key.
	Del(ctx context.Context, key string) error
}

// RedisTrustBackend is the kernel-mode trust backend. It reads and writes the
// fulcrum-trust Redis IPC state — per-agent keys of the form
// "{prefix}{agent_id}:circuit_state" (integer states 0=TRUSTED, 1=EVALUATING,
// 2=ISOLATED, 3=TERMINATED) and a companion "trust_score" key. It implements
// TrustBackend. When failClosed is set, a store error denies (returns
// TrustStateIsolated); otherwise CheckAgentState degrades open to
// TrustStateTrusted.
type RedisTrustBackend struct {
	store      RedisKV
	prefix     string
	timeout    time.Duration
	failClosed bool
}

// NewRedisTrustBackend returns a RedisTrustBackend backed by the given store
// and configured from cfg (zero-valued config fields take the kernel defaults).
// Use this to inject a custom RedisKV; NewRedisTrustBackendFromConfig builds
// the default RESP client for you.
func NewRedisTrustBackend(store RedisKV, cfg KernelTrustConfig) *RedisTrustBackend {
	cfg = cfg.withDefaults()
	return &RedisTrustBackend{
		store:      store,
		prefix:     cfg.IPCPrefix,
		timeout:    cfg.Timeout,
		failClosed: cfg.FailClosed,
	}
}

// NewRedisTrustBackendFromConfig builds a RedisTrustBackend whose store is a
// RESPRedisClient dialed from cfg.RedisURL. It returns an error if the URL is
// invalid or uses an unsupported scheme.
func NewRedisTrustBackendFromConfig(cfg KernelTrustConfig) (*RedisTrustBackend, error) {
	cfg = cfg.withDefaults()
	store, err := NewRESPRedisClient(cfg.RedisURL, cfg.Timeout)
	if err != nil {
		return nil, err
	}
	return NewRedisTrustBackend(store, cfg), nil
}

// CheckAgentState implements TrustChecker. On a store error it fails closed
// (TrustStateIsolated) when the backend is configured fail-closed, and
// otherwise degrades open to TrustStateTrusted. An agent with no stored record
// is reported TrustStateTrusted (the absent-record case, distinct from a store
// fault).
func (b *RedisTrustBackend) CheckAgentState(ctx context.Context, agentID string) (TrustState, error) {
	snapshot, err := b.GetAgentTrust(ctx, agentID)
	if err != nil {
		if b.failClosed {
			return TrustStateIsolated, err
		}
		return TrustStateTrusted, nil
	}
	return snapshot.State, nil
}

// GetAgentTrust implements TrustBackend. It reads the agent's circuit_state and
// trust_score keys; an absent state key yields a default trusted snapshot
// (Known == false). A store or parse error is returned to the caller. An empty
// agentID returns an error.
func (b *RedisTrustBackend) GetAgentTrust(ctx context.Context, agentID string) (TrustSnapshot, error) {
	if agentID == "" {
		return TrustSnapshot{}, fmt.Errorf("agent_id is required")
	}
	stateValue, err := b.store.Get(ctx, b.key(agentID, "circuit_state"))
	if err != nil {
		return TrustSnapshot{}, err
	}
	if stateValue == "" {
		return TrustSnapshot{AgentID: agentID, State: TrustStateTrusted, Score: 1, Known: false}, nil
	}
	state, err := parseTrustState(stateValue)
	if err != nil {
		return TrustSnapshot{}, err
	}
	score := trustScoreForState(state)
	if scoreValue, err := b.store.Get(ctx, b.key(agentID, "trust_score")); err == nil && scoreValue != "" {
		if parsed, parseErr := strconv.ParseFloat(scoreValue, 64); parseErr == nil {
			score = parsed
		}
	}
	return TrustSnapshot{AgentID: agentID, State: state, Score: score, Known: true}, nil
}

// RecordDecision implements TrustBackend. It reads the agent's current
// snapshot, applies a coarse transition (a failure on a TRUSTED agent moves it
// to EVALUATING with score 0.5), writes the result back to Redis, and returns
// the before/after update. A nil request or empty AgentID returns an error.
func (b *RedisTrustBackend) RecordDecision(ctx context.Context, req *GovernanceRequest, decision *GovernanceDecision) (TrustDecisionUpdate, error) {
	if req == nil || req.AgentID == "" {
		return TrustDecisionUpdate{}, fmt.Errorf("agent_id is required")
	}
	before, err := b.GetAgentTrust(ctx, req.AgentID)
	if err != nil {
		return TrustDecisionUpdate{}, err
	}
	after := before
	outcome := TrustOutcomeFromDecision(decision)
	if outcome == TrustOutcomeFailure && before.State == TrustStateTrusted {
		after.State = TrustStateEvaluating
		after.Score = 0.5
	}
	if err := b.writeSnapshot(ctx, after); err != nil {
		return TrustDecisionUpdate{}, err
	}
	return TrustDecisionUpdate{
		Before:     before,
		After:      after,
		Outcome:    outcome,
		Transition: before.State != after.State,
	}, nil
}

// ResetAgentTrust implements TrustBackend by deleting the agent's circuit_state
// and trust_score keys, returning it to the default trusted snapshot. An empty
// agentID returns an error.
func (b *RedisTrustBackend) ResetAgentTrust(ctx context.Context, agentID string) (TrustSnapshot, error) {
	if agentID == "" {
		return TrustSnapshot{}, fmt.Errorf("agent_id is required")
	}
	if err := b.store.Del(ctx, b.key(agentID, "circuit_state")); err != nil {
		return TrustSnapshot{}, err
	}
	_ = b.store.Del(ctx, b.key(agentID, "trust_score"))
	return TrustSnapshot{AgentID: agentID, State: TrustStateTrusted, Score: 1, Known: false}, nil
}

// TerminateAgent implements TrustBackend by writing the TERMINATED state to
// Redis, blocking all further execution for the agent until ResetAgentTrust is
// called. A store error is returned to the caller.
func (b *RedisTrustBackend) TerminateAgent(ctx context.Context, agentID string) (TrustSnapshot, error) {
	snapshot := TrustSnapshot{AgentID: agentID, State: TrustStateTerminated, Score: 0, Known: true}
	if err := b.writeSnapshot(ctx, snapshot); err != nil {
		return TrustSnapshot{}, err
	}
	return snapshot, nil
}

func (b *RedisTrustBackend) writeSnapshot(ctx context.Context, snapshot TrustSnapshot) error {
	if snapshot.AgentID == "" {
		return fmt.Errorf("agent_id is required")
	}
	if err := b.store.Set(ctx, b.key(snapshot.AgentID, "circuit_state"), strconv.Itoa(int(snapshot.State)), 24*time.Hour); err != nil {
		return err
	}
	return b.store.Set(ctx, b.key(snapshot.AgentID, "trust_score"), strconv.FormatFloat(snapshot.Score, 'f', 6, 64), 24*time.Hour)
}

func (b *RedisTrustBackend) key(agentID, suffix string) string {
	prefix := b.prefix
	if prefix == "" {
		prefix = "agent:"
	}
	return prefix + agentID + ":" + suffix
}

func parseTrustState(value string) (TrustState, error) {
	value = strings.TrimSpace(strings.ToUpper(value))
	switch value {
	case "0", "TRUSTED", "CLOSED":
		return TrustStateTrusted, nil
	case "1", "EVALUATING", "HALF_OPEN", "DEGRADED":
		return TrustStateEvaluating, nil
	case "2", "ISOLATED", "OPEN":
		return TrustStateIsolated, nil
	case "3", "TERMINATED":
		return TrustStateTerminated, nil
	default:
		return TrustStateIsolated, fmt.Errorf("unknown trust state %q", value)
	}
}

func trustScoreForState(state TrustState) float64 {
	switch state {
	case TrustStateTrusted:
		return 1
	case TrustStateEvaluating:
		return 0.5
	default:
		return 0
	}
}

// RESPRedisClient is a dependency-free Redis client implementing RedisKV over
// the RESP wire protocol. It opens a fresh, deadline-bounded TCP connection per
// command (no pooling) and supports only the GET/SET/DEL verbs the trust
// backend needs. It is intended for the low-frequency trust IPC path, not as a
// general-purpose Redis client.
type RESPRedisClient struct {
	addr    string
	timeout time.Duration
}

// NewRESPRedisClient parses a redis://host[:port] URL (defaulting the port to
// 6379 and the per-command timeout to 100ms when zero) and returns a client.
// It returns an error for a malformed URL or a non-"redis" scheme.
func NewRESPRedisClient(rawURL string, timeout time.Duration) (*RESPRedisClient, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "redis" {
		return nil, fmt.Errorf("unsupported redis URL scheme %q", parsed.Scheme)
	}
	addr := parsed.Host
	if !strings.Contains(addr, ":") {
		addr += ":6379"
	}
	if timeout == 0 {
		timeout = 100 * time.Millisecond
	}
	return &RESPRedisClient{addr: addr, timeout: timeout}, nil
}

// Get implements RedisKV. It issues GET key and returns "" for a missing key.
func (c *RESPRedisClient) Get(ctx context.Context, key string) (string, error) {
	return c.command(ctx, "GET", key)
}

// Set implements RedisKV. It issues SET key value EX <ttl-seconds>.
func (c *RESPRedisClient) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	_, err := c.command(ctx, "SET", key, value, "EX", strconv.Itoa(int(ttl.Seconds())))
	return err
}

// Del implements RedisKV. It issues DEL key.
func (c *RESPRedisClient) Del(ctx context.Context, key string) error {
	_, err := c.command(ctx, "DEL", key)
	return err
}

func (c *RESPRedisClient) command(ctx context.Context, args ...string) (string, error) {
	dialer := net.Dialer{Timeout: c.timeout}
	conn, err := dialer.DialContext(ctx, "tcp", c.addr)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	deadline := time.Now().Add(c.timeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	_ = conn.SetDeadline(deadline)
	if _, err := conn.Write([]byte(encodeRESP(args...))); err != nil {
		return "", err
	}
	reader := bufio.NewReader(conn)
	return readRESP(reader)
}

func encodeRESP(args ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "*%d\r\n", len(args))
	for _, arg := range args {
		fmt.Fprintf(&b, "$%d\r\n%s\r\n", len(arg), arg)
	}
	return b.String()
}

func readRESP(reader *bufio.Reader) (string, error) {
	prefix, err := reader.ReadByte()
	if err != nil {
		return "", err
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
	switch prefix {
	case '+':
		return line, nil
	case ':':
		return line, nil
	case '-':
		return "", fmt.Errorf("redis error: %s", line)
	case '$':
		n, err := strconv.Atoi(line)
		if err != nil {
			return "", err
		}
		if n < 0 {
			return "", nil
		}
		buf := make([]byte, n+2)
		if _, err := reader.Read(buf); err != nil {
			return "", err
		}
		return string(buf[:n]), nil
	default:
		return "", fmt.Errorf("unsupported redis response prefix %q", prefix)
	}
}
