package commandboundary

import (
	"errors"
	"fmt"
	"strings"
)

// SchemaVersionLineClassification identifies the payload ClassifyLine returns.
const SchemaVersionLineClassification = "boundary.command_line_classification.v1"

// MaxLineDepth caps how far ClassifyLine follows nested shell payloads — command
// substitution, subshells, and `sh -c` strings. A line that nests deeper than
// this cap is reported with Parseable false instead of being decomposed further.
const MaxLineDepth = 3

// ReasonUndecomposable is the fixed reason ClassifyLine reports when the
// tokenizer cannot confidently decompose a line. A line carrying this reason is
// never allowed: the aggregate recommended action floor is require_approval.
const ReasonUndecomposable = "compound command could not be safely decomposed"

// Segment origins recorded on every SegmentClassification.
const (
	// SegmentOriginCommand marks a simple command written directly on the line.
	SegmentOriginCommand = "command"
	// SegmentOriginSubstitution marks a command found inside `$( … )` or backticks.
	SegmentOriginSubstitution = "command_substitution"
	// SegmentOriginSubshell marks a command found inside a `( … )` subshell.
	SegmentOriginSubshell = "subshell"
	// SegmentOriginShellPayload marks a command found inside an `sh -c` string.
	SegmentOriginShellPayload = "shell_payload"
	// SegmentOriginArgumentCommand marks a command found in ARGUMENT position —
	// the program `find -exec` hands to the kernel, for example — rather than in
	// command position where the shell itself would run it.
	SegmentOriginArgumentCommand = "argument_command"
	// SegmentOriginRedirect marks the file an output redirection writes. It is
	// not a command: the shell performs the write, whatever the command in front
	// of the operator does.
	SegmentOriginRedirect = "redirect"
)

// redirectSegmentCommand stands in for the command name of a redirect segment,
// which has no command of its own: the shell performs the write.
const redirectSegmentCommand = ">"

// maxPrefixUnwrap bounds how many wrapper commands (`sudo`, `env`, `exec`, …)
// ClassifyLine unwraps on one simple command before giving up on the line.
const maxPrefixUnwrap = 4

// SegmentClassification is the classification of one simple command decomposed
// from a compound shell line. Segment text is redacted the same way
// Classification.ArgsRedacted is: secret-looking values never appear.
type SegmentClassification struct {
	// Segment is the redacted text of this simple command.
	Segment string `json:"segment"`
	// Origin records how the segment was reached: one of the SegmentOrigin
	// constants.
	Origin string `json:"origin"`
	// Depth is the nesting depth of the segment; 0 for a command written
	// directly on the line, 1 for one inside a substitution, and so on.
	Depth int `json:"depth"`
	// Command is the segment's leading command name.
	Command string `json:"command"`
	// ArgsRedacted holds the segment arguments with secret-looking values replaced.
	ArgsRedacted []string `json:"args_redacted,omitempty"`
	// Class is the Command Boundary taxonomy class for this segment.
	Class Class `json:"class"`
	// Risk is the posture risk for Class.
	Risk Risk `json:"risk"`
	// RecommendedAction is the posture action for Class.
	RecommendedAction RecommendedAction `json:"recommended_action"`
	// Reason states why the segment landed in Class.
	Reason string `json:"reason"`
}

// ClassLabel renders the segment class with its human-readable meaning.
func (s SegmentClassification) ClassLabel() string {
	return string(s.Class) + " " + classMeaning(s.Class)
}

// LineClassification is the result of decomposing one compound shell line into
// simple commands and classifying each of them.
//
// Aggregate is the verdict callers enforce: the most restrictive segment by
// recommended-action severity (deny > require_approval > warn > allow). When
// Parseable is false the aggregate additionally carries a require_approval
// floor, so an undecomposable line is never reported as allow.
type LineClassification struct {
	// SchemaVersion is always SchemaVersionLineClassification.
	SchemaVersion string `json:"schema_version"`
	// Parseable reports whether the tokenizer decomposed the whole line with
	// confidence. False means at least one construct was not modeled.
	Parseable bool `json:"parseable"`
	// Segments lists every simple command the tokenizer did decompose, in
	// discovery order. It may be partial when Parseable is false.
	Segments []SegmentClassification `json:"segments"`
	// Aggregate is the most restrictive verdict over Segments, floored at
	// require_approval when Parseable is false.
	Aggregate Classification `json:"aggregate"`
}

