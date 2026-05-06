//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/chenhg5/cc-connect/agent/codex"
	"github.com/chenhg5/cc-connect/core"
)

// skipUnlessAgentReady skips the test when the Codex CLI binary is not
// available or the required API credentials are missing.
func skipUnlessAgentReady(t *testing.T, agentType string) {
	t.Helper()
	bin, err := findAgentBin(agentType)
	if err != nil {
		t.Skipf("skip %s: %v", agentType, err)
	}
	if _, err := exec.LookPath(bin); err != nil {
		t.Skipf("skip %s: binary %q not in PATH", agentType, bin)
	}
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Skipf("skip %s: OPENAI_API_KEY not set", agentType)
	}
}

// mockPlatform records all messages sent through it for test verification.
type mockPlatform struct {
	mu       sync.Mutex
	messages []mockMessage
	agent    core.Agent
}

type mockMessage struct {
	Content string
	ReplyCtx any
	Images  []core.ImageAttachment
	Audio   []core.FileAttachment
}

func (m *mockPlatform) Name() string                       { return "mock" }
func (m *mockPlatform) Start(core.MessageHandler) error    { return nil }
func (m *mockPlatform) Stop() error                        { return nil }
func (m *mockPlatform) ClearMessage()                      {}
func (m *mockPlatform) Reply(ctx context.Context, replyCtx any, content string) error {
	return m.Send(ctx, replyCtx, content)
}
func (m *mockPlatform) Send(ctx context.Context, replyCtx any, content string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, mockMessage{Content: content, ReplyCtx: replyCtx})
	return nil
}
func (m *mockPlatform) ReplyCard(ctx context.Context, replyCtx any, card *core.Card) error {
	return m.SendCard(ctx, replyCtx, card)
}
func (m *mockPlatform) SendCard(ctx context.Context, replyCtx any, card *core.Card) error {
	return m.Send(ctx, replyCtx, card.RenderText())
}
func (m *mockPlatform) SendWithButtons(ctx context.Context, replyCtx any, content string, buttons [][]core.ButtonOption) error {
	return m.Send(ctx, replyCtx, content)
}
func (m *mockPlatform) SendImage(ctx context.Context, replyCtx any, img core.ImageAttachment) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, mockMessage{Images: []core.ImageAttachment{img}, ReplyCtx: replyCtx})
	return nil
}
func (m *mockPlatform) SendAudio(ctx context.Context, replyCtx any, audio core.FileAttachment) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, mockMessage{Audio: []core.FileAttachment{audio}, ReplyCtx: replyCtx})
	return nil
}
func (m *mockPlatform) getSent() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.messages))
	for i, msg := range m.messages {
		out[i] = msg.Content
	}
	return out
}
func (m *mockPlatform) getMessages() []mockMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]mockMessage, len(m.messages))
	copy(out, m.messages)
	return out
}
func (m *mockPlatform) clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = nil
}

func findAgentBin(agentType string) (string, error) {
	if agentType == "codex" {
		return "codex", nil
	}
	return "", fmt.Errorf("unsupported agent type: %s", agentType)
}

func setupIntegrationEngine(t *testing.T, agentType string) (*core.Engine, *mockPlatform, string, func()) {
	t.Helper()
	skipUnlessAgentReady(t, agentType)

	workDir := t.TempDir()
	binPath, err := findAgentBin(agentType)
	if err != nil {
		t.Skipf("agent %s not available: %v", agentType, err)
	}
	agent, err := core.CreateAgent(agentType, map[string]any{
		"command":  binPath,
		"work_dir": workDir,
	})
	if err != nil {
		t.Skipf("agent %s not available: %v", agentType, err)
	}

	mp := &mockPlatform{agent: agent}
	e := core.NewEngine("test", agent, []core.Platform{mp}, filepath.Join(workDir, "sessions.json"), core.LangEnglish)

	cleanup := func() {
		agent.Stop()
		e.Stop()
	}
	return e, mp, workDir, cleanup
}

func sessionKey(userID string) string {
	return fmt.Sprintf("mock:channel-1:%s", userID)
}

func waitForMessages(mp *mockPlatform, n int, timeout time.Duration) ([]mockMessage, bool) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
		msgs := mp.getMessages()
		if len(msgs) >= n {
			return msgs, true
		}
	}
	return mp.getMessages(), false
}

func waitForMessageContaining(mp *mockPlatform, substr string, timeout time.Duration) (string, bool) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
		for _, msg := range mp.getMessages() {
			if strings.Contains(strings.ToLower(msg.Content), strings.ToLower(substr)) {
				return msg.Content, true
			}
		}
	}
	return "", false
}

func TestNewSession_Codex(t *testing.T) {
	t.Parallel()
	e, mp, _, cleanup := setupIntegrationEngine(t, "codex")
	defer cleanup()

	e.ReceiveMessage(mp, &core.Message{
		SessionKey: sessionKey("user1"),
		Platform:   "mock",
		UserID:     "user1",
		UserName:   "testuser",
		Content:    "hello, just say hi briefly",
		ReplyCtx:   "ctx1",
	})

	if _, ok := waitForMessageContaining(mp, "hi", 30*time.Second); !ok {
		t.Fatalf("timeout waiting for response; got messages: %v", mp.getSent())
	}
}

func TestListSessions_ShowsActiveSessions(t *testing.T) {
	t.Parallel()
	e, mp, _, cleanup := setupIntegrationEngine(t, "codex")
	defer cleanup()

	e.ReceiveMessage(mp, &core.Message{
		SessionKey: sessionKey("user1"),
		Platform:   "mock",
		UserID:     "user1",
		UserName:   "testuser",
		Content:    "say hi",
		ReplyCtx:   "ctx1",
	})
	waitForMessageContaining(mp, "hi", 30*time.Second)
	mp.clear()

	e.ReceiveMessage(mp, &core.Message{
		SessionKey: sessionKey("user1"),
		Platform:   "mock",
		UserID:     "user1",
		UserName:   "testuser",
		Content:    "/list",
		ReplyCtx:   "ctx1",
	})

	msgs := mp.getSent()
	if len(msgs) == 0 {
		t.Fatal("no messages received for /list")
	}
	listContent := strings.Join(msgs, " ")
	if !strings.Contains(strings.ToLower(listContent), "session") {
		t.Logf("list output: %s", listContent)
	}
}

func TestProviderSwitch(t *testing.T) {
	t.Parallel()
	e, mp, _, cleanup := setupIntegrationEngine(t, "codex")
	defer cleanup()

	e.ReceiveMessage(mp, &core.Message{
		SessionKey: sessionKey("user1"),
		Platform:   "mock",
		UserID:     "user1",
		UserName:   "testuser",
		Content:    "hello",
		ReplyCtx:   "ctx1",
	})
	waitForMessageContaining(mp, "hi", 30*time.Second)
	mp.clear()

	e.ReceiveMessage(mp, &core.Message{
		SessionKey: sessionKey("user1"),
		Platform:   "mock",
		UserID:     "user1",
		UserName:   "testuser",
		Content:    "/provider list",
		ReplyCtx:   "ctx1",
	})

	if _, ok := waitForMessages(mp, 1, 15*time.Second); !ok {
		t.Fatalf("no response for /provider list; got: %v", mp.getSent())
	}
}
