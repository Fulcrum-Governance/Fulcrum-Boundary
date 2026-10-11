package madriver

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/fulcrum-governance/fulcrum-boundary/adapters/managedagents"
)

// Model list prices, USD per million tokens, at public list rates.
//
// WARNING: these prices MUST be re-checked against Anthropic's current pricing
// page immediately before any live run. A model absent from this table is
// treated as unknown spend and stops the session (fail closed) rather than
// estimating. Managed Agents sessions also accrue session running time and
// web-search charges per the budgets doc; those only reach the guard through
// the session.usage list_cost signal, which is why the guard prefers that
// figure when upstream emits it.
var priceTable = map[string]struct {
	InputPerMTok  float64
	OutputPerMTok float64
}{
	"claude-sonnet-4-5-20250929": {InputPerMTok: 3.00, OutputPerMTok: 15.00},
	"claude-haiku-4-5-20251001":  {InputPerMTok: 1.00, OutputPerMTok: 5.00},
	"claude-opus-4-5-20251101":   {InputPerMTok: 5.00, OutputPerMTok: 25.00},
}

var (
	// ErrSpendUnknown is returned when an event that must carry a spend signal
	// arrives without parseable usage fields, or when no usage evidence has
	// arrived within the configured blind-event/blind-window limits. Live mode
	// treats unknown spend as fail-closed and stops the session.
	ErrSpendUnknown = errors.New("spend unknown: usage fields missing or unparseable")

	// ErrSpendAbort is returned when observed spend reaches 80 percent of the
	// configured ceiling; the driver aborts and writes the transcript so far.
	ErrSpendAbort = errors.New("spend reached 80 percent of the configured ceiling; aborting")

	// ErrMaxTurns is returned when the session resolves more governable tool
	// events than --max-turns allows.
	ErrMaxTurns = errors.New("maximum governed turns reached; aborting")

	// ErrOutputTokenCap is returned when an observed model request reports
	// output tokens over --max-output-tokens. No per-request
	// max_output_tokens field is documented on the sessions API (UNVERIFIED
	// whether upstream supports one), so the driver cannot preempt a
	// generation; it enforces the cap by terminating the session as soon as
	// an over-cap request is reported.
	ErrOutputTokenCap = errors.New("observed request output exceeded --max-output-tokens; aborting session")
)

// spendGuard tracks observed session spend and decides when to stop the
// stream. Two spend signals are understood:
//
//   - session.usage events carrying the session's cumulative list cost
//     (whole US cents as a string under data.usage.list_cost or data.list_cost;
//     authoritative when present because it already includes runtime and
//     tool-use charges);
//   - span.model_request_end events carrying model_usage token counts, priced
//     through priceTable (fallback when no session.usage has been seen).
//
// An event of either type that lacks the expected usage payload is unknown
// spend: the guard reports ErrSpendUnknown and the driver stops the session.
type spendGuard struct {
	ceilingUSD      float64
	abortAtUSD      float64
	maxOutputTokens int
	listCostUSD     float64
	sawListCost     bool
	tokenCostUSD    float64
	// postSnapshotUSD accumulates priced spend observed after the most
	// recent list_cost snapshot so used() never under-reports the spend
	// accrued between snapshots.
	postSnapshotUSD float64
	// usageEvidence records that at least one usable spend signal arrived; a
	// transcript budget of Used=0 without it must never satisfy the budget
	// criterion.
	usageEvidence bool
	// eventsBlind counts consecutive events carrying no usable spend signal.
	// lastEvidence is the wall-clock time of the most recent signal.
	eventsBlind  int
	lastEvidence time.Time
	startedAt    time.Time
	blindEvents  int
	blindWindow  time.Duration
	now          func() time.Time
}

func newSpendGuard(ceilingUSD float64, maxOutputTokens, blindEvents int, blindWindow time.Duration) *spendGuard {
	return &spendGuard{
		ceilingUSD:      ceilingUSD,
		abortAtUSD:      ceilingUSD * 0.8,
		maxOutputTokens: maxOutputTokens,
		blindEvents:     blindEvents,
		blindWindow:     blindWindow,
		now:             time.Now,
	}
}

// used returns the best current spend estimate in USD: once a list_cost
// snapshot has been seen it is the snapshot plus the priced spend observed
// since it, never the snapshot alone.
func (g *spendGuard) used() float64 {
	if g.sawListCost {
		return g.listCostUSD + g.postSnapshotUSD
	}
	return g.tokenCostUSD
}

// observedUsage reports whether any usable spend signal has arrived. It backs
// the transcript's budget.usage_observed evidence flag.
func (g *spendGuard) observedUsage() bool { return g.usageEvidence }

// addPriced folds one token-priced spend signal into the estimate. Before the
// first list_cost snapshot it builds the token-only estimate; after one it
// accrues separately so the snapshot figure and the post-snapshot spend are
// never conflated or dropped.
func (g *spendGuard) addPriced(cost float64) {
	if g.sawListCost {
		g.postSnapshotUSD += cost
		return
	}
	g.tokenCostUSD += cost
}

// arm records when the stream started being read so a stream that produces
// no events at all is still bounded by the blind window. observe arms it on
// the first event; guardedSource arms it before the first read.
func (g *spendGuard) arm() {
	if g.startedAt.IsZero() {
		g.startedAt = g.now()
	}
}

