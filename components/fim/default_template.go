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
	"context"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/schema"
)

type DefaultTemplate struct {
	content string
	// templates is the templates for the completions template.
	templates []schema.CompletionsTemplate
	// formatType is the format type for the template.
	formatType schema.FormatType
}

func (t *DefaultTemplate) Format(ctx context.Context, _ ...Option) (result []*schema.CodeCompletionsMessage, err error) {

	defer func() {
		if err != nil {
			_ = callbacks.OnError(ctx, err)
		}
	}()

	ctx = callbacks.OnStart(ctx, &CallbackInput{
		Templates: t.templates,
	})

	result = make([]*schema.CodeCompletionsMessage, 0, len(t.templates))
	for _, template := range t.templates {
		msg, err := template.Format(ctx, t.content, t.formatType)
		if err != nil {
			return nil, err
		}

		result = append(result, msg)
	}

	_ = callbacks.OnEnd(ctx, &CallbackOutput{
		Result:    result,
		Templates: t.templates,
	})

	return result, nil
}
