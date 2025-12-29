package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// OrchestrationMode defines how multiple agents collaborate
type OrchestrationMode string

const (
	// ModeSequential - agents run one after another, passing results
	ModeSequential OrchestrationMode = "sequential"
	// ModeParallel - agents run simultaneously on the same task
	ModeParallel OrchestrationMode = "parallel"
	// ModeDebate - agents discuss and refine answers
	ModeDebate OrchestrationMode = "debate"
	// ModeVoting - agents vote on best answer
	ModeVoting OrchestrationMode = "voting"
	// ModeHierarchy - supervisor delegates to worker agents
	ModeHierarchy OrchestrationMode = "hierarchy"
)

// AgentRole defines an agent's role in orchestration
type AgentRole struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Template    string      `json:"template"` // Agent template (researcher, coder, etc.)
	Config      AgentConfig `json:"config"`
	Priority    int         `json:"priority"` // For hierarchy mode
}

// OrchestrationConfig configures multi-agent orchestration
type OrchestrationConfig struct {
	Mode            OrchestrationMode `json:"mode"`
	Agents          []AgentRole       `json:"agents"`
	MaxRounds       int               `json:"max_rounds"`       // For debate mode
	VotingQuorum    float64           `json:"voting_quorum"`    // For voting mode (0.5 = majority)
	Supervisor      string            `json:"supervisor"`       // For hierarchy mode
	FinalAggregator string            `json:"final_aggregator"` // How to combine results
	Timeout         time.Duration     `json:"timeout"`
}

// DefaultOrchestrationConfig returns a default configuration
func DefaultOrchestrationConfig() OrchestrationConfig {
	return OrchestrationConfig{
		Mode:            ModeSequential,
		MaxRounds:       3,
		VotingQuorum:    0.5,
		FinalAggregator: "combine",
		Timeout:         5 * time.Minute,
	}
}

// OrchestrationResult contains the results of multi-agent orchestration
type OrchestrationResult struct {
	ID            string            `json:"id"`
	Mode          OrchestrationMode `json:"mode"`
	AgentResults  []AgentResult     `json:"agent_results"`
	FinalResult   string            `json:"final_result"`
	Consensus     float64           `json:"consensus,omitempty"` // For voting/debate
	DebateRounds  []DebateRound     `json:"debate_rounds,omitempty"`
	TotalDuration time.Duration     `json:"total_duration"`
	Error         string            `json:"error,omitempty"`
}

// AgentResult contains a single agent's contribution
type AgentResult struct {
	AgentName  string        `json:"agent_name"`
	Role       string        `json:"role"`
	Result     string        `json:"result"`
	Duration   time.Duration `json:"duration"`
	Vote       string        `json:"vote,omitempty"` // For voting mode
	Confidence float64       `json:"confidence,omitempty"`
}

// DebateRound represents one round of agent debate
type DebateRound struct {
	Round     int             `json:"round"`
	Arguments []AgentArgument `json:"arguments"`
	Synthesis string          `json:"synthesis,omitempty"`
}

// AgentArgument represents an agent's argument in a debate
type AgentArgument struct {
	AgentName string `json:"agent_name"`
	Position  string `json:"position"`
	Reasoning string `json:"reasoning"`
	Rebuttal  string `json:"rebuttal,omitempty"` // Response to previous round
}

// Orchestrator manages multi-agent workflows
type Orchestrator struct {
	manager   *Manager
	mu        sync.RWMutex
	workflows map[string]*OrchestrationResult
	logger    func(string, ...interface{})
}

// NewOrchestrator creates a new orchestrator
func NewOrchestrator(manager *Manager) *Orchestrator {
	return &Orchestrator{
		manager:   manager,
		workflows: make(map[string]*OrchestrationResult),
		logger:    func(format string, args ...interface{}) {},
	}
}

// SetLogger sets the logging function
func (o *Orchestrator) SetLogger(logger func(string, ...interface{})) {
	o.logger = logger
}

