package tools

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"strconv"
	"strings"
)

type Calculator struct{}

func (Calculator) Spec() Spec {
	return Spec{
		Name:        "calculator",
		Description: "Evaluate an arithmetic expression with numbers, parentheses, +, -, *, and /. Use this for calculations instead of mental arithmetic.",
		Parameters: Schema{
			Type: "object", Properties: map[string]Property{
				"expression": {Type: "string", Description: "Arithmetic expression, for example (12+8)/4"},
			}, Required: []string{"expression"},
		},
	}
}

func (Calculator) Execute(_ context.Context, _ Invocation, args map[string]any) (any, error) {
	expression := strings.TrimSpace(args["expression"].(string))
	if expression == "" || len(expression) > 128 {
		return nil, errors.New("expression must contain 1 to 128 characters")
	}
	parsed, err := parser.ParseExpr(expression)
	if err != nil {
		return nil, errors.New("invalid arithmetic expression")
	}
	result, err := evaluate(parsed, 0)
	if err != nil {
		return nil, err
	}
	if math.IsInf(result, 0) || math.IsNaN(result) {
		return nil, errors.New("non-finite arithmetic result")
	}
	return map[string]any{"expression": expression, "result": result}, nil
}

func evaluate(node ast.Expr, depth int) (float64, error) {
	if depth > 32 {
		return 0, errors.New("expression is too deeply nested")
	}
	switch value := node.(type) {
	case *ast.ParenExpr:
		return evaluate(value.X, depth+1)
	case *ast.BasicLit:
		if value.Kind != token.INT && value.Kind != token.FLOAT {
			return 0, errors.New("only numeric literals are allowed")
		}
		number, err := strconv.ParseFloat(value.Value, 64)
		if err != nil || math.IsInf(number, 0) {
			return 0, errors.New("invalid number")
		}
		return number, nil
	case *ast.UnaryExpr:
		number, err := evaluate(value.X, depth+1)
		if err != nil {
			return 0, err
		}
		switch value.Op {
		case token.ADD:
			return number, nil
		case token.SUB:
			return -number, nil
		default:
			return 0, errors.New("unsupported unary operator")
		}
	case *ast.BinaryExpr:
		left, err := evaluate(value.X, depth+1)
		if err != nil {
			return 0, err
		}
		right, err := evaluate(value.Y, depth+1)
		if err != nil {
			return 0, err
		}
		switch value.Op {
		case token.ADD:
			return left + right, nil
		case token.SUB:
			return left - right, nil
		case token.MUL:
			return left * right, nil
		case token.QUO:
			if right == 0 {
				return 0, errors.New("division by zero")
			}
			return left / right, nil
		default:
			return 0, errors.New("unsupported arithmetic operator")
		}
	default:
		return 0, fmt.Errorf("unsupported expression element %T", node)
	}
}
