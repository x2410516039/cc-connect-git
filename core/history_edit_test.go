package core

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

type stubConversationAgent struct {
	session *stubConversationSession
	starts  []string
}

func (a *stubConversationAgent) Name() string { return "codex" }
func (a *stubConversationAgent) StartSession(_ context.Context, sessionID string) (AgentSession, error) {
	a.starts = append(a.starts, sessionID)
	if a.session.currentID == "" {
		a.session.currentID = sessionID
	}
	return a.session, nil
}
func (a *stubConversationAgent) ListSessions(_ context.Context) ([]AgentSessionInfo, error) {
	return nil, nil
}
func (a *stubConversationAgent) Stop() error { return nil }

type stubConversationSession struct {
	mu        sync.Mutex
	currentID string
	turns     []ConversationTurn
	drops     []int
	sendCh    chan string
	events    chan Event
}

func newStubConversationSession(threadID string, turns []ConversationTurn) *stubConversationSession {
	for i := range turns {
		turns[i].IndexFromNewest = i
	}
	return &stubConversationSession{
		currentID: threadID,
		turns:     turns,
		sendCh:    make(chan string, 4),
		events:    make(chan Event, 4),
	}
}

func (s *stubConversationSession) Send(prompt string, _ []ImageAttachment, _ []FileAttachment) error {
	s.mu.Lock()
	currentID := s.currentID
	s.mu.Unlock()
	s.sendCh <- prompt
	s.events <- Event{Type: EventResult, Content: "ok", SessionID: currentID, Done: true}
	return nil
}
func (s *stubConversationSession) RespondPermission(_ string, _ PermissionResult) error { return nil }
func (s *stubConversationSession) Events() <-chan Event                                 { return s.events }
func (s *stubConversationSession) CurrentSessionID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.currentID
}
func (s *stubConversationSession) Alive() bool  { return true }
func (s *stubConversationSession) Close() error { return nil }

func (s *stubConversationSession) ListConversationTurns(_ context.Context, _ string, limit int) ([]ConversationTurn, error) {
	if limit > 0 && limit < len(s.turns) {
		return append([]ConversationTurn(nil), s.turns[:limit]...), nil
	}
	return append([]ConversationTurn(nil), s.turns...), nil
}

func (s *stubConversationSession) RollbackConversation(_ context.Context, _ string, dropLastTurns int) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.drops = append(s.drops, dropLastTurns)
	return s.currentID, nil
}

func setupHistoryEditEngine(t *testing.T, turns []ConversationTurn) (*Engine, *stubCardPlatform, *stubConversationSession, string) {
	t.Helper()
	sessionKey := "feishu:user1"
	agentSession := newStubConversationSession("thread-1", turns)
	agent := &stubConversationAgent{session: agentSession}
	p := &stubCardPlatform{stubPlatformEngine: stubPlatformEngine{n: "feishu"}}
	e := NewEngine("test", agent, []Platform{p}, "", LangEnglish)
	e.sessions.GetOrCreateActive(sessionKey).SetAgentSessionID("thread-1", agent.Name())
	return e, p, agentSession, sessionKey
}

func completedTurn(user, assistant string) ConversationTurn {
	return ConversationTurn{ID: "turn-" + user, UserText: user, AssistantText: assistant, Completed: true}
}

func TestEditSelectsPreviousOrdinaryUserMessageAndRunsReplacement(t *testing.T) {
	e, p, session, sessionKey := setupHistoryEditEngine(t, []ConversationTurn{
		completedTurn("/slash should be skipped", "assistant"),
		completedTurn("older no assistant", ""),
		completedTurn("previous ordinary message", "assistant"),
		completedTurn("older ordinary message", "assistant"),
	})
	msg := &Message{SessionKey: sessionKey, ReplyCtx: "ctx"}

	e.cmdEdit(p, msg)
	if len(p.repliedCards) != 0 {
		t.Fatalf("replied cards = %d, want no prompt for direct /edit", len(p.repliedCards))
	}
	card := e.renderEditCard(sessionKey)
	if !strings.Contains(card.RenderText(), "previous o") {
		t.Fatalf("edit card = %q, want previous ordinary message", card.RenderText())
	}

	original := e.sessions.GetOrCreateActive(sessionKey)
	originalID := original.ID
	p.cardErr = fmt.Errorf("no tracked card")
	e.handleMessage(p, &Message{SessionKey: sessionKey, Platform: "feishu", ReplyCtx: "ctx", Content: "replacement"})
	select {
	case prompt := <-session.sendCh:
		if prompt != "replacement" {
			t.Fatalf("sent prompt = %q, want replacement", prompt)
		}
	case <-timeAfterTest():
		t.Fatal("timed out waiting for edit prompt")
	}
	if got := strings.Trim(fmt.Sprint(session.drops), "[]"); got != "3" {
		t.Fatalf("drop counts = %v, want [3]", session.drops)
	}
	active := e.sessions.GetOrCreateActive(sessionKey)
	if active.ID != originalID {
		t.Fatalf("active session id = %q, want original %q", active.ID, originalID)
	}
	if got := active.GetAgentSessionID(); got != "thread-1" {
		t.Fatalf("active agent session id = %q, want same thread", got)
	}
	refreshed := p.getRefreshedCards()
	if len(refreshed) != 0 {
		t.Fatalf("direct /edit should not push help card without tracked card, got %#v", refreshed)
	}
}

