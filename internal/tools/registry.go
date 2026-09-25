package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
)

type Invocation struct {
	UserID    string
	SessionID string
}

type Property struct {
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Enum        []string `json:"enum,omitempty"`
}

type Schema struct {
	Type                 string              `json:"type"`
	Properties           map[string]Property `json:"properties"`
	Required             []string            `json:"required"`
	AdditionalProperties bool                `json:"additionalProperties"`
}

type Spec struct {
	Name        string
	Description string
	Parameters  Schema
}

type Tool interface {
	Spec() Spec
	Execute(context.Context, Invocation, map[string]any) (any, error)
}

type Registry struct {
	byName map[string]Tool
	order  []string
}

func NewRegistry() *Registry { return &Registry{byName: make(map[string]Tool)} }

func (r *Registry) Register(tool Tool) error {
	if tool == nil {
		return errors.New("tool must not be nil")
	}
	spec := tool.Spec()
	if strings.TrimSpace(spec.Name) == "" || strings.TrimSpace(spec.Description) == "" {
		return errors.New("tool name and description are required")
	}
	if _, exists := r.byName[spec.Name]; exists {
		return fmt.Errorf("duplicate tool %q", spec.Name)
	}
	if spec.Parameters.Type != "object" || spec.Parameters.Properties == nil || spec.Parameters.AdditionalProperties {
		return fmt.Errorf("tool %q requires a strict object schema", spec.Name)
	}
	required := make(map[string]bool, len(spec.Parameters.Required))
	for _, name := range spec.Parameters.Required {
		if _, exists := spec.Parameters.Properties[name]; !exists || required[name] {
			return fmt.Errorf("tool %q has invalid required property %q", spec.Name, name)
		}
		required[name] = true
	}
	for name, property := range spec.Parameters.Properties {
		if !required[name] || property.Type != "string" {
			return fmt.Errorf("tool %q property %q must be a required string", spec.Name, name)
		}
	}
	r.byName[spec.Name] = tool
	r.order = append(r.order, spec.Name)
	return nil
}

func (r *Registry) Get(name string) (Tool, bool) {
	tool, found := r.byName[name]
	return tool, found
}

func (r *Registry) Definitions() ([]json.RawMessage, error) {
	definitions := make([]json.RawMessage, 0, len(r.order))
	for _, name := range r.order {
		spec := r.byName[name].Spec()
		encoded, err := json.Marshal(struct {
			Type        string `json:"type"`
			Name        string `json:"name"`
			Description string `json:"description"`
			Parameters  Schema `json:"parameters"`
			Strict      bool   `json:"strict"`
		}{"function", spec.Name, spec.Description, spec.Parameters, true})
		if err != nil {
			return nil, fmt.Errorf("encode tool %q: %w", name, err)
		}
		definitions = append(definitions, encoded)
	}
	return definitions, nil
}

func (r *Registry) Names() []string {
	names := append([]string(nil), r.order...)
	sort.Strings(names)
	return names
}

func ValidateArguments(schema Schema, raw json.RawMessage) (map[string]any, error) {
	var args map[string]any
	if len(raw) == 0 || string(raw) == "null" {
		return nil, errors.New("arguments must be a JSON object")
	}
	if err := json.Unmarshal(raw, &args); err != nil || args == nil {
		return nil, errors.New("arguments must be a JSON object")
	}
	for name := range args {
		if _, exists := schema.Properties[name]; !exists {
			return nil, fmt.Errorf("unexpected argument %q", name)
		}
	}
	for _, name := range schema.Required {
		value, exists := args[name]
		if !exists {
			return nil, fmt.Errorf("missing argument %q", name)
		}
		property := schema.Properties[name]
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("argument %q must be a string", name)
		}
		if len(property.Enum) > 0 {
			allowed := false
			for _, option := range property.Enum {
				if text == option {
					allowed = true
					break
				}
			}
			if !allowed {
				return nil, fmt.Errorf("argument %q has an unsupported value", name)
			}
		}
	}
	return args, nil
}
