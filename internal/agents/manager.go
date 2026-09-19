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
	TaskPending     TaskStatus = "pending"
	TaskRunning     TaskStatus = "running"
	TaskWaiting     TaskStatus = "waiting_for_approval"
	TaskCompleted   TaskStatus = "completed"
	TaskFailed      TaskStatus = "failed"
	TaskCancelled   TaskStatus = "cancelled"
	TaskInterrupted TaskStatus = "interrupted"
	TaskUncertain   TaskStatus = "uncertain"
)

// Task represents an agent task
type Task struct {
	ID              string       `json:"id"`
	Prompt          string       `json:"prompt"`
	Status          TaskStatus   `json:"status"`
	Result          string       `json:"result,omitempty"`
	Error           string       `json:"error,omitempty"`
	Steps           []Step       `json:"steps,omitempty"`
	Config          AgentConfig  `json:"config"`
	CreatedAt       time.Time    `json:"created_at"`
	StartedAt       *time.Time   `json:"started_at,omitempty"`
	CompletedAt     *time.Time   `json:"completed_at,omitempty"`
	DeletedAt       *time.Time   `json:"deleted_at,omitempty"`
	Model           string       `json:"model,omitempty"`
	Actor           string       `json:"actor,omitempty"`
	PendingApproval *Approval    `json:"pending_approval,omitempty"`
	Checkpoint      *Checkpoint  `json:"checkpoint,omitempty"`
	Progress        *RunProgress `json:"progress,omitempty"`
	cancel          context.CancelFunc
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
	dataDir     string // Directory for persisting tasks
	storageErr  error
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

// NewManagerWithPersistence creates a new agent manager with disk persistence
func NewManagerWithPersistence(tools []api.Tool, executor ToolExecutor, llmCaller LLMCaller, dataDir string) *Manager {
	m := &Manager{
		tasks:       make(map[string]*Task),
		tools:       tools,
		executor:    executor,
		llmCaller:   llmCaller,
		maxParallel: 3,
		logger:      func(format string, args ...interface{}) {},
		dataDir:     dataDir,
	}
	// Load existing tasks from disk
	m.loadTasks()
	return m
}

// SetDataDir sets the data directory for persistence
func (m *Manager) SetDataDir(dataDir string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dataDir = dataDir
	if dataDir != "" {
		m.loadTasks()
	}
}

// SetLogger sets the logging function
func (m *Manager) SetLogger(logger func(string, ...interface{})) {
	m.logger = logger
}

// SetMaxParallel sets the maximum number of parallel tasks
func (m *Manager) SetMaxParallel(max int) {
	m.mu.Lock()
	defer m.mu.Unlock()
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

// CreateTask persists a new task before exposing it to callers.
func (m *Manager) CreateTask(id, prompt string, config *AgentConfig) (*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.tasks[id]; exists {
		return nil, ErrRunConflict
	}
	cfg := DefaultAgentConfig()
	if config != nil {
		cfg = *config
	}
	task := &Task{ID: id, Prompt: prompt, Status: TaskPending, Config: cfg, CreatedAt: time.Now().UTC()}
	if err := m.saveTask(task); err != nil {
		return nil, err
	}
	m.tasks[id] = task
	return copyTask(task)
}

func (m *Manager) StartTask(id string) error {
	_, err := m.updateTask(id, func(task *Task) error {
		if task.Status != TaskPending {
			return ErrRunConflict
		}
		now := time.Now().UTC()
		task.StartedAt, task.Status = &now, TaskRunning
		return nil
	})
	return err
}

func (m *Manager) AddTaskStep(id string, step Step) error {
	_, err := m.updateTask(id, func(task *Task) error {
		task.Steps = append(task.Steps, step)
		return nil
	})
	return err
}

func (m *Manager) WaitForApproval(id string, reason error) error {
	_, err := m.updateTask(id, func(task *Task) error {
		task.Status, task.CompletedAt = TaskWaiting, nil
		if reason != nil {
			task.Error = reason.Error()
		}
		return nil
	})
	return err
}

func (m *Manager) CompleteTask(id string, result string, cause error) error {
	_, err := m.updateTask(id, func(task *Task) error {
		if task.Status == TaskCancelled {
			return nil
		}
		if cause != nil {
			finishTask(task, TaskFailed, cause.Error())
		} else {
			finishTask(task, TaskCompleted, "")
			task.Result = result
		}
		return nil
	})
	return err
}

func (m *Manager) RunTask(ctx context.Context, id string) (*Task, error) {
	if err := m.StartTask(id); err != nil {
		return nil, err
	}
	return m.executeTask(ctx, id)
}

func (m *Manager) RunTaskAsync(id string) error {
	m.mu.Lock()
	if m.running >= m.maxParallel {
		m.mu.Unlock()
		return ErrRunConflict
	}
	m.running++
	m.mu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	if err := m.StartTask(id); err != nil {
		cancel()
		m.mu.Lock()
		m.running--
		m.mu.Unlock()
		return err
	}
	if _, err := m.updateTask(id, func(task *Task) error { task.cancel = cancel; return nil }); err != nil {
		cancel()
		m.mu.Lock()
		m.running--
		m.mu.Unlock()
		return err
	}
	go func() {
		defer cancel()
		defer func() { m.mu.Lock(); m.running--; m.mu.Unlock() }()
		if _, err := m.executeTask(ctx, id); err != nil {
			m.logger("Agent task failed: %v", err)
		}
	}()
	return nil
}

func (m *Manager) executeTask(ctx context.Context, id string) (*Task, error) {
	task, ok := m.GetTask(id)
	if !ok {
		return nil, ErrTaskNotFound
	}
	m.mu.RLock()
	toolList := append([]api.Tool(nil), m.tools...)
	executor, caller := m.executor, m.llmCaller
	m.mu.RUnlock()
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	agent := NewAgent(task.Config, toolList, executor, caller)
	var persistenceErr error
	agent.SetStepCallback(func(step Step) {
		if persistenceErr == nil {
			persistenceErr = m.AddTaskStep(id, step)
		}
		if persistenceErr != nil {
			cancel()
		}
	})
	result, err := agent.Run(runCtx, task.Prompt)
	if persistenceErr != nil {
		return nil, persistenceErr
	}
	if saveErr := m.CompleteTask(id, result, err); saveErr != nil {
		return nil, saveErr
	}
	task, _ = m.GetTask(id)
	return task, err
}

func (m *Manager) RunImmediate(ctx context.Context, prompt string, config *AgentConfig) (string, []Step, error) {
	id := fmt.Sprintf("immediate-%d", time.Now().UnixNano())
	if _, err := m.CreateTask(id, prompt, config); err != nil {
		return "", nil, err
	}
	task, err := m.RunTask(ctx, id)
	if task == nil {
		return "", nil, err
	}
	return task.Result, task.Steps, err
}

// GetTask and ListTasks return detached snapshots; callers cannot mutate storage
// or race with execution through a returned pointer.
func (m *Manager) GetTask(id string) (*Task, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	task, ok := m.tasks[id]
	if !ok || task.DeletedAt != nil {
		return nil, false
	}
	copy, err := copyTask(task)
	return copy, err == nil
}

func (m *Manager) ListTasks() []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tasks := make([]*Task, 0, len(m.tasks))
	for _, task := range m.tasks {
		if task.DeletedAt != nil {
			continue
		}
		if copy, err := copyTask(task); err == nil {
			tasks = append(tasks, copy)
		}
	}
	return tasks
}

func (m *Manager) CancelTask(id string) error {
	_, err := m.updateTask(id, func(task *Task) error {
		if task.Status != TaskRunning && task.Status != TaskWaiting && task.Status != TaskPending && task.Status != TaskInterrupted {
			return ErrRunConflict
		}
		if task.cancel != nil {
			task.cancel()
		}
		finishTask(task, TaskCancelled, "Cancelled by user.")
		return nil
	})
	return err
}

func (m *Manager) DeleteTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.deleteTaskLocked(id)
}

