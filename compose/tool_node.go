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

package compose

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/internal/safe"
	"github.com/cloudwego/eino/schema"
)

type toolsNodeOptions struct {
	ToolOptions []tool.Option
	ToolList    []tool.BaseTool
}

// ToolsNodeOption is the option func type for ToolsNode.
type ToolsNodeOption func(o *toolsNodeOptions)

// WithToolOption adds tool options to the ToolsNode.
func WithToolOption(opts ...tool.Option) ToolsNodeOption {
	return func(o *toolsNodeOptions) {
		o.ToolOptions = append(o.ToolOptions, opts...)
	}
}

// WithToolList sets the tool list for the ToolsNode.
func WithToolList(tool ...tool.BaseTool) ToolsNodeOption {
	return func(o *toolsNodeOptions) {
		o.ToolList = tool
	}
}

// ToolsNode a node that can run tools in a graph. the interface in Graph Node as below:
//
//	Invoke(ctx context.Context, input *schema.Message, opts ...ToolsNodeOption) ([]*schema.Message, error)
//	Stream(ctx context.Context, input *schema.Message, opts ...ToolsNodeOption) (*schema.StreamReader[[]*schema.Message], error)
type ToolsNode struct {
	tuple *toolsTuple
}

// ToolsNodeConfig is the config for ToolsNode. It requires a list of tools.
// Tools are BaseTool but must implement InvokableTool or StreamableTool.
type ToolsNodeConfig struct {
	Tools []tool.BaseTool
}

// NewToolNode creates a new ToolsNode.
// e.g.
//
//	conf := &ToolsNodeConfig{
//		Tools: []tool.BaseTool{invokableTool1, streamableTool2},
//	}
//	toolsNode, err := NewToolNode(ctx, conf)
func NewToolNode(ctx context.Context, conf *ToolsNodeConfig) (*ToolsNode, error) {
	tuple, err := convTools(ctx, conf.Tools)
	if err != nil {
		return nil, err
	}

	return &ToolsNode{
		tuple: tuple,
	}, nil
}

type toolsTuple struct {
	indexes map[string]int
	meta    []*executorMeta
	rps     []*runnablePacker[string, string, tool.Option]
}

