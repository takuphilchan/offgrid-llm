package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/takuphilchan/offgrid-llm/pkg/api"
)

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskRunning   TaskStatus = "running"
	TaskCompleted TaskStatus = "completed"
	TaskFailed    TaskStatus = "failed"
	TaskCancelled TaskStatus = "cancelled"
)

// Task represents an agent task
type Task struct {
	ID          string      `json:"id"`
	Prompt      string      `json:"prompt"`
	Status      TaskStatus  `json:"status"`
	Result      string      `json:"result,omitempty"`
	Error       string      `json:"error,omitempty"`
	Steps       []Step      `json:"steps,omitempty"`
	Config      AgentConfig `json:"config"`
	CreatedAt   time.Time   `json:"created_at"`
	StartedAt   *time.Time  `json:"started_at,omitempty"`
	CompletedAt *time.Time  `json:"completed_at,omitempty"`
	cancel      context.CancelFunc
}

// Manager manages agent tasks and workflows
type Manager struct {
	mu          sync.RWMutex
	tasks       map[string]*Task
	tools       []api.Tool
	executor    ToolExecutor
	llmCaller   LLMCaller
	maxParallel int
	running     int
	logger      func(string, ...interface{})
}

// NewManager creates a new agent manager
func NewManager(tools []api.Tool, executor ToolExecutor, llmCaller LLMCaller) *Manager {
	return &Manager{
		tasks:       make(map[string]*Task),
		tools:       tools,
		executor:    executor,
		llmCaller:   llmCaller,
		maxParallel: 3,
		logger:      func(format string, args ...interface{}) {},
	}
}

// SetLogger sets the logging function
func (m *Manager) SetLogger(logger func(string, ...interface{})) {
	m.logger = logger
}

// SetMaxParallel sets the maximum number of parallel tasks
func (m *Manager) SetMaxParallel(max int) {
	m.maxParallel = max
}

// SetExecutor sets the tool executor
func (m *Manager) SetExecutor(executor ToolExecutor) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.executor = executor
}

// SetLLMCaller sets the LLM caller
func (m *Manager) SetLLMCaller(caller LLMCaller) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.llmCaller = caller
}

// RegisterTool adds a tool for agents to use
func (m *Manager) RegisterTool(tool api.Tool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tools = append(m.tools, tool)
}

// CreateTask creates a new task (doesn't start it)
func (m *Manager) CreateTask(id, prompt string, config *AgentConfig) *Task {
	m.mu.Lock()
	defer m.mu.Unlock()

	cfg := DefaultAgentConfig()
	if config != nil {
		cfg = *config
	}

	task := &Task{
		ID:        id,
		Prompt:    prompt,
		Status:    TaskPending,
		Config:    cfg,
		CreatedAt: time.Now(),
	}

	m.tasks[id] = task
	return task
}

// StartTask marks a task as running
func (m *Manager) StartTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[id]
	if !exists {
		return fmt.Errorf("task not found: %s", id)
	}

	now := time.Now()
	task.StartedAt = &now
	task.Status = TaskRunning
	return nil
}

// AddTaskStep adds a step to a task
func (m *Manager) AddTaskStep(id string, step Step) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[id]
	if !exists {
		return fmt.Errorf("task not found: %s", id)
	}

	task.Steps = append(task.Steps, step)
	return nil
}

// CompleteTask marks a task as completed or failed
func (m *Manager) CompleteTask(id string, result string, err error) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[id]
	if !exists {
		return fmt.Errorf("task not found: %s", id)
	}

	completedAt := time.Now()
	task.CompletedAt = &completedAt

	if err != nil {
		task.Status = TaskFailed
		task.Error = err.Error()
	} else {
		task.Status = TaskCompleted
		task.Result = result
	}
	return nil
}

// RunTask runs a task synchronously
func (m *Manager) RunTask(ctx context.Context, id string) (*Task, error) {
	m.mu.RLock()
	task, exists := m.tasks[id]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("task not found: %s", id)
	}

	if task.Status != TaskPending {
		return nil, fmt.Errorf("task already started or completed")
	}

	return m.executeTask(ctx, task)
}

// RunTaskAsync runs a task asynchronously
func (m *Manager) RunTaskAsync(id string) error {
	m.mu.Lock()
	task, exists := m.tasks[id]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("task not found: %s", id)
	}

	if task.Status != TaskPending {
		m.mu.Unlock()
		return fmt.Errorf("task already started or completed")
	}

	if m.running >= m.maxParallel {
		m.mu.Unlock()
		return fmt.Errorf("maximum parallel tasks reached (%d)", m.maxParallel)
	}

	m.running++
	ctx, cancel := context.WithCancel(context.Background())
	task.cancel = cancel
	m.mu.Unlock()

	go func() {
		defer func() {
			m.mu.Lock()
			m.running--
			m.mu.Unlock()
		}()

		m.executeTask(ctx, task)
	}()

	return nil
}

