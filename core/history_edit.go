package core

import (
	"fmt"
	"strings"
)

const (
	historyEditScanLimit = 200
)

type historyEditState struct {
	phase     string
	selected  historyEditTarget
	errorText string
}

type historyEditTarget struct {
	UserText    string
	DropThrough int
	Before      []HistoryEntry
}

func (e *Engine) cmdEdit(p Platform, msg *Message) {
	if err := e.prepareEditWaitForSession(msg.SessionKey, p, msg.ReplyCtx); err != nil {
		e.reply(p, msg.ReplyCtx, err.Error())
		return
	}
}

func (e *Engine) prepareEditWaitForSession(sessionKey string, p Platform, replyCtx any) error {
	_, _, _, state, editor, sessionID, unlock, err := e.lockedHistoryEditor(sessionKey, p, replyCtx)
	if err != nil {
		e.setHistoryEditError(sessionKey, err.Error())
		return err
	}
	defer unlock()

	turns, err := editor.ListConversationTurns(e.ctx, sessionID, historyEditScanLimit)
	if err != nil {
		e.setHistoryEditError(sessionKey, err.Error())
		return err
	}
	targets := buildHistoryEditTargets(turns, 1)
	if len(targets) == 0 {
		err := fmt.Errorf("%s", e.i18n.T(MsgEditNoMessage))
		e.setHistoryEditError(sessionKey, err.Error())
		return err
	}

	state.mu.Lock()
	state.historyEdit = &historyEditState{
		phase:    "wait",
		selected: targets[0],
	}
	state.mu.Unlock()
	return nil
}

func (e *Engine) lockedHistoryEditor(sessionKey string, p Platform, replyCtx any) (Agent, *SessionManager, *Session, *interactiveState, ConversationEditor, string, func(), error) {
	agent, sessions := e.sessionContextForKey(sessionKey)
	session := sessions.GetOrCreateActive(sessionKey)
	if !session.TryLock() {
		return nil, nil, nil, nil, nil, "", func() {}, fmt.Errorf("%s", e.i18n.T(MsgPreviousProcessing))
	}
	unlock := func() { session.UnlockWithoutUpdate() }

	interactiveKey := e.interactiveKeyForSessionKey(sessionKey)
	sessionID := session.GetAgentSessionID()
	if state := e.peekInteractiveState(interactiveKey); state != nil {
		state.mu.Lock()
		if state.agentSession != nil && state.agentSession.Alive() && state.agentSession.CurrentSessionID() != "" {
			sessionID = state.agentSession.CurrentSessionID()
		}
		state.mu.Unlock()
	}
	if sessionID == "" {
		unlock()
		return nil, nil, nil, nil, nil, "", func() {}, fmt.Errorf("%s", e.i18n.T(MsgHistoryEditNoSession))
	}

	if p == nil {
		p = e.platformForSessionKey(sessionKey)
	}
	if p == nil {
		unlock()
		return nil, nil, nil, nil, nil, "", func() {}, fmt.Errorf("%s", e.i18n.T(MsgHistoryEditNoPlatform))
	}

	var agentOverride Agent
	if agent != e.agent {
		agentOverride = agent
	}
	state := e.getOrCreateInteractiveStateWith(interactiveKey, p, replyCtx, session, sessions, agentOverride, "")
	if state.agentSession == nil {
		unlock()
		return nil, nil, nil, nil, nil, "", func() {}, fmt.Errorf("%s", e.i18n.T(MsgFailedToStartAgentSession))
	}
	editor, ok := state.agentSession.(ConversationEditor)
	if !ok {
		unlock()
		return nil, nil, nil, nil, nil, "", func() {}, fmt.Errorf("%s", e.i18n.T(MsgHistoryEditNotSupported))
	}
	if currentID := state.agentSession.CurrentSessionID(); currentID != "" {
		sessionID = currentID
	}
	return agent, sessions, session, state, editor, sessionID, unlock, nil
}

func (e *Engine) peekInteractiveState(interactiveKey string) *interactiveState {
	e.interactiveMu.Lock()
	defer e.interactiveMu.Unlock()
	return e.interactiveStates[interactiveKey]
}

func (e *Engine) platformForSessionKey(sessionKey string) Platform {
	platformName := extractPlatformName(sessionKey)
	for _, p := range e.platforms {
		if p.Name() == platformName {
			return p
		}
	}
	return nil
}

func buildHistoryEditTargets(turns []ConversationTurn, max int) []historyEditTarget {
	targets := make([]historyEditTarget, 0, max)
	for i, turn := range turns {
		if len(targets) >= max {
			break
		}
		userText := strings.TrimSpace(turn.UserText)
		if userText == "" || !isOrdinaryHistoryUserText(userText) {
			continue
		}
		hasAssistantReply := strings.TrimSpace(turn.AssistantText) != ""
		isInterruptedLatest := i == 0 && (!turn.Completed || !hasAssistantReply)
		if !isInterruptedLatest && (!turn.Completed || !hasAssistantReply) {
			continue
		}
		indexFromNewest := turn.IndexFromNewest
		if i > 0 && indexFromNewest == 0 {
			indexFromNewest = i
		}
		targets = append(targets, historyEditTarget{
			UserText:    userText,
			DropThrough: indexFromNewest + 1,
			Before:      historyEntriesFromTurnsDesc(turns[i+1:]),
		})
	}
	return targets
}

func isOrdinaryHistoryUserText(text string) bool {
	return !strings.HasPrefix(strings.TrimSpace(text), "/")
}