// RunOrchestration executes a multi-agent workflow
func (o *Orchestrator) RunOrchestration(ctx context.Context, id, prompt string, config OrchestrationConfig) (*OrchestrationResult, error) {
	if len(config.Agents) == 0 {
		return nil, fmt.Errorf("no agents configured for orchestration")
	}

	// Create context with timeout
	if config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, config.Timeout)
		defer cancel()
	}

	result := &OrchestrationResult{
		ID:   id,
		Mode: config.Mode,
	}

	startTime := time.Now()

	var err error
	switch config.Mode {
	case ModeSequential:
		err = o.runSequential(ctx, prompt, config, result)
	case ModeParallel:
		err = o.runParallel(ctx, prompt, config, result)
	case ModeDebate:
		err = o.runDebate(ctx, prompt, config, result)
	case ModeVoting:
		err = o.runVoting(ctx, prompt, config, result)
	case ModeHierarchy:
		err = o.runHierarchy(ctx, prompt, config, result)
	default:
		err = fmt.Errorf("unknown orchestration mode: %s", config.Mode)
	}

	result.TotalDuration = time.Since(startTime)
	if err != nil {
		result.Error = err.Error()
	}

	// Store result
	o.mu.Lock()
	o.workflows[id] = result
	o.mu.Unlock()

	return result, err
}

// runSequential executes agents one after another, passing context
func (o *Orchestrator) runSequential(ctx context.Context, prompt string, config OrchestrationConfig, result *OrchestrationResult) error {
	currentPrompt := prompt
	for i, agent := range config.Agents {
		o.logger("[Orchestration] Running agent %d/%d: %s", i+1, len(config.Agents), agent.Name)

		// Add context from previous agent if available
		if i > 0 && len(result.AgentResults) > 0 {
			prevResult := result.AgentResults[i-1]
			currentPrompt = fmt.Sprintf("%s\n\nPrevious analysis from %s:\n%s", prompt, prevResult.AgentName, prevResult.Result)
		}

		agentResult, err := o.runSingleAgent(ctx, agent, currentPrompt)
		if err != nil {
			return fmt.Errorf("agent %s failed: %w", agent.Name, err)
		}

		result.AgentResults = append(result.AgentResults, *agentResult)
	}

	// Final result is the last agent's output
	if len(result.AgentResults) > 0 {
		result.FinalResult = result.AgentResults[len(result.AgentResults)-1].Result
	}

	return nil
}

// runParallel executes all agents simultaneously
func (o *Orchestrator) runParallel(ctx context.Context, prompt string, config OrchestrationConfig, result *OrchestrationResult) error {
	var wg sync.WaitGroup
	resultsChan := make(chan AgentResult, len(config.Agents))
	errorsChan := make(chan error, len(config.Agents))

	for _, agent := range config.Agents {
		wg.Add(1)
		go func(a AgentRole) {
			defer wg.Done()

			o.logger("[Orchestration] Running parallel agent: %s", a.Name)
			agentResult, err := o.runSingleAgent(ctx, a, prompt)
			if err != nil {
				errorsChan <- fmt.Errorf("agent %s failed: %w", a.Name, err)
				return
			}
			resultsChan <- *agentResult
		}(agent)
	}

	wg.Wait()
	close(resultsChan)
	close(errorsChan)

	// Collect results
	for ar := range resultsChan {
		result.AgentResults = append(result.AgentResults, ar)
	}

	// Check for errors
	var errs []string
	for err := range errorsChan {
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		return fmt.Errorf("parallel execution errors: %s", strings.Join(errs, "; "))
	}

	// Aggregate results
	result.FinalResult = o.aggregateResults(result.AgentResults, config.FinalAggregator)

	return nil
}

