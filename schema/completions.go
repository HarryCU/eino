/*
 * Copyright 2024 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package schema

import (
	"context"
	"fmt"
	"github.com/slongfield/pyfmt"
	"strings"
	"text/template"
)

// CompletionsRoleType is the type of the role of a completions message.
type CompletionsRoleType string

const (
	// Completion is the role of completion code, means the message is returned by CodeCompletionsMessage.
	Completion CompletionsRoleType = "completion"
	// Guess is the role of guess code, means the message is a user message.
	Guess CompletionsRoleType = "guess"
)

type CodeCompletionsMessage struct {
	Role CompletionsRoleType `json:"role"`

	Prompt string  `json:"prompt"`
	Suffix *string `json:"suffix,omitempty"`

	Choices *[]CompletionsChoice   `json:"choices,omitempty"`
	Usage   *CompletionsTokenUsage `json:"usage,omitempty"`

	// customized information for model implementation
	Extra map[string]any `json:"extra,omitempty"`
}

type CompletionsChoice struct {
	Text         string                     `json:"text"`
	Index        int                        `json:"index"`
	FinishReason string                     `json:"finish_reason"`
	Logprobs     *CompletionsChoiceLogprobs `json:"logprobs,omitempty"`
}

type CompletionsChoiceLogprobs struct {
	Tokens        interface{} `json:"tokens,omitempty"`
	TokenLogprobs interface{} `json:"token_logprobs,omitempty"`
	TopLogprobs   interface{} `json:"top_logprobs,omitempty"`
	TextOffset    interface{} `json:"text_offset,omitempty"`
}

type CompletionsTokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

var _ CompletionsTemplate = &CodeCompletionsMessage{}

type CompletionsTemplate interface {
	Format(ctx context.Context, template string, formatType FormatType) (*CodeCompletionsMessage, error)
}

func formatCompletionsPrompt(content string, vs map[string]any, formatType FormatType) (string, error) {
	switch formatType {
	case FString:
		return pyfmt.Fmt(content, vs)
	case GoTemplate:
		parsedTmpl, err := template.New("template").
			Option("missingkey=error").
			Parse(content)
		if err != nil {
			return "", err
		}
		sb := new(strings.Builder)
		err = parsedTmpl.Execute(sb, vs)
		if err != nil {
			return "", err
		}
		return sb.String(), nil
	case Jinja2:
		env, err := getJinjaEnv()
		if err != nil {
			return "", err
		}
		tpl, err := env.FromString(content)
		if err != nil {
			return "", err
		}
		out, err := tpl.Execute(vs)
		if err != nil {
			return "", err
		}
		return out, nil
	default:
		return "", fmt.Errorf("unknown format type: %v", formatType)
	}
}

func (m *CodeCompletionsMessage) Format(_ context.Context, template string, formatType FormatType) (*CodeCompletionsMessage, error) {
	vs := map[string]any{
		"prefix": m.Prompt,
		"suffix": *m.Suffix,
	}
	c, err := formatCompletionsPrompt(template, vs, formatType)
	if err != nil {
		return nil, err
	}

	copied := *m
	copied.Prompt = c
	copied.Suffix = nil
	return &copied, nil
}