// blindDeadline is the instant the stream becomes usage-blind: the most
// recent usable usage signal plus the configured window (stream start when
// no signal has arrived yet). A zero time means the window is not armed.
func (g *spendGuard) blindDeadline() time.Time {
	ref := g.lastEvidence
	if ref.IsZero() {
		ref = g.startedAt
	}
	if ref.IsZero() {
		return time.Time{}
	}
	return ref.Add(g.blindWindow)
}

// blindExceeded reports whether the blind window has passed per the guard's
// clock. now is injectable so tests can move time without real sleeps.
func (g *spendGuard) blindExceeded() bool {
	deadline := g.blindDeadline()
	return !deadline.IsZero() && !g.now().Before(deadline)
}

// observe folds one upstream event into the spend estimate.
func (g *spendGuard) observe(event managedagents.Event) error {
	now := g.now()
	if g.startedAt.IsZero() {
		g.startedAt = now
	}
	evidence := false
	switch event.Type {
	case "session.usage":
		cents, ok := listCostCents(event.Data)
		if !ok {
			return fmt.Errorf("%w: session.usage event carried no list_cost", ErrSpendUnknown)
		}
		g.sawListCost = true
		// list_cost is cumulative and REPLACES the prior figure rather than
		// adding to it; a later snapshot that reports less must not lower the
		// spend estimate, so the guard keeps the maximum observed value. A
		// snapshot that sets a new maximum already covers every priced event
		// observed so far, so the post-snapshot accrual restarts from zero.
		if usd := float64(cents) / 100; usd > g.listCostUSD {
			g.listCostUSD = usd
			g.postSnapshotUSD = 0
		}
		evidence = true
	case "span.model_request_end":
		cost, outputTokens, err := modelRequestCost(event.Data)
		if err != nil {
			return err
		}
		g.addPriced(cost)
		evidence = true
		if outputTokens > 0 && outputTokens > g.maxOutputTokens {
			return ErrOutputTokenCap
		}
	}
	if event.Usage != nil && event.Usage.CostUSD > 0 {
		g.addPriced(event.Usage.CostUSD)
		evidence = true
	}
	if evidence {
		g.usageEvidence = true
		g.eventsBlind = 0
		g.lastEvidence = now
	} else {
		// Fail closed when the stream runs blind: no usable usage signal
		// after --usage-blind-events events or --usage-blind-window seconds
		// means spend is unknown and the session must stop.
		g.eventsBlind++
		ref := g.lastEvidence
		if ref.IsZero() {
			ref = g.startedAt
		}
		if g.eventsBlind >= g.blindEvents || now.Sub(ref) >= g.blindWindow {
			return fmt.Errorf("%w: no usage signal after %d events / %s", ErrSpendUnknown, g.eventsBlind, now.Sub(ref).Round(time.Second))
		}
	}
	if g.used() >= g.abortAtUSD {
		return ErrSpendAbort
	}
	return nil
}

// listCostCents extracts the cumulative list cost (a whole-cents string per
// the budgets doc) from a session.usage event's payload. Both the documented
// nested shape (usage.list_cost) and a flat list_cost are accepted.
func listCostCents(data map[string]any) (int64, bool) {
	if data == nil {
		return 0, false
	}
	if usage, ok := data["usage"].(map[string]any); ok {
		if cents, ok := parseCents(usage["list_cost"]); ok {
			return cents, true
		}
	}
	return parseCents(data["list_cost"])
}

func parseCents(v any) (int64, bool) {
	switch value := v.(type) {
	case string:
		cents, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err != nil {
			return 0, false
		}
		return cents, true
	case float64:
		return int64(value), true
	case json.Number:
		cents, err := value.Int64()
		if err != nil {
			return 0, false
		}
		return cents, true
	default:
		return 0, false
	}
}

// modelRequestCost prices one span.model_request_end event. The expected shape
// is data.model_usage{model, input_tokens, output_tokens}; field names are the
// best reading of the published reference and are UNVERIFIED against a live
// capture — a missing or unparseable payload is unknown spend and fails closed.
func modelRequestCost(data map[string]any) (costUSD float64, outputTokens int, err error) {
	if data == nil {
		return 0, 0, fmt.Errorf("%w: span.model_request_end carried no data", ErrSpendUnknown)
	}
	usage, ok := data["model_usage"].(map[string]any)
	if !ok {
		// Some captures may nest usage one level deeper.
		inner, ok2 := data["usage"].(map[string]any)
		if !ok2 {
			return 0, 0, fmt.Errorf("%w: span.model_request_end missing model_usage", ErrSpendUnknown)
		}
		usage = inner
	}
	model, _ := usage["model"].(string)
	input := numberField(usage, "input_tokens")
	output := numberField(usage, "output_tokens")
	if input == 0 && output == 0 {
		return 0, 0, fmt.Errorf("%w: span.model_request_end carried no token counts", ErrSpendUnknown)
	}
	price, ok := priceTable[model]
	if !ok {
		return 0, output, fmt.Errorf("%w: model %q not in driver price table", ErrSpendUnknown, model)
	}
	return (float64(input)*price.InputPerMTok + float64(output)*price.OutputPerMTok) / 1_000_000, output, nil
}

func numberField(m map[string]any, key string) int {
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(v))
		return n
	default:
		return 0
	}
}