func convTools(ctx context.Context, tools []tool.BaseTool) (*toolsTuple, error) {
	ret := &toolsTuple{
		indexes: make(map[string]int),
		meta:    make([]*executorMeta, len(tools)),
		rps:     make([]*runnablePacker[string, string, tool.Option], len(tools)),
	}
	for idx, bt := range tools {
		tl, err := bt.Info(ctx)
		if err != nil {
			return nil, fmt.Errorf("(NewToolNode) failed to get tool info at idx= %d: %w", idx, err)
		}

		toolName := tl.Name

		var (
			st tool.StreamableTool
			it tool.InvokableTool

			invokable  func(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error)
			streamable func(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (*schema.StreamReader[string], error)

			ok   bool
			meta *executorMeta
		)

		if st, ok = bt.(tool.StreamableTool); ok {
			streamable = st.StreamableRun
		}

		if it, ok = bt.(tool.InvokableTool); ok {
			invokable = it.InvokableRun
		}

		if st == nil && it == nil {
			return nil, fmt.Errorf("tool %s is not invokable or streamable", toolName)
		}

		if st != nil {
			meta = parseExecutorInfoFromComponent(components.ComponentOfTool, st)
		} else {
			meta = parseExecutorInfoFromComponent(components.ComponentOfTool, it)
		}

		ret.indexes[toolName] = idx
		ret.meta[idx] = meta
		ret.rps[idx] = newRunnablePacker(invokable, streamable,
			nil, nil, !meta.isComponentCallbackEnabled)
	}
	return ret, nil
}

type toolCallTask struct {
	// in
	r      *runnablePacker[string, string, tool.Option]
	meta   *executorMeta
	name   string
	arg    string
	callID string

	// out
	output  string
	sOutput *schema.StreamReader[string]
	err     error
}

func genToolCallTasks(tuple *toolsTuple, input *schema.Message) ([]toolCallTask, error) {
	if input.Role != schema.Assistant {
		return nil, fmt.Errorf("expected message role is Assistant, got %s", input.Role)
	}

	n := len(input.ToolCalls)
	if n == 0 {
		return nil, errors.New("no tool call found in input message")
	}

	toolCallTasks := make([]toolCallTask, n)

	for i := 0; i < n; i++ {
		toolCall := input.ToolCalls[i]
		index, ok := tuple.indexes[toolCall.Function.Name]
		if !ok {
			return nil, fmt.Errorf("tool %s not found in toolsNode indexes", toolCall.Function.Name)
		}

		toolCallTasks[i].r = tuple.rps[index]
		toolCallTasks[i].meta = tuple.meta[index]
		toolCallTasks[i].name = toolCall.Function.Name
		toolCallTasks[i].arg = toolCall.Function.Arguments
		toolCallTasks[i].callID = toolCall.ID
	}

	return toolCallTasks, nil
}

func runToolCallTaskByInvoke(ctx context.Context, task *toolCallTask, opts ...tool.Option) {
	ctx = callbacks.ReuseHandlers(ctx, &callbacks.RunInfo{
		Name:      task.name,
		Type:      task.meta.componentImplType,
		Component: task.meta.component,
	})

	ctx = setToolCallInfo(ctx, &toolCallInfo{toolCallID: task.callID})

	task.output, task.err = task.r.Invoke(ctx, task.arg, opts...) // nolint: byted_returned_err_should_do_check
}

func runToolCallTaskByStream(ctx context.Context, task *toolCallTask, opts ...tool.Option) {
	ctx = callbacks.ReuseHandlers(ctx, &callbacks.RunInfo{
		Name:      task.name,
		Type:      task.meta.componentImplType,
		Component: task.meta.component,
	})

	ctx = setToolCallInfo(ctx, &toolCallInfo{toolCallID: task.callID})

	task.sOutput, task.err = task.r.Stream(ctx, task.arg, opts...) // nolint: byted_returned_err_should_do_check
}

func parallelRunToolCall(ctx context.Context,
	run func(ctx2 context.Context, callTask *toolCallTask, opts ...tool.Option), tasks []toolCallTask, opts ...tool.Option) {

	if len(tasks) == 1 {
		run(ctx, &tasks[0], opts...)
		return
	}

	var wg sync.WaitGroup
	for i := 1; i < len(tasks); i++ {
		wg.Add(1)
		go func(ctx_ context.Context, t *toolCallTask, opts ...tool.Option) {
			defer wg.Done()
			defer func() {
				panicErr := recover()
				if panicErr != nil {
					t.err = safe.NewPanicErr(panicErr, debug.Stack()) // nolint: byted_returned_err_should_do_check
				}
			}()
			run(ctx_, t, opts...)
		}(ctx, &tasks[i], opts...)
	}

	run(ctx, &tasks[0], opts...)
	wg.Wait()
}

// Invoke calls the tools and collects the results of invokable tools.
// it's parallel if there are multiple tool calls in the input message.
func (tn *ToolsNode) Invoke(ctx context.Context, input *schema.Message,
	opts ...ToolsNodeOption) ([]*schema.Message, error) {

	opt := getToolsNodeOptions(opts...)
	tuple := tn.tuple
	if opt.ToolList != nil {
		var err error
		tuple, err = convTools(ctx, opt.ToolList)
		if err != nil {
			return nil, fmt.Errorf("failed to convert tool list from call option: %w", err)
		}
	}

	tasks, err := genToolCallTasks(tuple, input)
	if err != nil {
		return nil, err
	}

	parallelRunToolCall(ctx, runToolCallTaskByInvoke, tasks, opt.ToolOptions...)

	n := len(tasks)
	output := make([]*schema.Message, n)
	for i := 0; i < n; i++ {
		if tasks[i].err != nil {
			return nil, fmt.Errorf("failed to invoke tool call %s: %w", tasks[i].callID, tasks[i].err)
		}

		output[i] = schema.ToolMessage(tasks[i].output, tasks[i].callID)
	}

	return output, nil
}

// Stream calls the tools and collects the results of stream readers.
// it's parallel if there are multiple tool calls in the input message.
func (tn *ToolsNode) Stream(ctx context.Context, input *schema.Message,
	opts ...ToolsNodeOption) (*schema.StreamReader[[]*schema.Message], error) {

	opt := getToolsNodeOptions(opts...)
	tuple := tn.tuple
	if opt.ToolList != nil {
		var err error
		tuple, err = convTools(ctx, opt.ToolList)
		if err != nil {
			return nil, fmt.Errorf("failed to convert tool list from call option: %w", err)
		}
	}

	tasks, err := genToolCallTasks(tuple, input)
	if err != nil {
		return nil, err
	}

	parallelRunToolCall(ctx, runToolCallTaskByStream, tasks, opt.ToolOptions...)

	n := len(tasks)
	sOutput := make([]*schema.StreamReader[[]*schema.Message], n)

	for i := 0; i < n; i++ {
		if tasks[i].err != nil {
			return nil, fmt.Errorf("failed to stream tool call %s: %w", tasks[i].callID, tasks[i].err)
		}

		index := i
		callID := tasks[i].callID
		convert := func(s string) ([]*schema.Message, error) {
			ret := make([]*schema.Message, n)
			ret[index] = schema.ToolMessage(s, callID)

			return ret, nil
		}

		sOutput[i] = schema.StreamReaderWithConvert(tasks[i].sOutput, convert)
	}

	return schema.MergeStreamReaders(sOutput), nil
}

func (tn *ToolsNode) GetType() string {
	return ""
}

func getToolsNodeOptions(opts ...ToolsNodeOption) *toolsNodeOptions {
	o := &toolsNodeOptions{
		ToolOptions: make([]tool.Option, 0),
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

type toolCallInfoKey struct{}
type toolCallInfo struct {
	toolCallID string
}

func setToolCallInfo(ctx context.Context, toolCallInfo *toolCallInfo) context.Context {
	return context.WithValue(ctx, toolCallInfoKey{}, toolCallInfo)
}

// GetToolCallID gets the current tool call id from the context.
func GetToolCallID(ctx context.Context) string {
	v := ctx.Value(toolCallInfoKey{})
	if v == nil {
		return ""
	}

	info, ok := v.(*toolCallInfo)
	if !ok {
		return ""
	}

	return info.toolCallID
}
