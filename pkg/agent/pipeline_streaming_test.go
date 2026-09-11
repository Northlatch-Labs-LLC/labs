package agent

import (
	"context"
	"testing"

	"github.com/Northlatch-Labs-LLC/labs/pkg/bus"
	"github.com/Northlatch-Labs-LLC/labs/pkg/providers"
)

type configuredStreamingProvider struct {
	chatCalls    int
	streamCalls  int
	eventCalls   int
	chatModels   []string
	streamModels []string

	chatResponse *providers.LLMResponse
	streamPlan   []configuredStreamingCall
	eventPlan    []configuredStreamingEventCall
}

type configuredStreamingCall struct {
	chunks   []string
	response *providers.LLMResponse
	err      error
}

type configuredStreamingEventCall struct {
	chunks   []providers.StreamChunk
	response *providers.LLMResponse
	err      error
}

func (p *configuredStreamingProvider) Chat(
	ctx context.Context,
	messages []providers.Message,
	tools []providers.ToolDefinition,
	model string,
	opts map[string]any,
) (*providers.LLMResponse, error) {
	p.chatCalls++
	p.chatModels = append(p.chatModels, model)
	if p.chatResponse != nil {
		return p.chatResponse, nil
	}
	return &providers.LLMResponse{Content: "chat response"}, nil
}

func (p *configuredStreamingProvider) ChatStream(
	ctx context.Context,
	messages []providers.Message,
	tools []providers.ToolDefinition,
	model string,
	opts map[string]any,
	onChunk func(accumulated string),
) (*providers.LLMResponse, error) {
	p.streamCalls++
	p.streamModels = append(p.streamModels, model)
	var plan configuredStreamingCall
	if len(p.streamPlan) >= p.streamCalls {
		plan = p.streamPlan[p.streamCalls-1]
	}
	for _, chunk := range plan.chunks {
		onChunk(chunk)
	}
	if plan.err != nil {
		return nil, plan.err
	}
	if plan.response != nil {
		return plan.response, nil
	}
	return &providers.LLMResponse{Content: "stream response"}, nil
}

func (p *configuredStreamingProvider) ChatStreamEvents(
	ctx context.Context,
	messages []providers.Message,
	tools []providers.ToolDefinition,
	model string,
	opts map[string]any,
	onChunk func(providers.StreamChunk),
) (*providers.LLMResponse, error) {
	p.eventCalls++
	p.streamCalls++
	p.streamModels = append(p.streamModels, model)
	var plan configuredStreamingEventCall
	if len(p.eventPlan) >= p.eventCalls {
		plan = p.eventPlan[p.eventCalls-1]
	} else if len(p.streamPlan) >= p.eventCalls {
		legacyPlan := p.streamPlan[p.eventCalls-1]
		plan.response = legacyPlan.response
		plan.err = legacyPlan.err
		for _, chunk := range legacyPlan.chunks {
			plan.chunks = append(plan.chunks, providers.StreamChunk{Content: chunk})
		}
	}
	for _, chunk := range plan.chunks {
		onChunk(chunk)
	}
	if plan.err != nil {
		return nil, plan.err
	}
	if plan.response != nil {
		return plan.response, nil
	}
	return &providers.LLMResponse{Content: "stream response"}, nil
}

func (p *configuredStreamingProvider) GetDefaultModel() string {
	return "mock-model"
}

type configuredStreamingChatOnlyProvider struct {
	chatCalls int
}

func (p *configuredStreamingChatOnlyProvider) Chat(
	ctx context.Context,
	messages []providers.Message,
	tools []providers.ToolDefinition,
	model string,
	opts map[string]any,
) (*providers.LLMResponse, error) {
	p.chatCalls++
	return &providers.LLMResponse{Content: "chat only"}, nil
}

func (p *configuredStreamingChatOnlyProvider) GetDefaultModel() string {
	return "mock-model"
}

type configuredStreamingDelegate struct {
	streamer bus.Streamer
}

func (d configuredStreamingDelegate) GetStreamer(
	ctx context.Context,
	channel, chatID, sessionKey string,
) (bus.Streamer, bool) {
	if d.streamer == nil {
		return nil, false
	}
	return d.streamer, true
}