func TestEditSelectsLatestInterruptedOrEmptyResponse(t *testing.T) {
	tests := []struct {
		name string
		turn ConversationTurn
	}{
		{
			name: "interrupted",
			turn: ConversationTurn{ID: "turn-third", UserText: "third interrupted", Completed: false},
		},
		{
			name: "empty assistant",
			turn: ConversationTurn{ID: "turn-third", UserText: "third empty response", Completed: true},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e, p, session, sessionKey := setupHistoryEditEngine(t, []ConversationTurn{
				tc.turn,
				completedTurn("second ordinary message", "assistant"),
			})

			e.cmdEdit(p, &Message{SessionKey: sessionKey, ReplyCtx: "ctx"})
			card := e.renderEditCard(sessionKey)
			if !strings.Contains(card.RenderText(), firstRunes(tc.turn.UserText, 10)) {
				t.Fatalf("edit card = %q, want latest turn %q", card.RenderText(), tc.turn.UserText)
			}

			e.handleMessage(p, &Message{SessionKey: sessionKey, Platform: "feishu", ReplyCtx: "ctx", Content: "replacement"})
			select {
			case prompt := <-session.sendCh:
				if prompt != "replacement" {
					t.Fatalf("sent prompt = %q, want replacement", prompt)
				}
			case <-timeAfterTest():
				t.Fatal("timed out waiting for edit prompt")
			}
			if got := strings.Trim(fmt.Sprint(session.drops), "[]"); got != "1" {
				t.Fatalf("drop counts = %v, want [1]", session.drops)
			}
		})
	}
}

func TestEditCardActionShowsWaitingCardAndCompletionRefreshesHelp(t *testing.T) {
	e, p, session, sessionKey := setupHistoryEditEngine(t, []ConversationTurn{
		completedTurn("previous ordinary message", "assistant"),
	})
	card := e.handleCardNav("act:/edit", sessionKey)
	if card == nil || !strings.Contains(card.RenderText(), "previous o") {
		t.Fatalf("edit action card = %q, want waiting edit card", card.RenderText())
	}

	e.handleMessage(p, &Message{SessionKey: sessionKey, Platform: "feishu", ReplyCtx: "ctx", Content: "replacement"})
	select {
	case prompt := <-session.sendCh:
		if prompt != "replacement" {
			t.Fatalf("sent prompt = %q, want replacement", prompt)
		}
	case <-timeAfterTest():
		t.Fatal("timed out waiting for edit prompt")
	}
	refreshed := p.getRefreshedCards()
	if len(refreshed) == 0 {
		t.Fatal("expected help card refresh after edit completion")
	}
	if text := refreshed[len(refreshed)-1].RenderText(); !strings.Contains(text, "/edit") || strings.Contains(text, "/fork") {
		t.Fatalf("refreshed card = %q, want help card", text)
	}
}

func TestEditCancelAndManualCancel(t *testing.T) {
	e, p, _, sessionKey := setupHistoryEditEngine(t, []ConversationTurn{
		completedTurn("previous ordinary", "assistant"),
	})
	msg := &Message{SessionKey: sessionKey, ReplyCtx: "ctx"}
	e.cmdEdit(p, msg)

	cancelCard := e.handleCardNav("act:/edit-mode cancel", sessionKey)
	if cancelCard == nil || !strings.Contains(cancelCard.RenderText(), "/edit") || strings.Contains(cancelCard.RenderText(), "/fork") {
		t.Fatalf("cancel card = %q, want help", cancelCard.RenderText())
	}

	e.cmdEdit(p, msg)
	e.handleMessage(p, &Message{SessionKey: sessionKey, Platform: "feishu", ReplyCtx: "ctx", Content: "/cancel"})
	if got := len(p.sent); got != 0 {
		t.Fatalf("manual /cancel sent %d messages, want none: %#v", got, p.sent)
	}
	if he := e.getHistoryEditState(sessionKey); he != nil {
		t.Fatalf("manual /cancel should clear edit state, got %#v", he)
	}
}

func TestEditNonAppServerBackendUnsupported(t *testing.T) {
	p := &stubCardPlatform{stubPlatformEngine: stubPlatformEngine{n: "feishu"}}
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
	sessionKey := "feishu:user1"
	e.sessions.GetOrCreateActive(sessionKey).SetAgentSessionID("thread-1", "stub")

	e.cmdEdit(p, &Message{SessionKey: sessionKey, ReplyCtx: "ctx"})

	if len(p.sent) == 0 || !strings.Contains(p.sent[0], "app_server") {
		t.Fatalf("reply = %#v, want app_server unsupported message", p.sent)
	}
}

func TestHelpCardIncludesHistoryEditCommands(t *testing.T) {
	e := NewEngine("test", &stubAgent{}, []Platform{&stubPlatformEngine{n: "test"}}, "", LangEnglish)

	card := e.renderHelpGroupCard("session")

	if id := matchPrefix("fork", builtinCommands); id != "" {
		t.Fatalf("/fork command id = %q, want unregistered", id)
	}
	if _, ok := findCardAction(card, "nav:/fork"); ok {
		t.Fatal("did not expect /fork help action")
	}
	if _, ok := findCardAction(card, "act:/edit"); !ok {
		t.Fatal("expected /edit help action")
	}
}

func timeAfterTest() <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		<-time.After(2 * time.Second)
		close(ch)
	}()
	return ch
}

func waitForSessionHistoryLen(t *testing.T, session *Session, want int) []HistoryEntry {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		history := session.GetHistory(0)
		if len(history) >= want {
			return history
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for session history len >= %d, got %d", want, len(history))
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func historyEntriesText(entries []HistoryEntry) string {
	var sb strings.Builder
	for _, entry := range entries {
		sb.WriteString(entry.Content)
		sb.WriteByte('\n')
	}
	return sb.String()
}
