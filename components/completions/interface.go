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

package completions

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

// CodeCompletionsModel support openai.
// use Generate for completed output, use Stream as for stream output.
//
//go:generate  mockgen -destination ../../internal/mock/components/codecompletions/CodeCompletions_mock.go --package codecompletions -source interface.go
type CodeCompletionsModel interface {
	Generate(ctx context.Context, input *schema.CodeCompletionsMessage, opts ...Option) (*schema.CodeCompletionsMessage, error)
	Stream(ctx context.Context, input *schema.CodeCompletionsMessage, opts ...Option) (
		*schema.StreamReader[*schema.CodeCompletionsMessage], error)
}