type recordingStreamer struct {
	updates            []string
	finalized          []string
	reasoningUpdates   []string
	reasoningFinalized []string
	events             []string
	canceled           int
}

func (s *recordingStreamer) Update(ctx context.Context, content string) error {
	s.updates = append(s.updates, content)
	s.events = append(s.events, "content:"+content)
	return nil
}

func (s *recordingStreamer) Finalize(ctx context.Context, content string) error {
	s.finalized = append(s.finalized, content)
	s.events = append(s.events, "final:"+content)
	return nil
}

func (s *recordingStreamer) UpdateReasoning(ctx context.Context, content string) error {
	s.reasoningUpdates = append(s.reasoningUpdates, content)
	s.events = append(s.events, "reasoning:"+content)
	return nil
}

func (s *recordingStreamer) FinalizeReasoning(ctx context.Context, content string) error {
	s.reasoningFinalized = append(s.reasoningFinalized, content)
	s.events = append(s.events, "reasoning-final:"+content)
	return nil
}

func (s *recordingStreamer) Cancel(context.Context) {
	s.canceled++
}

type cleanableRecordingStreamer struct {
	recordingStreamer
	clearMarkers int
}

func (s *cleanableRecordingStreamer) ClearFinalizedStreamMarker() {
	s.clearMarkers++
}

type failingFinalizeStreamer struct {
	recordingStreamer
	err error
}

func (s *failingFinalizeStreamer) Finalize(ctx context.Context, content string) error {
	s.finalized = append(s.finalized, content)
	return s.err
}

type failingUpdateStreamer struct {
	recordingStreamer
	err error
}

func (s *failingUpdateStreamer) Update(ctx context.Context, content string) error {
	s.updates = append(s.updates, content)
	return s.err
}

type failNthUpdateStreamer struct {
	recordingStreamer
	failOn int
	err    error
}

func (s *failNthUpdateStreamer) Update(ctx context.Context, content string) error {
	s.updates = append(s.updates, content)
	if len(s.updates) == s.failOn {
		return s.err
	}
	return nil
}

type configuredStreamingAfterHook struct {
	content string
	action  HookAction
}

func (h configuredStreamingAfterHook) BeforeLLM(
	ctx context.Context,
	req *LLMHookRequest,
) (*LLMHookRequest, HookDecision, error) {
	return req, HookDecision{Action: HookActionContinue}, nil
}

func (h configuredStreamingAfterHook) AfterLLM(
	ctx context.Context,
	resp *LLMHookResponse,
) (*LLMHookResponse, HookDecision, error) {
	if h.action == HookActionAbortTurn || h.action == HookActionHardAbort {
		return resp, HookDecision{Action: h.action}, nil
	}
	next := resp.Clone()
	next.Response.Content = h.content
	return next, HookDecision{Action: HookActionModify}, nil
}

type configuredStreamingBeforeModelHook struct {
	model string
}

func (h configuredStreamingBeforeModelHook) BeforeLLM(
	ctx context.Context,
	req *LLMHookRequest,
) (*LLMHookRequest, HookDecision, error) {
	next := req.Clone()
	next.Model = h.model
	return next, HookDecision{Action: HookActionModify}, nil
}

func (h configuredStreamingBeforeModelHook) AfterLLM(
	ctx context.Context,
	resp *LLMHookResponse,
) (*LLMHookResponse, HookDecision, error) {
	return resp, HookDecision{Action: HookActionContinue}, nil
}

func configuredStreamingProcessOptions(channel string) processOptions {
	return processOptions{
		SessionKey:              "agent:main:" + channel + ":session-1",
		Channel:                 channel,
		ChatID:                  "session-1",
		UserMessage:             "hello",
		DefaultResponse:         defaultResponse,
		EnableSummary:           false,
		SendResponse:            false,
		AllowInterimPicoPublish: true,
		NoHistory:               true,
	}
}

func runConfiguredStreamingTurn(t *testing.T, al *AgentLoop, channel string) string {
	t.Helper()
	got, err := al.runAgentLoop(
		context.Background(),
		al.GetRegistry().GetDefaultAgent(),
		configuredStreamingProcessOptions(channel),
	)
	if err != nil {
		t.Fatalf("runAgentLoop() error = %v", err)
	}
	return got
}