// runDebate has agents discuss and refine their answers
func (o *Orchestrator) runDebate(ctx context.Context, prompt string, config OrchestrationConfig, result *OrchestrationResult) error {
	if len(config.Agents) < 2 {
		return fmt.Errorf("debate mode requires at least 2 agents")
	}

	// Round 1: Initial positions
	round := DebateRound{Round: 1}
	for _, agent := range config.Agents {
		o.logger("[Debate Round 1] Agent %s forming initial position", agent.Name)

		debatePrompt := fmt.Sprintf("Question: %s\n\nProvide your initial analysis and position. Be thorough but concise.", prompt)
		agentResult, err := o.runSingleAgent(ctx, agent, debatePrompt)
		if err != nil {
			return fmt.Errorf("agent %s failed in round 1: %w", agent.Name, err)
		}

		round.Arguments = append(round.Arguments, AgentArgument{
			AgentName: agent.Name,
			Position:  agentResult.Result,
			Reasoning: "Initial position",
		})
	}
	result.DebateRounds = append(result.DebateRounds, round)

	// Subsequent rounds: Rebuttals and refinement
	for r := 2; r <= config.MaxRounds; r++ {
		round := DebateRound{Round: r}
		prevRound := result.DebateRounds[r-2]

		for i, agent := range config.Agents {
			// Collect other agents' positions
			var otherPositions []string
			for j, arg := range prevRound.Arguments {
				if j != i {
					otherPositions = append(otherPositions, fmt.Sprintf("%s's position: %s", arg.AgentName, arg.Position))
				}
			}

			o.logger("[Debate Round %d] Agent %s responding to others", r, agent.Name)

			debatePrompt := fmt.Sprintf(`Question: %s

Your previous position: %s

Other positions:
%s

Consider the other perspectives. You may:
1. Strengthen your original position with additional reasoning
2. Acknowledge valid points from others and refine your view
3. Present a synthesis that incorporates the best insights

Provide your updated analysis.`, prompt, prevRound.Arguments[i].Position, strings.Join(otherPositions, "\n\n"))

			agentResult, err := o.runSingleAgent(ctx, agent, debatePrompt)
			if err != nil {
				return fmt.Errorf("agent %s failed in round %d: %w", agent.Name, r, err)
			}

			round.Arguments = append(round.Arguments, AgentArgument{
				AgentName: agent.Name,
				Position:  agentResult.Result,
				Reasoning: "Refined position",
				Rebuttal:  fmt.Sprintf("Response to round %d positions", r-1),
			})
		}
		result.DebateRounds = append(result.DebateRounds, round)
	}

	// Final synthesis
	lastRound := result.DebateRounds[len(result.DebateRounds)-1]
	var finalPositions []string
	for _, arg := range lastRound.Arguments {
		finalPositions = append(finalPositions, fmt.Sprintf("%s: %s", arg.AgentName, arg.Position))
		result.AgentResults = append(result.AgentResults, AgentResult{
			AgentName: arg.AgentName,
			Result:    arg.Position,
		})
	}

	// Use first agent to synthesize if no supervisor specified
	synthesizer := config.Agents[0]
	if config.Supervisor != "" {
		for _, a := range config.Agents {
			if a.Name == config.Supervisor {
				synthesizer = a
				break
			}
		}
	}

	synthesisPrompt := fmt.Sprintf(`Question: %s

After %d rounds of debate, here are the final positions:

%s

Provide a final synthesis that:
1. Identifies points of consensus
2. Highlights remaining disagreements
3. Gives a well-reasoned final answer

Be comprehensive but concise.`, prompt, config.MaxRounds, strings.Join(finalPositions, "\n\n"))

	synthResult, err := o.runSingleAgent(ctx, synthesizer, synthesisPrompt)
	if err != nil {
		return fmt.Errorf("synthesis failed: %w", err)
	}

	result.FinalResult = synthResult.Result
	return nil
}