// AggregateClassLabel renders the aggregate class with its human-readable meaning.
func (l LineClassification) AggregateClassLabel() string {
	return l.Aggregate.ClassLabel()
}

// RedactedCommandLine returns the redacted text of the segment that produced the
// aggregate verdict, or a fixed placeholder when no segment was decomposed.
func (l LineClassification) RedactedCommandLine() string {
	if l.Aggregate.Command == "" {
		return "(no decomposed command segment)"
	}
	return l.Aggregate.RedactedCommandLine()
}

// ClassifyLine classifies a compound shell line without executing any part of it.
//
// The line is split into simple commands on the operators `&&`, `||`, `;`, `|`,
// `&`, and newline outside quotes; command substitution (`$( … )` and backticks),
// subshells, and `sh -c` payloads are decomposed recursively up to MaxLineDepth.
// Each resulting simple command is classified with Classify. An output
// redirection (`>`, `>>`, `>|`, `&>`, `&>>`) contributes its own segment for the
// file the shell writes, so a redirect is never laundered into the argv of the
// command in front of it. A command run from ARGUMENT position — `find -exec`,
// `xargs` — is decomposed and classified too, rather than left inert inside the
// wrapper's arguments.
//
// Any construct the tokenizer does not model — unbalanced quotes, heredocs,
// process substitution, `eval`, backslash-newline continuations, arithmetic or
// parameter expansion containing a substitution, or nesting past MaxLineDepth —
// sets Parseable false and floors Aggregate.RecommendedAction at
// require_approval. A segment that was decomposed and classifies stricter still
// wins, so an undecomposed remainder can raise the verdict but never lower it;
// either way Aggregate.Reason names ReasonUndecomposable. ClassifyLine never
// returns allow for a line it could not decompose, and never executes anything.
//
// Parseable true is a claim about the CONSTRUCTS above, not a claim that every
// program the line can reach was classified: an argument-position shape outside
// argumentCommands (`watch`, `parallel`, `make`, an interpreter's inline program)
// leaves its payload unclassified with Parseable still true. See
// docs/command-boundary/CLASSIFY.md.
//
// An empty or whitespace-only line is an error.
func ClassifyLine(line string) (LineClassification, error) {
	if strings.TrimSpace(line) == "" {
		return LineClassification{}, errors.New("command line is required")
	}

	decomposer := &lineDecomposer{parseable: true}
	decomposer.decompose(line, 0, SegmentOriginCommand)

	segments := decomposer.segments
	if segments == nil {
		segments = []SegmentClassification{}
	}
	return LineClassification{
		SchemaVersion: SchemaVersionLineClassification,
		Parseable:     decomposer.parseable,
		Segments:      segments,
		Aggregate:     aggregateOf(segments, decomposer.parseable),
	}, nil
}

// ContainsShellOperators reports whether a single argument reads as a shell line
// rather than a bare command word. Callers use it to decide between Classify
// (argv already split by the caller) and ClassifyLine (one unsplit shell line).
func ContainsShellOperators(arg string) bool {
	return strings.ContainsAny(arg, "&|;<>()`\n")
}

// actionSeverity orders recommended actions from least to most restrictive.
// Unrecognized actions sort at require_approval so an unknown value is never
// treated as permissive.
func actionSeverity(action RecommendedAction) int {
	switch action {
	case ActionDeny:
		return 4
	case ActionRequireApproval:
		return 3
	case ActionWarn:
		return 2
	case ActionAllow:
		return 1
	default:
		return 3
	}
}

