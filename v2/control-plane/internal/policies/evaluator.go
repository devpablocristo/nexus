package policies

import (
	"fmt"
	"strings"
	"sync"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
)

const defaultProgramCacheSize = 256

type Evaluator struct {
	env         *cel.Env
	envErr      error
	mu          sync.Mutex
	programs    map[string]cel.Program
	order       []string
	maxPrograms int
}

func NewEvaluator() *Evaluator {
	return newEvaluatorWithLimit(defaultProgramCacheSize)
}

func newEvaluatorWithLimit(limit int) *Evaluator {
	if limit <= 0 {
		limit = defaultProgramCacheSize
	}

	env, err := cel.NewEnv(
		cel.Variable("action", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("resource", cel.MapType(cel.StringType, cel.DynType)),
	)

	return &Evaluator{
		env:         env,
		envErr:      err,
		programs:    make(map[string]cel.Program),
		maxPrograms: limit,
	}
}

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

func (e *Evaluator) Matches(expression string, action, resource map[string]any) (bool, error) {
	if strings.TrimSpace(expression) == "" {
		return true, nil
	}

	program, err := e.program(expression, nil)
	if err != nil {
		return false, err
	}

	result, _, err := program.Eval(map[string]any{
		"action":   action,
		"resource": resource,
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
	if e.envErr != nil {
		return nil, fmt.Errorf("create CEL env: %w", e.envErr)
	}
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
