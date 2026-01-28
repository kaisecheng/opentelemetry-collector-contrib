// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package condition // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/condition"

import (
	"fmt"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type Action string

const (
	ActionDrop Action = "drop"
	ActionKeep Action = "keep"
)

func (a *Action) UnmarshalText(text []byte) error {
	str := Action(strings.ToLower(string(text)))
	switch str {
	case ActionDrop, ActionKeep:
		*a = str
		return nil
	default:
		return fmt.Errorf("unknown action %q, must be %q or %q", str, ActionDrop, ActionKeep)
	}
}

var _ ottl.ConditionsGetter = (*ContextConditions)(nil)

type ContextID string

const (
	Resource  ContextID = "resource"
	Scope     ContextID = "scope"
	Span      ContextID = "span"
	SpanEvent ContextID = "spanevent"
	Metric    ContextID = "metric"
	DataPoint ContextID = "datapoint"
	Log       ContextID = "log"
	Profile   ContextID = "profile"
)

func (c *ContextID) UnmarshalText(text []byte) error {
	str := ContextID(strings.ToLower(string(text)))
	switch str {
	case Resource, Scope, Span, SpanEvent, Metric, DataPoint, Log, Profile:
		*c = str
		return nil
	default:
		return fmt.Errorf("unknown context %v", str)
	}
}

// ContextConditions is a wrapper struct for OTTL conditions.
type ContextConditions struct {
	Context    ContextID `mapstructure:"context"`
	Conditions []string  `mapstructure:"conditions"`
	// ErrorMode determines how the processor reacts to errors that occur while processing
	// this group of conditions. When provided, it overrides the default Config ErrorMode.
	ErrorMode ottl.ErrorMode `mapstructure:"error_mode"`
	// Action determines what happens when conditions match. Valid values are `drop` and `keep`.
	Action Action `mapstructure:"action"`
}

func (c ContextConditions) GetConditions() []string {
	return c.Conditions
}

func toContextConditions(conditions any) (*ContextConditions, error) {
	contextConditions, ok := conditions.(ContextConditions)
	if !ok {
		return nil, fmt.Errorf("invalid context conditions type, expected: common.ContextConditions, got: %T", conditions)
	}
	return &contextConditions, nil
}

func getErrorMode[T any](pc *ottl.ParserCollection[T], contextConditions *ContextConditions) ottl.ErrorMode {
	errorMode := pc.ErrorMode
	if contextConditions.ErrorMode != "" {
		errorMode = contextConditions.ErrorMode
	}
	return errorMode
}