// Only terminal, reconciled runs or interrupted records with no recovery state
// can be removed. A persisted tombstone prevents
// the event projection resurrecting history after restart; it retains no prompt,
// response, arguments, progress or checkpoint. Audit logs/backups are separate.
func CanDeleteTask(task *Task) bool {
	return task != nil && task.DeletedAt == nil && task.PendingApproval == nil &&
		(task.Checkpoint == nil || task.Checkpoint.ExecutingCall == "") &&
		(task.Status == TaskCompleted || task.Status == TaskFailed || task.Status == TaskCancelled ||
			(task.Status == TaskInterrupted && task.Checkpoint == nil))
}

func (m *Manager) IsDeleted(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	task := m.tasks[id]
	return task != nil && task.DeletedAt != nil
}

func (m *Manager) deleteTaskLocked(id string) error {
	task, exists := m.tasks[id]
	if !exists || task.DeletedAt != nil {
		return ErrTaskNotFound
	}
	if !CanDeleteTask(task) {
		return ErrRunConflict
	}
	now := time.Now().UTC()
	tombstone := &Task{ID: task.ID, Actor: task.Actor, Status: task.Status, CreatedAt: task.CreatedAt, DeletedAt: &now}
	if err := m.saveTask(tombstone); err != nil {
		return err
	}
	m.tasks[id] = tombstone
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

// ============================================================================
// Task Persistence
// ============================================================================

// ClearHistory clears all completed/failed tasks
func (m *Manager) ClearHistory() (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.storageErr != nil {
		return 0, fmt.Errorf("%w: %v", ErrRunStorage, m.storageErr)
	}

	count := 0
	for id, task := range m.tasks {
		if CanDeleteTask(task) {
			if err := m.deleteTaskLocked(id); err != nil {
				return count, err
			}
			count++
		}
	}
	return count, nil
}