// executeTask executes a task
func (m *Manager) executeTask(ctx context.Context, task *Task) (*Task, error) {
	now := time.Now()
	task.StartedAt = &now
	task.Status = TaskRunning

	// Create agent
	agent := NewAgent(task.Config, m.tools, m.executor, m.llmCaller)
	agent.SetStepCallback(func(step Step) {
		m.mu.Lock()
		task.Steps = append(task.Steps, step)
		m.mu.Unlock()
	})

	// Run agent
	result, err := agent.Run(ctx, task.Prompt)

	completedAt := time.Now()
	task.CompletedAt = &completedAt

	if err != nil {
		task.Status = TaskFailed
		task.Error = err.Error()
		return task, err
	}

	task.Status = TaskCompleted
	task.Result = result
	return task, nil
}

// RunImmediate creates and runs a task immediately
func (m *Manager) RunImmediate(ctx context.Context, prompt string, config *AgentConfig) (string, []Step, error) {
	id := fmt.Sprintf("immediate-%d", time.Now().UnixNano())
	task := m.CreateTask(id, prompt, config)

	_, err := m.RunTask(ctx, id)
	if err != nil {
		return "", task.Steps, err
	}

	return task.Result, task.Steps, nil
}

// GetTask returns a task by ID
func (m *Manager) GetTask(id string) (*Task, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	task, exists := m.tasks[id]
	return task, exists
}

// ListTasks returns all tasks
func (m *Manager) ListTasks() []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*Task, 0, len(m.tasks))
	for _, task := range m.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

// CancelTask cancels a running task
func (m *Manager) CancelTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[id]
	if !exists {
		return fmt.Errorf("task not found: %s", id)
	}

	if task.Status != TaskRunning {
		return fmt.Errorf("task is not running")
	}

	if task.cancel != nil {
		task.cancel()
	}

	task.Status = TaskCancelled
	return nil
}

// DeleteTask removes a task
func (m *Manager) DeleteTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[id]
	if !exists {
		return fmt.Errorf("task not found: %s", id)
	}

	if task.Status == TaskRunning {
		if task.cancel != nil {
			task.cancel()
		}
	}

	delete(m.tasks, id)
	return nil
}

// Workflow represents a multi-step workflow
type Workflow struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Steps       []WorkflowStep `json:"steps"`
	Variables   map[string]any `json:"variables"`
}

// WorkflowStep represents a step in a workflow
type WorkflowStep struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Type      string         `json:"type"` // "agent", "tool", "condition", "loop"
	Prompt    string         `json:"prompt,omitempty"`
	ToolName  string         `json:"tool_name,omitempty"`
	ToolArgs  map[string]any `json:"tool_args,omitempty"`
	Condition string         `json:"condition,omitempty"`
	OnTrue    string         `json:"on_true,omitempty"`    // Next step ID if condition is true
	OnFalse   string         `json:"on_false,omitempty"`   // Next step ID if condition is false
	OutputVar string         `json:"output_var,omitempty"` // Variable to store result
	DependsOn []string       `json:"depends_on,omitempty"`
}

// WorkflowExecution tracks a workflow execution
type WorkflowExecution struct {
	ID            string                         `json:"id"`
	WorkflowID    string                         `json:"workflow_id"`
	Status        TaskStatus                     `json:"status"`
	Variables     map[string]any                 `json:"variables"`
	StepResults   map[string]*WorkflowStepResult `json:"step_results"`
	CurrentStepID string                         `json:"current_step_id"`
	StartedAt     time.Time                      `json:"started_at"`
	CompletedAt   *time.Time                     `json:"completed_at,omitempty"`
	Error         string                         `json:"error,omitempty"`
}