// aggregateOf reduces the decomposed segments to the single most restrictive
// classification, applying the undecomposable require_approval floor when the
// line was not fully parseable.
func aggregateOf(segments []SegmentClassification, parseable bool) Classification {
	aggregate := Classification{SchemaVersion: SchemaVersionClassification}
	best := -1
	bestSeverity := 0
	if !parseable {
		bestSeverity = actionSeverity(ActionRequireApproval)
	}
	for i, segment := range segments {
		severity := actionSeverity(segment.RecommendedAction)
		if severity > bestSeverity {
			bestSeverity = severity
			best = i
		}
	}

	if best < 0 {
		// Either the line was not decomposable, or it carried no executable
		// command. Both resolve to the conservative review posture; C7 is the
		// taxonomy's existing "unclassified, requires review" bucket.
		aggregate.Class = ClassPackageLifecycle
		aggregate.Risk, aggregate.RecommendedAction = postureFor(ClassPackageLifecycle)
		aggregate.Reason = ReasonUndecomposable
		if parseable {
			aggregate.Reason = "no executable command segment was decomposed from the line"
		}
		return aggregate
	}

	segment := segments[best]
	aggregate.Command = segment.Command
	aggregate.ArgsRedacted = segment.ArgsRedacted
	aggregate.Class = segment.Class
	aggregate.Risk = segment.Risk
	aggregate.RecommendedAction = segment.RecommendedAction
	aggregate.Reason = aggregateReason(segment, len(segments), parseable)
	return aggregate
}

func aggregateReason(segment SegmentClassification, total int, parseable bool) string {
	reason := segment.Reason
	if total > 1 {
		reason = fmt.Sprintf("%s (most restrictive of %d compound segments: %s)", reason, total, segment.Segment)
	}
	if !parseable {
		reason += "; " + ReasonUndecomposable
	}
	return reason
}

// lineDecomposer accumulates the segments found in one line and tracks whether
// every construct encountered was modeled.
type lineDecomposer struct {
	segments  []SegmentClassification
	parseable bool
}

func (d *lineDecomposer) decompose(line string, depth int, origin string) {
	if depth > MaxLineDepth {
		d.parseable = false
		return
	}
	tokens, ok := scanLine(line)
	if !ok {
		// Keep whatever was tokenized before the unmodeled construct: extra
		// segments can only raise the aggregate, never lower it.
		d.parseable = false
	}
	for _, command := range splitSimpleCommands(tokens) {
		d.decomposeSimple(command, depth, origin)
	}
}

func (d *lineDecomposer) decomposeSimple(command simpleCommand, depth int, origin string) {
	for _, substitution := range command.subs {
		d.decompose(substitution, depth+1, SegmentOriginSubstitution)
	}
	for _, subshell := range command.subshells {
		d.decompose(subshell, depth+1, SegmentOriginSubshell)
	}
	if argv := commandWords(command.words); len(argv) > 0 {
		d.classifyArgv(argv, depth, origin, command.piped)
	}
	// The writes come last but are not conditional on there being a command: a
	// bare `> important.db` truncates the file with no command at all.
	for _, target := range command.writes {
		d.appendRedirectSegment(target, depth)
	}
}

func (d *lineDecomposer) classifyArgv(argv []string, depth int, origin string, piped bool) {
	for i := 0; i < maxPrefixUnwrap; i++ {
		if undecomposableCommands[strings.ToLower(argv[0])] {
			d.parseable = false
		}
		if payload, ok := shellCommandPayload(argv); ok {
			d.decompose(payload, depth+1, SegmentOriginShellPayload)
		}
		for _, embedded := range argumentCommands(argv) {
			d.decomposeArgv(embedded, depth+1, SegmentOriginArgumentCommand)
		}
		d.appendSegment(argv, depth, origin, piped)

		inner, ok := prefixInnerArgv(argv)
		if !ok {
			return
		}
		argv = inner
	}
	// More wrapper commands than the unwrap budget: stop, but never allow.
	d.parseable = false
}

// decomposeArgv classifies an already-split argv found nested inside another
// command, applying the same depth cap decompose applies to nested shell text.
func (d *lineDecomposer) decomposeArgv(argv []string, depth int, origin string) {
	if depth > MaxLineDepth {
		d.parseable = false
		return
	}
	if len(argv) == 0 {
		return
	}
	d.classifyArgv(argv, depth, origin, false)
}

