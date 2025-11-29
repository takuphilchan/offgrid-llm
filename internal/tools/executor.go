package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// Executor executes built-in tools
type Executor struct {
	ragSearchFunc func(ctx context.Context, query string, topK int) (string, error)
}

// NewExecutor creates a new tool executor
func NewExecutor() *Executor {
	return &Executor{}
}

// SetRAGSearchFunc sets the function to use for RAG searches
func (e *Executor) SetRAGSearchFunc(fn func(ctx context.Context, query string, topK int) (string, error)) {
	e.ragSearchFunc = fn
}

// Execute executes a tool call and returns the result
func (e *Executor) Execute(ctx context.Context, toolName, arguments string) (string, error) {
	var args map[string]interface{}
	if arguments != "" {
		if err := json.Unmarshal([]byte(arguments), &args); err != nil {
			return "", fmt.Errorf("invalid arguments JSON: %w", err)
		}
	}

	switch toolName {
	case "get_current_time":
		return e.executeGetCurrentTime()
	case "calculate":
		return e.executeCalculate(args)
	case "search_documents":
		return e.executeSearchDocuments(ctx, args)
	default:
		return "", fmt.Errorf("unknown tool: %s", toolName)
	}
}

func (e *Executor) executeGetCurrentTime() (string, error) {
	now := time.Now()
	return fmt.Sprintf(`{"date": "%s", "time": "%s", "timezone": "%s", "unix": %d}`,
		now.Format("2006-01-02"),
		now.Format("15:04:05"),
		now.Location().String(),
		now.Unix(),
	), nil
}

func (e *Executor) executeCalculate(args map[string]interface{}) (string, error) {
	expr, ok := args["expression"].(string)
	if !ok {
		return "", fmt.Errorf("expression is required")
	}

	result, err := evaluateExpression(expr)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(`{"expression": "%s", "result": %v}`, expr, result), nil
}

func (e *Executor) executeSearchDocuments(ctx context.Context, args map[string]interface{}) (string, error) {
	if e.ragSearchFunc == nil {
		return "", fmt.Errorf("RAG search not available")
	}

	query, ok := args["query"].(string)
	if !ok {
		return "", fmt.Errorf("query is required")
	}

	topK := 5
	if tk, ok := args["top_k"].(float64); ok {
		topK = int(tk)
	}

	return e.ragSearchFunc(ctx, query, topK)
}

// Simple expression evaluator for basic math
func evaluateExpression(expr string) (float64, error) {
	expr = strings.TrimSpace(expr)
	expr = strings.ToLower(expr)

	// Handle common math functions
	if strings.HasPrefix(expr, "sqrt(") && strings.HasSuffix(expr, ")") {
		inner := expr[5 : len(expr)-1]
		val, err := strconv.ParseFloat(inner, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid number: %s", inner)
		}
		return math.Sqrt(val), nil
	}

	if strings.HasPrefix(expr, "abs(") && strings.HasSuffix(expr, ")") {
		inner := expr[4 : len(expr)-1]
		val, err := strconv.ParseFloat(inner, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid number: %s", inner)
		}
		return math.Abs(val), nil
	}

	if strings.HasPrefix(expr, "pow(") && strings.HasSuffix(expr, ")") {
		inner := expr[4 : len(expr)-1]
		parts := strings.Split(inner, ",")
		if len(parts) != 2 {
			return 0, fmt.Errorf("pow requires two arguments")
		}
		base, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		if err != nil {
			return 0, fmt.Errorf("invalid base: %s", parts[0])
		}
		exp, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err != nil {
			return 0, fmt.Errorf("invalid exponent: %s", parts[1])
		}
		return math.Pow(base, exp), nil
	}

	// Simple arithmetic evaluation
	return evaluateArithmetic(expr)
}

// Evaluate simple arithmetic expressions
func evaluateArithmetic(expr string) (float64, error) {
	expr = strings.ReplaceAll(expr, " ", "")

	// Handle parentheses
	for strings.Contains(expr, "(") {
		start := strings.LastIndex(expr, "(")
		end := strings.Index(expr[start:], ")") + start
		if end <= start {
			return 0, fmt.Errorf("mismatched parentheses")
		}
		inner := expr[start+1 : end]
		result, err := evaluateArithmetic(inner)
		if err != nil {
			return 0, err
		}
		expr = expr[:start] + fmt.Sprintf("%f", result) + expr[end+1:]
	}

	// Handle addition and subtraction (lowest precedence)
	for i := len(expr) - 1; i >= 0; i-- {
		if (expr[i] == '+' || expr[i] == '-') && i > 0 {
			left, err := evaluateArithmetic(expr[:i])
			if err != nil {
				continue // Might be a negative number
			}
			right, err := evaluateArithmetic(expr[i+1:])
			if err != nil {
				return 0, err
			}
			if expr[i] == '+' {
				return left + right, nil
			}
			return left - right, nil
		}
	}

	// Handle multiplication and division
	for i := len(expr) - 1; i >= 0; i-- {
		if expr[i] == '*' || expr[i] == '/' {
			left, err := evaluateArithmetic(expr[:i])
			if err != nil {
				return 0, err
			}
			right, err := evaluateArithmetic(expr[i+1:])
			if err != nil {
				return 0, err
			}
			if expr[i] == '*' {
				return left * right, nil
			}
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			return left / right, nil
		}
	}

	// Handle modulo
	for i := len(expr) - 1; i >= 0; i-- {
		if expr[i] == '%' {
			left, err := evaluateArithmetic(expr[:i])
			if err != nil {
				return 0, err
			}
			right, err := evaluateArithmetic(expr[i+1:])
			if err != nil {
				return 0, err
			}
			return math.Mod(left, right), nil
		}
	}

	// Handle power (^)
	for i := len(expr) - 1; i >= 0; i-- {
		if expr[i] == '^' {
			left, err := evaluateArithmetic(expr[:i])
			if err != nil {
				return 0, err
			}
			right, err := evaluateArithmetic(expr[i+1:])
			if err != nil {
				return 0, err
			}
			return math.Pow(left, right), nil
		}
	}

	// Parse as number
	return strconv.ParseFloat(expr, 64)
}
