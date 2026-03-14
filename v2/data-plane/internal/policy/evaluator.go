package policy

import (
	"fmt"
	"strings"
	"sync"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
)

const defaultProgramCacheSize = 256

// ToolAttributes exposes tool fields to policy conditions.
type ToolAttributes struct {
	Name   string
	Kind   string
	Method string
	URL    string
}

// Evaluator evaluates policy conditions against run input, context and tool data.
type Evaluator struct {
	env         *cel.Env
	mu          sync.Mutex
	programs    map[string]cel.Program
	order       []string
	maxPrograms int
}

// NewEvaluator builds a policy evaluator.
func NewEvaluator() *Evaluator {
	return newEvaluatorWithLimit(defaultProgramCacheSize)
}

func newEvaluatorWithLimit(limit int) *Evaluator {
	if limit <= 0 {
		limit = defaultProgramCacheSize
	}

	env, err := cel.NewEnv(
		cel.Variable("input", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("context", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("tool", cel.MapType(cel.StringType, cel.DynType)),
	)
	if err != nil {
		panic(fmt.Sprintf("create CEL env: %v", err))
	}

	return &Evaluator{
		env:         env,
		programs:    make(map[string]cel.Program),
		maxPrograms: limit,
	}
}

// Validate checks that an expression is a valid CEL boolean expression.
func (e *Evaluator) Validate(expression string) error {
	if strings.TrimSpace(expression) == "" {
		return nil
	}

	ast, err := e.compile(expression)
	if err != nil {
		return err
	}
	if ast.OutputType() != cel.BoolType {
		return fmt.Errorf("policy expression must return bool, got %s", ast.OutputType())
	}
	if _, err := e.program(expression, ast); err != nil {
		return err
	}
	return nil
}

// Matches reports whether the provided expression matches.
func (e *Evaluator) Matches(expression string, input, context map[string]any, tool ToolAttributes) (bool, error) {
	if strings.TrimSpace(expression) == "" {
		return true, nil
	}

	program, err := e.program(expression, nil)
	if err != nil {
		return false, err
	}

	result, _, err := program.Eval(map[string]any{
		"input":   input,
		"context": context,
		"tool": map[string]any{
			"name":   tool.Name,
			"kind":   tool.Kind,
			"method": tool.Method,
			"url":    tool.URL,
		},
	})
	if err != nil {
		return false, fmt.Errorf("evaluate policy expression: %w", err)
	}

	if result.Type() != types.BoolType {
		return false, fmt.Errorf("policy expression must return bool, got %s", result.Type())
	}

	matched, _ := result.Value().(bool)
	return matched, nil
}

func (e *Evaluator) compile(expression string) (*cel.Ast, error) {
	ast, issues := e.env.Compile(expression)
	if issues.Err() != nil {
		return nil, fmt.Errorf("compile policy expression: %w", issues.Err())
	}
	return ast, nil
}

func (e *Evaluator) program(expression string, ast *cel.Ast) (cel.Program, error) {
	e.mu.Lock()
	if cached, ok := e.programs[expression]; ok {
		e.mu.Unlock()
		return cached, nil
	}
	e.mu.Unlock()

	if ast == nil {
		compiled, err := e.compile(expression)
		if err != nil {
			return nil, err
		}
		ast = compiled
	}

	program, err := e.env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("build policy program: %w", err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if cached, ok := e.programs[expression]; ok {
		return cached, nil
	}

	e.programs[expression] = program
	e.order = append(e.order, expression)
	if len(e.order) > e.maxPrograms {
		oldest := e.order[0]
		e.order = e.order[1:]
		delete(e.programs, oldest)
	}

	return program, nil
}