// appendRedirectSegment classifies an output redirection as the file write it is.
//
// The shell writes the target of `>`, `>>`, `>|`, `&>`, or `&>>` — truncating or
// appending — whatever the command in front of the operator does. Leaving the
// target in that command's argv laundered the write into the command's own class,
// so `cat notes > important.db` read as a file read; it gets its own segment
// instead, and the line's aggregate carries the write.
//
// The target classifies as a local file write (C1), or as credential access (C6)
// when it names a secret-bearing path. Nothing here stats the target, so whether
// the write truncates existing content, creates a new file, or fails is not
// knowable on this path and is not asserted.
func (d *lineDecomposer) appendRedirectSegment(target string, depth int) {
	class := ClassLocalFileWrite
	reason := "shell redirection writes this file"
	if isSensitiveArg(target) {
		class = ClassCredentialAccess
		reason = "shell redirection writes a secret-bearing path"
	}
	risk, action := postureFor(class)
	redacted := RedactArgs([]string{target})
	d.segments = append(d.segments, SegmentClassification{
		Segment:           redirectSegmentCommand + " " + strings.Join(redacted, " "),
		Origin:            SegmentOriginRedirect,
		Depth:             depth,
		Command:           redirectSegmentCommand,
		ArgsRedacted:      redacted,
		Class:             class,
		Risk:              risk,
		RecommendedAction: action,
		Reason:            reason,
	})
}

func (d *lineDecomposer) appendSegment(argv []string, depth int, origin string, piped bool) {
	classification, err := Classify(argv)
	if err != nil {
		d.parseable = false
		return
	}
	// A shell or interpreter on the receiving end of a pipe runs a program the
	// classifier never sees. Only the reason is refined; the class (and so the
	// posture) is left exactly where Classify put it.
	if piped && classification.Class == ClassPackageLifecycle && isInterpreter(classification.Command) && !hasScriptArgument(argv[1:]) {
		classification.Reason = "shell interpreter executes piped input"
	}
	d.segments = append(d.segments, SegmentClassification{
		Segment:           classification.RedactedCommandLine(),
		Origin:            origin,
		Depth:             depth,
		Command:           classification.Command,
		ArgsRedacted:      classification.ArgsRedacted,
		Class:             classification.Class,
		Risk:              classification.Risk,
		RecommendedAction: classification.RecommendedAction,
		Reason:            classification.Reason,
	})
}

// simpleCommand is one command position in a decomposed line: its words, the
// files its output redirections write, plus the nested payloads that run as part
// of evaluating it.
type simpleCommand struct {
	words     []string
	writes    []string
	subs      []string
	subshells []string
	piped     bool
}

// splitSimpleCommands cuts the token stream at every separator, dropping empty
// command positions (`a ;; b`, a trailing `&`).
func splitSimpleCommands(tokens []shellToken) []simpleCommand {
	commands := make([]simpleCommand, 0, 4)
	current := simpleCommand{}
	pipedNext := false

	flush := func() {
		if len(current.words) > 0 || len(current.writes) > 0 ||
			len(current.subs) > 0 || len(current.subshells) > 0 {
			commands = append(commands, current)
		}
		current = simpleCommand{piped: pipedNext}
	}

	for _, token := range tokens {
		switch token.kind {
		case tokenWord:
			if token.value != "" {
				current.words = append(current.words, token.value)
			}
			current.subs = append(current.subs, token.subs...)
		case tokenRedirectTarget:
			if token.value != "" {
				current.writes = append(current.writes, token.value)
			}
			// A substitution inside a redirect target still runs.
			current.subs = append(current.subs, token.subs...)
		case tokenSubshell:
			current.subshells = append(current.subshells, token.value)
		case tokenSeparator:
			pipedNext = token.value == "|"
			flush()
		}
	}
	pipedNext = false
	flush()
	return commands
}

// argumentCommands returns the commands argv runs from ARGUMENT position: a
// program named inside another command's arguments, which the shell never sees
// as a command word and the line decomposer would otherwise leave unclassified
// while still reporting the line fully decomposed.
//
// One shape is modeled here — `find`'s `-exec`, `-execdir`, `-ok`, and `-okdir`,
// whose payload runs to the terminating `;` or `+` argument. `xargs` is handled
// as a wrapper instead (see prefixCommands), because its payload is simply the
// argv that follows its flags.
//
// It is NOT an inventory of every command that can run another: `watch`,
// `parallel`, `make`, `ssh`, and an interpreter's inline `-e`/`-c` program are
// not modeled, and their payloads stay unclassified.
func argumentCommands(argv []string) [][]string {
	if len(argv) == 0 || strings.ToLower(argv[0]) != "find" {
		return nil
	}
	return findExecCommands(argv[1:])
}

