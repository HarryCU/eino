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

package fim

import (
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/schema"
)

// CallbackInput is the input for the callback.
type CallbackInput struct {
	// Templates is the templates for the callback.
	Templates []schema.CompletionsTemplate
	// Extra is the extra information for the callback.
	Extra map[string]any
}

// CallbackOutput is the output for the callback.
type CallbackOutput struct {
	// Result is the result for the callback.
	Result []*schema.CodeCompletionsMessage
	// Templates is the templates for the callback.
	Templates []schema.CompletionsTemplate
	// Extra is the extra information for the callback.
	Extra map[string]any
}

// ConvCallbackInput converts the callback input to the prompt callback input.
func ConvCallbackInput(src callbacks.CallbackInput) *CallbackInput {
	switch t := src.(type) {
	case *CallbackInput:
		return t
	default:
		return nil
	}
}

// ConvCallbackOutput converts the callback output to the prompt callback output.
func ConvCallbackOutput(src callbacks.CallbackOutput) *CallbackOutput {
	switch t := src.(type) {
	case *CallbackOutput:
		return t
	case []*schema.CodeCompletionsMessage:
		return &CallbackOutput{
			Result: t,
		}
	default:
		return nil
	}
}