// runVoting has agents vote on the best answer
func (o *Orchestrator) runVoting(ctx context.Context, prompt string, config OrchestrationConfig, result *OrchestrationResult) error {
	if len(config.Agents) < 2 {
		return fmt.Errorf("voting mode requires at least 2 agents")
	}

	// Phase 1: Each agent generates an answer
	var answers []string
	for _, agent := range config.Agents {
		o.logger("[Voting] Agent %s generating answer", agent.Name)

		agentResult, err := o.runSingleAgent(ctx, agent, prompt)
		if err != nil {
			return fmt.Errorf("agent %s failed to generate answer: %w", agent.Name, err)
		}
		answers = append(answers, agentResult.Result)
		result.AgentResults = append(result.AgentResults, *agentResult)
	}

	// Phase 2: Each agent votes for the best answer (not their own)
	votes := make(map[int]int) // answer index -> vote count
	for i, agent := range config.Agents {
		// Present all answers for voting
		var answerList []string
		for j, ans := range answers {
			if j != i { // Can't vote for own answer
				answerList = append(answerList, fmt.Sprintf("Option %d:\n%s", j+1, ans))
			}
		}

		votePrompt := fmt.Sprintf(`Question: %s

Here are the candidate answers from other experts:

%s

Evaluate each answer for:
- Accuracy and correctness
- Completeness
- Clarity

Which option number is the best answer? Reply with just the number.`, prompt, strings.Join(answerList, "\n\n---\n\n"))

		voteResult, err := o.runSingleAgent(ctx, agent, votePrompt)
		if err != nil {
			o.logger("[Voting] Agent %s failed to vote: %v", agent.Name, err)
			continue
		}

		// Parse vote
		var votedFor int
		fmt.Sscanf(strings.TrimSpace(voteResult.Result), "%d", &votedFor)
		if votedFor > 0 && votedFor <= len(answers) {
			votes[votedFor-1]++
			result.AgentResults[i].Vote = fmt.Sprintf("Option %d", votedFor)
		}
	}

	// Determine winner
	maxVotes := 0
	winner := 0
	for idx, count := range votes {
		if count > maxVotes {
			maxVotes = count
			winner = idx
		}
	}

	result.Consensus = float64(maxVotes) / float64(len(config.Agents))
	if result.Consensus >= config.VotingQuorum {
		result.FinalResult = answers[winner]
	} else {
		// No clear winner - combine top answers
		result.FinalResult = fmt.Sprintf("No consensus reached (%.0f%%). Top answer:\n\n%s", result.Consensus*100, answers[winner])
	}

	return nil
}

// runHierarchy has a supervisor delegate work to workers
func (o *Orchestrator) runHierarchy(ctx context.Context, prompt string, config OrchestrationConfig, result *OrchestrationResult) error {
	// Find supervisor
	var supervisor *AgentRole
	var workers []AgentRole
	for _, agent := range config.Agents {
		if agent.Name == config.Supervisor || agent.Priority == 0 {
			supervisor = &agent
		} else {
			workers = append(workers, agent)
		}
	}

	if supervisor == nil {
		// Use first agent as supervisor
		supervisor = &config.Agents[0]
		workers = config.Agents[1:]
	}

	if len(workers) == 0 {
		return fmt.Errorf("hierarchy mode requires at least one worker agent")
	}

	// Supervisor plans the work
	o.logger("[Hierarchy] Supervisor %s planning work", supervisor.Name)

	// Create worker descriptions
	var workerDescs []string
	for _, w := range workers {
		workerDescs = append(workerDescs, fmt.Sprintf("- %s: %s", w.Name, w.Description))
	}

	planPrompt := fmt.Sprintf(`You are a supervisor coordinating a team of specialists. 

Task: %s

Available team members:
%s

Create a work plan as JSON with this structure:
{
  "subtasks": [
    {"agent": "agent_name", "task": "specific subtask description"},
    ...
  ]
}

Delegate subtasks to the most appropriate team members. Each team member can handle one subtask.`, prompt, strings.Join(workerDescs, "\n"))

	planResult, err := o.runSingleAgent(ctx, *supervisor, planPrompt)
	if err != nil {
		return fmt.Errorf("supervisor failed to create plan: %w", err)
	}

	// Parse plan
	type WorkPlan struct {
		Subtasks []struct {
			Agent string `json:"agent"`
			Task  string `json:"task"`
		} `json:"subtasks"`
	}

	var plan WorkPlan
	// Extract JSON from response
	planJSON := extractJSON(planResult.Result)
	if err := json.Unmarshal([]byte(planJSON), &plan); err != nil {
		// Fallback: run all workers on original prompt
		o.logger("[Hierarchy] Could not parse plan, falling back to parallel execution")
		return o.runParallel(ctx, prompt, config, result)
	}

	// Execute subtasks
	var workerResults []AgentResult
	for _, subtask := range plan.Subtasks {
		// Find the worker
		var worker *AgentRole
		for _, w := range workers {
			if w.Name == subtask.Agent {
				worker = &w
				break
			}
		}

		if worker == nil {
			o.logger("[Hierarchy] Worker %s not found, skipping subtask", subtask.Agent)
			continue
		}

		o.logger("[Hierarchy] Worker %s executing: %s", worker.Name, subtask.Task)
		workerResult, err := o.runSingleAgent(ctx, *worker, subtask.Task)
		if err != nil {
			o.logger("[Hierarchy] Worker %s failed: %v", worker.Name, err)
			continue
		}

		workerResult.Role = subtask.Task
		workerResults = append(workerResults, *workerResult)
	}

	result.AgentResults = workerResults

	// Supervisor synthesizes results
	var resultsSummary []string
	for _, r := range workerResults {
		resultsSummary = append(resultsSummary, fmt.Sprintf("%s (Task: %s):\n%s", r.AgentName, r.Role, r.Result))
	}

	synthesisPrompt := fmt.Sprintf(`Original task: %s

Your team has completed their assigned work. Here are the results:

%s

Synthesize these results into a comprehensive final answer. Ensure the answer is complete, coherent, and addresses the original task.`, prompt, strings.Join(resultsSummary, "\n\n---\n\n"))

	synthResult, err := o.runSingleAgent(ctx, *supervisor, synthesisPrompt)
	if err != nil {
		return fmt.Errorf("supervisor failed to synthesize: %w", err)
	}

	result.FinalResult = synthResult.Result
	return nil
}