// findExecCommands extracts the payload of every `find` exec-family primary in
// args: the words after the flag up to the `;` or `+` that terminates it.
//
// A payload with no terminator — the shell consumed an unescaped `;` as a
// separator, or the line was truncated — runs to the end of args rather than
// being dropped, because classifying too much of a `find -exec` payload can only
// raise the verdict.
func findExecCommands(args []string) [][]string {
	var commands [][]string
	for i := 0; i < len(args); i++ {
		if !findExecFlags[strings.ToLower(args[i])] {
			continue
		}
		embedded := make([]string, 0, len(args)-i)
		for i++; i < len(args); i++ {
			if args[i] == ";" || args[i] == "+" {
				break
			}
			embedded = append(embedded, args[i])
		}
		if len(embedded) > 0 {
			commands = append(commands, embedded)
		}
	}
	return commands
}

// findExecFlags are the `find` primaries that run a command per match.
var findExecFlags = map[string]bool{
	"-exec": true, "-execdir": true, "-ok": true, "-okdir": true,
}

// commandWords drops the leading variable assignments and shell keywords that
// precede the real command word, so `FOO=bar rm -rf .` and `if rm -rf .` both
// surface `rm` as the command being classified.
func commandWords(words []string) []string {
	argv := words
	for len(argv) > 0 && (isAssignmentWord(argv[0]) || structuralWords[strings.ToLower(argv[0])]) {
		argv = argv[1:]
	}
	return argv
}

// isAssignmentWord reports whether a word has the `NAME=VALUE` shape. Quoting is
// deliberately ignored: treating a quoted lookalike as an assignment only makes
// the classifier look further down the command line, which is the safe direction.
func isAssignmentWord(word string) bool {
	name, _, found := strings.Cut(word, "=")
	if !found || name == "" {
		return false
	}
	for i, r := range name {
		switch {
		case r == '_', r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}

// shellCommandPayload returns the program string a shell was asked to run with a
// `-c`-style flag, so the payload can be decomposed instead of trusted.
func shellCommandPayload(argv []string) (payload string, ok bool) {
	if len(argv) < 3 || !shellInterpreters[strings.ToLower(argv[0])] {
		return "", false
	}
	for i := 1; i < len(argv)-1; i++ {
		arg := argv[i]
		if arg == "--" {
			return "", false
		}
		if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") && strings.Contains(arg, "c") {
			return argv[i+1], true
		}
	}
	return "", false
}

// prefixInnerArgv peels a wrapper command (`sudo`, `env`, `exec`, …) off argv and
// returns the command it wraps. The wrapper itself is still classified, so
// peeling can only add a segment, never replace a stricter one.
func prefixInnerArgv(argv []string) (inner []string, ok bool) {
	if len(argv) < 2 {
		return nil, false
	}
	name := strings.ToLower(argv[0])
	if !prefixCommands[name] {
		return nil, false
	}
	valueFlags := prefixValueFlags[name]
	rest := argv[1:]
	for len(rest) > 0 {
		arg := rest[0]
		switch {
		case isAssignmentWord(arg), arg == "--":
			rest = rest[1:]
		case len(arg) > 1 && strings.HasPrefix(arg, "-"):
			if valueFlags[arg] && len(rest) > 1 {
				rest = rest[2:]
				continue
			}
			rest = rest[1:]
		default:
			return rest, true
		}
	}
	return nil, false
}

func isInterpreter(command string) bool {
	name := strings.ToLower(command)
	return shellInterpreters[name] || scriptInterpreters[name]
}

func hasScriptArgument(args []string) bool {
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			return true
		}
	}
	return false
}

// structuralWords are shell keywords that occupy a command position without
// being commands. Skipping them exposes the command they guard.
var structuralWords = map[string]bool{
	"!": true, "{": true, "}": true, "[[": true, "]]": true,
	"case": true, "coproc": true, "do": true, "done": true, "elif": true,
	"else": true, "esac": true, "fi": true, "for": true, "function": true,
	"if": true, "in": true, "select": true, "then": true, "time": true,
	"until": true, "while": true,
}

// undecomposableCommands run a program the tokenizer cannot read. Seeing one
// marks the line not parseable; the command itself is still classified.
var undecomposableCommands = map[string]bool{
	".": true, "eval": true, "source": true,
}

// prefixCommands wrap another command and hand execution to it. `xargs` belongs
// here for the same reason `sudo` does: the words after its flags are a command
// it runs, so classifying `xargs` alone would leave `git status | xargs rm -rf`
// governed by the wrapper's own class.
var prefixCommands = map[string]bool{
	"builtin": true, "command": true, "doas": true, "env": true,
	"exec": true, "nohup": true, "sudo": true, "xargs": true,
}