// WorkflowStepResult contains the result of a workflow step
type WorkflowStepResult struct {
	StepID      string     `json:"step_id"`
	Status      TaskStatus `json:"status"`
	Output      any        `json:"output,omitempty"`
	Error       string     `json:"error,omitempty"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt time.Time  `json:"completed_at"`
}

// WorkflowEngine executes workflows
type WorkflowEngine struct {
	mu         sync.RWMutex
	workflows  map[string]*Workflow
	executions map[string]*WorkflowExecution
	manager    *Manager
}

// NewWorkflowEngine creates a new workflow engine
func NewWorkflowEngine(manager *Manager) *WorkflowEngine {
	return &WorkflowEngine{
		workflows:  make(map[string]*Workflow),
		executions: make(map[string]*WorkflowExecution),
		manager:    manager,
	}
}

// RegisterWorkflow registers a workflow
func (e *WorkflowEngine) RegisterWorkflow(workflow *Workflow) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.workflows[workflow.ID] = workflow
}

// ExecuteWorkflow executes a workflow
func (e *WorkflowEngine) ExecuteWorkflow(ctx context.Context, workflowID string, inputVars map[string]any) (*WorkflowExecution, error) {
	e.mu.RLock()
	workflow, exists := e.workflows[workflowID]
	e.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}

	execID := fmt.Sprintf("exec-%d", time.Now().UnixNano())
	execution := &WorkflowExecution{
		ID:          execID,
		WorkflowID:  workflowID,
		Status:      TaskRunning,
		Variables:   make(map[string]any),
		StepResults: make(map[string]*WorkflowStepResult),
		StartedAt:   time.Now(),
	}

	// Copy workflow variables
	for k, v := range workflow.Variables {
		execution.Variables[k] = v
	}
	// Override with input variables
	for k, v := range inputVars {
		execution.Variables[k] = v
	}

	e.mu.Lock()
	e.executions[execID] = execution
	e.mu.Unlock()

	// Execute steps
	err := e.executeSteps(ctx, workflow, execution)

	now := time.Now()
	execution.CompletedAt = &now

	if err != nil {
		execution.Status = TaskFailed
		execution.Error = err.Error()
		return execution, err
	}

	execution.Status = TaskCompleted
	return execution, nil
}

// executeSteps executes workflow steps
func (e *WorkflowEngine) executeSteps(ctx context.Context, workflow *Workflow, execution *WorkflowExecution) error {
	if len(workflow.Steps) == 0 {
		return nil
	}

	// Build step map
	stepMap := make(map[string]*WorkflowStep)
	for i := range workflow.Steps {
		stepMap[workflow.Steps[i].ID] = &workflow.Steps[i]
	}

	// Start with first step
	currentStepID := workflow.Steps[0].ID

	for currentStepID != "" {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		step, exists := stepMap[currentStepID]
		if !exists {
			return fmt.Errorf("step not found: %s", currentStepID)
		}

		execution.CurrentStepID = currentStepID

		result := &WorkflowStepResult{
			StepID:    step.ID,
			Status:    TaskRunning,
			StartedAt: time.Now(),
		}

		var output any
		var err error
		var nextStepID string

		switch step.Type {
		case "agent":
			prompt := e.interpolateVariables(step.Prompt, execution.Variables)
			output, _, err = e.manager.RunImmediate(ctx, prompt, nil)
			nextStepID = e.getNextStep(workflow, step.ID)

		case "tool":
			args, _ := json.Marshal(step.ToolArgs)
			output, err = e.manager.executor(ctx, step.ToolName, args)
			nextStepID = e.getNextStep(workflow, step.ID)

		case "condition":
			// Simple condition evaluation
			condResult := e.evaluateCondition(step.Condition, execution.Variables)
			if condResult {
				nextStepID = step.OnTrue
			} else {
				nextStepID = step.OnFalse
			}
			output = condResult

		default:
			err = fmt.Errorf("unknown step type: %s", step.Type)
		}

		result.CompletedAt = time.Now()

		if err != nil {
			result.Status = TaskFailed
			result.Error = err.Error()
			execution.StepResults[step.ID] = result
			return err
		}

		result.Status = TaskCompleted
		result.Output = output
		execution.StepResults[step.ID] = result

		// Store output in variable if specified
		if step.OutputVar != "" {
			execution.Variables[step.OutputVar] = output
		}

		currentStepID = nextStepID
	}

	return nil
}

// getNextStep returns the next step ID in sequence
func (e *WorkflowEngine) getNextStep(workflow *Workflow, currentID string) string {
	for i, step := range workflow.Steps {
		if step.ID == currentID && i+1 < len(workflow.Steps) {
			return workflow.Steps[i+1].ID
		}
	}
	return ""
}

// interpolateVariables replaces {{var}} with actual values
func (e *WorkflowEngine) interpolateVariables(template string, vars map[string]any) string {
	result := template
	for k, v := range vars {
		placeholder := fmt.Sprintf("{{%s}}", k)
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", v))
	}
	return result
}

// evaluateCondition evaluates a simple condition
func (e *WorkflowEngine) evaluateCondition(condition string, vars map[string]any) bool {
	// Simple implementation - just check if variable is truthy
	for k, v := range vars {
		if condition == k {
			switch val := v.(type) {
			case bool:
				return val
			case string:
				return val != ""
			case int, int64, float64:
				return v != 0
			default:
				return v != nil
			}
		}
	}
	return false
}