func historyEntriesFromTurnsDesc(turns []ConversationTurn) []HistoryEntry {
	entries := make([]HistoryEntry, 0, len(turns)*2)
	for i := len(turns) - 1; i >= 0; i-- {
		turn := turns[i]
		if strings.TrimSpace(turn.UserText) != "" {
			entries = append(entries, HistoryEntry{Role: "user", Content: turn.UserText, Timestamp: turn.StartedAt})
		}
		if strings.TrimSpace(turn.AssistantText) != "" {
			entries = append(entries, HistoryEntry{Role: "assistant", Content: turn.AssistantText, Timestamp: turn.CompletedAt})
		}
	}
	return entries
}

func firstRunes(text string, n int) string {
	rs := []rune(strings.TrimSpace(text))
	if len(rs) <= n {
		return string(rs)
	}
	return string(rs[:n])
}

func (e *Engine) getHistoryEditState(sessionKey string) *historyEditState {
	state := e.peekInteractiveState(e.interactiveKeyForSessionKey(sessionKey))
	if state == nil {
		return nil
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.historyEdit == nil {
		return nil
	}
	cp := *state.historyEdit
	return &cp
}

func (e *Engine) setHistoryEditError(sessionKey, text string) {
	interactiveKey := e.interactiveKeyForSessionKey(sessionKey)
	e.interactiveMu.Lock()
	state := e.interactiveStates[interactiveKey]
	if state == nil {
		state = &interactiveState{}
		e.interactiveStates[interactiveKey] = state
	}
	e.interactiveMu.Unlock()
	state.mu.Lock()
	state.historyEdit = &historyEditState{phase: "error", errorText: text}
	state.mu.Unlock()
}

func (e *Engine) clearHistoryEditState(sessionKey string) {
	state := e.peekInteractiveState(e.interactiveKeyForSessionKey(sessionKey))
	if state == nil {
		return
	}
	state.mu.Lock()
	state.historyEdit = nil
	state.mu.Unlock()
}

func (e *Engine) renderEditCard(sessionKey string) *Card {
	he := e.getHistoryEditState(sessionKey)
	if he == nil {
		if err := e.prepareEditWaitForSession(sessionKey, nil, nil); err != nil {
			return NewCard().Title(e.i18n.T(MsgEditWaitingTitle), "red").Markdown(err.Error()).Buttons(e.cardBackButton()).Build()
		}
		he = e.getHistoryEditState(sessionKey)
	}
	if he == nil {
		return e.simpleCard(e.i18n.T(MsgEditWaitingTitle), "red", e.i18n.T(MsgEditNoMessage))
	}
	if he.phase == "error" {
		return NewCard().Title(e.i18n.T(MsgEditWaitingTitle), "red").Markdown(he.errorText).Buttons(e.cardBackButton()).Build()
	}
	return NewCard().
		Title(e.i18n.T(MsgEditWaitingTitle), "yellow").
		Markdown(e.i18n.Tf(MsgEditWaitingBody, firstRunes(he.selected.UserText, 10))).
		Buttons(DefaultBtn(e.i18n.T(MsgEditCancelButton), "act:/edit-mode cancel")).
		Build()
}

func (e *Engine) executeEditModeAction(sessionKey, args string) {
	if strings.HasPrefix(strings.TrimSpace(args), "cancel") {
		e.clearHistoryEditState(sessionKey)
	}
}

func (e *Engine) handlePendingHistoryEdit(p Platform, msg *Message, content string) bool {
	trimmed := strings.TrimSpace(content)
	he := e.getHistoryEditState(msg.SessionKey)
	if he == nil {
		return false
	}
	if strings.EqualFold(trimmed, "/cancel") {
		e.clearHistoryEditState(msg.SessionKey)
		return true
	}
	if he.phase != "wait" {
		return false
	}
	if strings.HasPrefix(trimmed, "/") {
		return false
	}

	agent, sessions, session, _, editor, sessionID, unlock, err := e.lockedHistoryEditor(msg.SessionKey, p, msg.ReplyCtx)
	if err != nil {
		e.reply(p, msg.ReplyCtx, err.Error())
		return true
	}

	dropTurns := he.selected.DropThrough
	history := he.selected.Before
	newAgentSessionID, editErr := editor.RollbackConversation(e.ctx, sessionID, dropTurns)
	unlock()
	if editErr != nil {
		e.reply(p, msg.ReplyCtx, e.i18n.Tf(MsgHistoryEditFailed, editErr))
		return true
	}

	if newAgentSessionID != "" {
		session.SetAgentSessionID(newAgentSessionID, agent.Name())
	}
	session.SetHistory(history)
	sessions.Save()
	e.clearHistoryEditState(msg.SessionKey)
	e.refreshHelpAfterHistoryEditComplete(p, msg)
	return false
}

func (e *Engine) refreshHelpAfterHistoryEditComplete(p Platform, msg *Message) {
	refresher, ok := p.(CardRefresher)
	if !ok {
		return
	}
	_ = refresher.RefreshCard(e.ctx, msg.SessionKey, e.renderHelpCard())
}

func (e *Engine) refreshHelpAfterHistoryCancel(p Platform, msg *Message) {
	card := e.renderHelpCard()
	if refresher, ok := p.(CardRefresher); ok {
		if err := refresher.RefreshCard(e.ctx, msg.SessionKey, card); err == nil {
			return
		}
	}
	if supportsCards(p) {
		e.replyWithCard(p, msg.ReplyCtx, card)
		return
	}
	e.reply(p, msg.ReplyCtx, e.i18n.T(MsgHelp))
}