// prefixValueFlags names the wrapper flags that consume the following argument,
// so the wrapped command is not confused with a flag value. A value flag missing
// from this table costs a spurious extra segment for the value, never a skipped
// command: prefixInnerArgv stops at the first non-flag word either way.
var prefixValueFlags = map[string]map[string]bool{
	"sudo":  {"-u": true, "-g": true, "-p": true, "-C": true, "-U": true, "-T": true, "-h": true, "-r": true, "-t": true},
	"doas":  {"-u": true, "-C": true},
	"env":   {"-u": true, "-S": true},
	"xargs": {"-n": true, "-I": true, "-i": true, "-L": true, "-l": true, "-P": true, "-a": true, "-d": true, "-E": true, "-e": true, "-s": true},
}

var shellInterpreters = map[string]bool{
	"ash": true, "bash": true, "dash": true, "ksh": true, "sh": true, "zsh": true,
}

var scriptInterpreters = map[string]bool{
	"node": true, "perl": true, "php": true, "python": true, "python3": true, "ruby": true,
}

// tokenKind labels the three things scanLine emits.
type tokenKind int

const (
	// tokenWord is one shell word, already unquoted.
	tokenWord tokenKind = iota
	// tokenSeparator is a command separator: `&&`, `||`, `;`, `|`, `&`, or newline.
	tokenSeparator
	// tokenSubshell is a `( … )` group; value holds the unparsed inner text.
	tokenSubshell
	// tokenRedirectTarget is the word an output redirection writes. It is not
	// part of the command's argv: the shell, not the command, performs the write.
	tokenRedirectTarget
)

// shellToken is one lexed unit of a shell line.
type shellToken struct {
	kind tokenKind
	// value is the unquoted word text, the separator text, or the subshell body.
	value string
	// subs holds the command-substitution bodies carried by a word token.
	subs []string
}

// lineScanner is a conservative, hand-rolled POSIX-shell lexer. It models only
// the constructs Command Boundary can classify with confidence; anything else
// stops the scan so the caller can floor the verdict at require_approval.
type lineScanner struct {
	src      []rune
	pos      int
	tokens   []shellToken
	word     strings.Builder
	wordOpen bool
	subs     []string
	// writeNext marks the next flushed word as the target of an output
	// redirection rather than an argv word.
	writeNext bool
}

// scanLine lexes a shell line. ok reports whether every construct encountered was
// modeled; the tokens scanned before an unmodeled construct are still returned,
// because extra segments can only raise a verdict, never lower it.
func scanLine(src string) (tokens []shellToken, ok bool) {
	scanner := &lineScanner{src: []rune(src)}
	ok = scanner.run()
	scanner.flushWord()
	return scanner.tokens, ok
}

func (s *lineScanner) run() bool {
	for s.pos < len(s.src) {
		ch := s.src[s.pos]
		switch ch {
		case '\\':
			if !s.scanEscape() {
				return false
			}
		case '\'':
			if !s.scanSingleQuoted() {
				return false
			}
		case '"':
			if !s.scanDoubleQuoted() {
				return false
			}
		case '`':
			if !s.scanBacktick() {
				return false
			}
		case '$':
			if !s.scanDollar() {
				return false
			}
		case ' ', '\t', '\r':
			s.flushWord()
			s.pos++
		case '\n', ';', '&', '|':
			s.scanSeparator()
		case '<', '>':
			if !s.scanRedirect() {
				return false
			}
		case '(':
			if !s.scanSubshell() {
				return false
			}
		case ')':
			// A close paren with no open paren: the line does not parse.
			return false
		default:
			s.appendLiteral(string(ch))
			s.pos++
		}
	}
	return true
}

// scanEscape consumes a backslash escape. A backslash-newline continuation and a
// trailing backslash are both unmodeled and stop the scan.
func (s *lineScanner) scanEscape() bool {
	if s.pos+1 >= len(s.src) || s.src[s.pos+1] == '\n' {
		return false
	}
	s.appendLiteral(string(s.src[s.pos+1]))
	s.pos += 2
	return true
}

