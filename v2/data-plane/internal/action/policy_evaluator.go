package action

import (
	"fmt"
	"strings"
	"sync"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
)

type actionPolicyEvaluator interface {
	Matches(expression string, action, resource map[string]any) (bool, error)
}

type ActionPolicyEvaluator struct {
	env         *cel.Env
	envErr      error
	mu          sync.Mutex
	programs    map[string]cel.Program
	order       []string
	maxPrograms int
}

func NewActionPolicyEvaluator() *ActionPolicyEvaluator {
	env, err := cel.NewEnv(
		cel.Variable("action", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("resource", cel.MapType(cel.StringType, cel.DynType)),
	)
	return &ActionPolicyEvaluator{
		env:         env,
		envErr:      err,
		programs:    make(map[string]cel.Program),
		maxPrograms: 256,
	}
}

func (e *ActionPolicyEvaluator) Matches(expression string, action, resource map[string]any) (bool, error) {
	if strings.TrimSpace(expression) == "" {
		return true, nil
	}

	program, err := e.program(expression)
	if err != nil {
		return false, err
	}

	result, _, err := program.Eval(map[string]any{
		"action":   action,
		"resource": resource,
	})
	if err != nil {
		return false, fmt.Errorf("evaluate action policy expression: %w", err)
	}
	if result.Type() != types.BoolType {
		return false, fmt.Errorf("policy expression must return bool, got %s", result.Type())
	}
	matched, _ := result.Value().(bool)
	return matched, nil
}

func (e *ActionPolicyEvaluator) program(expression string) (cel.Program, error) {
	if e.envErr != nil {
		return nil, fmt.Errorf("create CEL env: %w", e.envErr)
	}
	e.mu.Lock()
	if cached, ok := e.programs[expression]; ok {
		e.mu.Unlock()
		return cached, nil
	}
	e.mu.Unlock()

	ast, issues := e.env.Compile(expression)
	if issues.Err() != nil {
		return nil, fmt.Errorf("compile action policy expression: %w", issues.Err())
	}
	if ast.OutputType() != cel.BoolType {
		return nil, fmt.Errorf("policy expression must return bool, got %s", ast.OutputType())
	}
	program, err := e.env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("build action policy program: %w", err)
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