// runSingleAgent runs a single agent and returns the result
func (o *Orchestrator) runSingleAgent(ctx context.Context, agent AgentRole, prompt string) (*AgentResult, error) {
	startTime := time.Now()

	// Apply agent template if specified
	config := agent.Config
	if config.SystemPrompt == "" && agent.Template != "" {
		for _, template := range BuiltInTemplates() {
			if template.ID == agent.Template {
				config.SystemPrompt = template.Config.SystemPrompt
				break
			}
		}
	}

	// Create task ID
	taskID := fmt.Sprintf("orch-%s-%d", agent.Name, time.Now().UnixNano())

	// Create and run task (for tracking)
	o.manager.CreateTask(taskID, prompt, &config)
	if err := o.manager.StartTask(taskID); err != nil {
		return nil, err
	}

	// Run agent using RunImmediate
	result, _, err := o.manager.RunImmediate(ctx, prompt, &config)
	if err != nil {
		o.manager.CompleteTask(taskID, "", err)
		return nil, err
	}

	o.manager.CompleteTask(taskID, result, nil)

	return &AgentResult{
		AgentName: agent.Name,
		Role:      agent.Description,
		Result:    result,
		Duration:  time.Since(startTime),
	}, nil
}

// aggregateResults combines multiple agent results
func (o *Orchestrator) aggregateResults(results []AgentResult, method string) string {
	if len(results) == 0 {
		return ""
	}

	switch method {
	case "first":
		return results[0].Result
	case "last":
		return results[len(results)-1].Result
	case "combine":
		var parts []string
		for _, r := range results {
			parts = append(parts, fmt.Sprintf("## %s\n%s", r.AgentName, r.Result))
		}
		return strings.Join(parts, "\n\n---\n\n")
	default:
		return results[len(results)-1].Result
	}
}

// extractJSON extracts JSON from a string that may contain other text
func extractJSON(s string) string {
	start := strings.Index(s, "{")
	if start == -1 {
		return "{}"
	}

	// Find matching closing brace
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}

	return s[start:]
}

// GetWorkflow returns a workflow result by ID
func (o *Orchestrator) GetWorkflow(id string) (*OrchestrationResult, bool) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	result, exists := o.workflows[id]
	return result, exists
}

// ListWorkflows returns all workflow results
func (o *Orchestrator) ListWorkflows() []*OrchestrationResult {
	o.mu.RLock()
	defer o.mu.RUnlock()

	var results []*OrchestrationResult
	for _, r := range o.workflows {
		results = append(results, r)
	}
	return results
}