// scanSingleQuoted consumes a single-quoted string, in which nothing expands.
func (s *lineScanner) scanSingleQuoted() bool {
	for i := s.pos + 1; i < len(s.src); i++ {
		if s.src[i] == '\'' {
			s.appendLiteral(string(s.src[s.pos+1 : i]))
			s.pos = i + 1
			return true
		}
	}
	return false
}

// scanDoubleQuoted consumes a double-quoted string. Command substitution stays
// active inside it, so `"$(rm -rf .)"` is decomposed, not treated as text.
func (s *lineScanner) scanDoubleQuoted() bool {
	s.wordOpen = true
	s.pos++
	for s.pos < len(s.src) {
		switch s.src[s.pos] {
		case '"':
			s.pos++
			return true
		case '\\':
			if s.pos+1 >= len(s.src) || s.src[s.pos+1] == '\n' {
				return false
			}
			s.appendLiteral(string(s.src[s.pos+1]))
			s.pos += 2
		case '`':
			if !s.scanBacktick() {
				return false
			}
		case '$':
			if !s.scanDollar() {
				return false
			}
		default:
			s.appendLiteral(string(s.src[s.pos]))
			s.pos++
		}
	}
	return false
}

// scanBacktick consumes a backquoted command substitution and records its body.
func (s *lineScanner) scanBacktick() bool {
	var body strings.Builder
	for i := s.pos + 1; i < len(s.src); i++ {
		switch s.src[i] {
		case '\\':
			if i+1 >= len(s.src) {
				return false
			}
			body.WriteRune(s.src[i+1])
			i++
		case '`':
			s.addSubstitution(body.String())
			s.pos = i + 1
			return true
		default:
			body.WriteRune(s.src[i])
		}
	}
	return false
}

// scanDollar consumes an expansion. `$( … )` is recorded as a substitution to be
// decomposed; `$(( … ))` and `${ … }` are literal only while they carry no nested
// substitution; `$'…'` and `$"…"` are not decoded and stop the scan.
func (s *lineScanner) scanDollar() bool {
	rest := s.src[s.pos:]
	switch {
	case hasRunePrefix(rest, "$'"), hasRunePrefix(rest, `$"`):
		return false
	case hasRunePrefix(rest, "$(("):
		inner, end, ok := scanBalanced(s.src, s.pos+1, '(', ')')
		if !ok || containsSubstitutionMarker(inner) {
			return false
		}
		s.appendLiteral(string(s.src[s.pos:end]))
		s.pos = end
	case hasRunePrefix(rest, "$("):
		inner, end, ok := scanBalanced(s.src, s.pos+1, '(', ')')
		if !ok {
			return false
		}
		s.addSubstitution(inner)
		s.pos = end
	case hasRunePrefix(rest, "${"):
		inner, end, ok := scanBalanced(s.src, s.pos+1, '{', '}')
		if !ok || containsSubstitutionMarker(inner) {
			return false
		}
		s.appendLiteral(string(s.src[s.pos:end]))
		s.pos = end
	default:
		s.appendLiteral("$")
		s.pos++
	}
	return true
}

// scanSeparator consumes a command separator. `&>` and `&>>` are output
// redirections rather than separators and are consumed here so `&` is not
// misread; like `>`, they mark the following word as a write target.
func (s *lineScanner) scanSeparator() {
	rest := s.src[s.pos:]
	switch {
	case hasRunePrefix(rest, "&>>"):
		s.flushWord()
		s.writeNext = true
		s.pos += 3
	case hasRunePrefix(rest, "&>"):
		s.flushWord()
		s.writeNext = true
		s.pos += 2
	case hasRunePrefix(rest, "&&"):
		s.emitSeparator("&&", 2)
	case hasRunePrefix(rest, "||"):
		s.emitSeparator("||", 2)
	case hasRunePrefix(rest, "|&"):
		s.emitSeparator("|", 2)
	case hasRunePrefix(rest, ";;"):
		s.emitSeparator(";", 2)
	default:
		s.emitSeparator(string(s.src[s.pos]), 1)
	}
}

// scanRedirect consumes a redirection operator.
//
// An OUTPUT redirection (`>`, `>>`, `>|`, and the `&>` forms handled in
// scanSeparator) marks the following word as a write target, so the file the
// shell writes is classified as a write instead of being absorbed into the argv
// of the command in front of the operator.
//
// An INPUT redirection (`<`, `<<<`) and a descriptor duplication (`>&`, `<&`,
// `<>`) leave their operand to be scanned as an ordinary word: the operand of a
// read is genuinely read by the command, and a `>&` operand is usually a file
// descriptor (`2>&1`), not a path. Heredocs and process substitution are not
// modeled and stop the scan.
func (s *lineScanner) scanRedirect() bool {
	rest := s.src[s.pos:]
	switch {
	case hasRunePrefix(rest, "<("), hasRunePrefix(rest, ">("):
		return false
	case hasRunePrefix(rest, "<<<"):
		s.flushWord()
		s.pos += 3
	case hasRunePrefix(rest, "<<"):
		return false
	case hasRunePrefix(rest, ">>"), hasRunePrefix(rest, ">|"):
		s.flushWord()
		s.writeNext = true
		s.pos += 2
	case hasRunePrefix(rest, ">&"), hasRunePrefix(rest, "<&"), hasRunePrefix(rest, "<>"):
		s.flushWord()
		s.pos += 2
	case hasRunePrefix(rest, ">"):
		s.flushWord()
		s.writeNext = true
		s.pos++
	default:
		s.flushWord()
		s.pos++
	}
	return true
}

// scanSubshell consumes a `( … )` group and records its body for decomposition.
func (s *lineScanner) scanSubshell() bool {
	inner, end, ok := scanBalanced(s.src, s.pos, '(', ')')
	if !ok {
		return false
	}
	s.flushWord()
	s.tokens = append(s.tokens, shellToken{kind: tokenSubshell, value: inner})
	s.pos = end
	return true
}

func (s *lineScanner) appendLiteral(text string) {
	s.word.WriteString(text)
	s.wordOpen = true
}

func (s *lineScanner) addSubstitution(body string) {
	s.subs = append(s.subs, body)
	s.wordOpen = true
}

func (s *lineScanner) emitSeparator(value string, width int) {
	s.flushWord()
	// A separator ends the command position, so a redirect operator left dangling
	// by a degenerate line (`cat x > ; ls`) must not claim the next command's
	// first word as its target.
	s.writeNext = false
	s.tokens = append(s.tokens, shellToken{kind: tokenSeparator, value: value})
	s.pos += width
}

// flushWord emits the word under construction. A word flushed while writeNext is
// set is the target of the output redirection that set it, not an argv word.
func (s *lineScanner) flushWord() {
	if !s.wordOpen {
		return
	}
	kind := tokenWord
	if s.writeNext {
		kind = tokenRedirectTarget
		s.writeNext = false
	}
	s.tokens = append(s.tokens, shellToken{kind: kind, value: s.word.String(), subs: s.subs})
	s.word.Reset()
	s.subs = nil
	s.wordOpen = false
}

// scanBalanced returns the text between src[start] (which must be open) and its
// matching closer, honoring quotes and backslash escapes. end is the index just
// past the closer.
func scanBalanced(src []rune, start int, open, closer rune) (inner string, end int, ok bool) {
	if start >= len(src) || src[start] != open {
		return "", 0, false
	}
	depth := 0
	for i := start; i < len(src); i++ {
		switch src[i] {
		case '\\':
			i++
		case '\'':
			next := indexRune(src, i+1, '\'')
			if next < 0 {
				return "", 0, false
			}
			i = next
		case '"', '`':
			next := indexUnescapedRune(src, i+1, src[i])
			if next < 0 {
				return "", 0, false
			}
			i = next
		case open:
			depth++
		case closer:
			depth--
			if depth == 0 {
				return string(src[start+1 : i]), i + 1, true
			}
		}
	}
	return "", 0, false
}

func indexRune(src []rune, from int, target rune) int {
	for i := from; i < len(src); i++ {
		if src[i] == target {
			return i
		}
	}
	return -1
}

func indexUnescapedRune(src []rune, from int, target rune) int {
	for i := from; i < len(src); i++ {
		if src[i] == '\\' {
			i++
			continue
		}
		if src[i] == target {
			return i
		}
	}
	return -1
}

func containsSubstitutionMarker(text string) bool {
	return strings.Contains(text, "$(") || strings.Contains(text, "`")
}

func hasRunePrefix(src []rune, prefix string) bool {
	runes := []rune(prefix)
	if len(src) < len(runes) {
		return false
	}
	for i, r := range runes {
		if src[i] != r {
			return false
		}
	}
	return true
}
