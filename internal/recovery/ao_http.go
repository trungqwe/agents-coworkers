package recovery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const taskReceiptPrefix = "TASK_RECEIPT "

type AOHTTPClient struct {
	BaseURL string
	Client  *http.Client
}

type steerOrSendRequest struct {
	Text            string `json:"text,omitempty"`
	ClientMessageID string `json:"clientMessageId"`
	RecoverOnly     bool   `json:"recoverOnly,omitempty"`
}

type steerOrSendResponse struct {
	Outcome        string `json:"outcome"`
	TurnID         string `json:"turnId"`
	ProviderTurnID string `json:"providerTurnId"`
	State          string `json:"state"`
}

type conversationSnapshot struct {
	Controller     string `json:"controller"`
	OldestSequence int64  `json:"oldestSequence"`
	HasMoreBefore  bool   `json:"hasMoreBefore"`
	Settings       struct {
		Model           string `json:"model"`
		ReasoningEffort string `json:"reasoningEffort"`
	} `json:"settings"`
	Turns []struct {
		ID             string `json:"id"`
		ProviderTurnID string `json:"providerTurnId"`
		State          string `json:"state"`
	} `json:"turns"`
	Messages []struct {
		TurnID string `json:"turnId"`
		Role   string `json:"role"`
		Text   string `json:"text"`
	} `json:"messages"`
}

type sessionView struct {
	ID           string `json:"id"`
	ProjectID    string `json:"projectId"`
	Kind         string `json:"kind"`
	Harness      string `json:"harness"`
	Model        string `json:"model"`
	Branch       string `json:"branch"`
	IsTerminated bool   `json:"isTerminated"`
}

type sessionResponse struct {
	Session sessionView `json:"session"`
}

func NewAOHTTPClient(baseURL string, client *http.Client) (*AOHTTPClient, error) {
	u, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return nil, err
	}
	host := strings.ToLower(u.Hostname())
	if u.Scheme != "http" || host != "127.0.0.1" && host != "localhost" && host != "::1" {
		return nil, errors.New("AO recovery adapter requires an HTTP loopback base URL")
	}
	if client == nil {
		client = http.DefaultClient
	}
	return &AOHTTPClient{BaseURL: strings.TrimRight(baseURL, "/"), Client: client}, nil
}

func (c *AOHTTPClient) Observe(ctx context.Context, sessionID, turnHandle string) (Observation, error) {
	sessionPath := "/api/v1/sessions/" + url.PathEscape(sessionID)
	status, _, err := c.request(ctx, http.MethodGet, sessionPath, nil)
	if err != nil {
		return Observation{}, err
	}
	if status == http.StatusNotFound {
		return Observation{SessionExists: false}, nil
	}
	if status != http.StatusOK {
		return Observation{}, fmt.Errorf("AO session read returned HTTP %d", status)
	}
	if turnHandle == "" {
		return Observation{SessionExists: true, Delivery: DeliveryNotFound}, nil
	}
	providerHandle := strings.TrimPrefix(turnHandle, "provider:")
	var before int64
	for page := 0; page < 1000; page++ {
		path := sessionPath + "/conversation?limit=500"
		if before > 0 {
			path += "&beforeSequence=" + fmt.Sprint(before)
		}
		status, body, err := c.request(ctx, http.MethodGet, path, nil)
		if err != nil {
			return Observation{}, err
		}
		if status != http.StatusOK {
			return Observation{}, fmt.Errorf("AO conversation read returned HTTP %d", status)
		}
		var snapshot conversationSnapshot
		if err := json.Unmarshal(body, &snapshot); err != nil {
			return Observation{}, fmt.Errorf("decode AO conversation snapshot: %w", err)
		}
		for _, turn := range snapshot.Turns {
			if turn.ID != turnHandle && (turn.ProviderTurnID == "" || turn.ProviderTurnID != providerHandle) {
				continue
			}
			obs := Observation{SessionExists: true, Delivery: mapTurnState(turn.State)}
			if obs.Delivery == DeliveryCompleted {
				obs.Receipt = receiptForTurn(snapshot, turn.ID)
			}
			return obs, nil
		}
		if !snapshot.HasMoreBefore {
			return Observation{SessionExists: true, Delivery: DeliveryNotFound}, nil
		}
		if snapshot.OldestSequence <= 0 || (before > 0 && snapshot.OldestSequence >= before) {
			return Observation{}, errors.New("AO conversation pagination did not advance")
		}
		before = snapshot.OldestSequence
	}
	return Observation{}, errors.New("AO conversation pagination limit exceeded")
}

func (c *AOHTTPClient) Preflight(ctx context.Context, expected SessionExpectation) error {
	path := "/api/v1/sessions/" + url.PathEscape(expected.SessionID)
	status, body, err := c.request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("%w: session HTTP %d", ErrSessionOwnerMismatch, status)
	}
	var response sessionResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("decode AO session: %w", err)
	}
	session := response.Session
	if expected.SessionID == "" || expected.ProjectID == "" || expected.Kind == "" || expected.Harness == "" || expected.Model == "" || expected.Effort == "" || expected.Branch == "" || !expected.Exclusive {
		return fmt.Errorf("%w: checkpoint session identity is incomplete", ErrSessionOwnerMismatch)
	}
	if session.ID != expected.SessionID || session.IsTerminated ||
		expected.ProjectID != "" && session.ProjectID != expected.ProjectID ||
		expected.Kind != "" && session.Kind != expected.Kind ||
		expected.Harness != "" && session.Harness != expected.Harness ||
		expected.Model != "" && session.Model != expected.Model ||
		expected.Branch != "" && session.Branch != expected.Branch {
		return ErrSessionOwnerMismatch
	}
	if session.ID == "" || session.ProjectID == "" || session.Kind == "" || session.Harness == "" || session.Model == "" || session.Branch == "" {
		return fmt.Errorf("%w: AO session response is incomplete", ErrSessionOwnerMismatch)
	}
	status, body, err = c.request(ctx, http.MethodGet, path+"/conversation?limit=1", nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("AO conversation preflight HTTP %d", status)
	}
	var snapshot conversationSnapshot
	if err := json.Unmarshal(body, &snapshot); err != nil {
		return err
	}
	if snapshot.Controller != "ready" {
		return fmt.Errorf("%w: controller=%s", ErrSessionNotIdle, snapshot.Controller)
	}
	if snapshot.Settings.Model == "" || snapshot.Settings.ReasoningEffort == "" || snapshot.Settings.Model != expected.Model || snapshot.Settings.ReasoningEffort != expected.Effort {
		return fmt.Errorf("%w: conversation model/effort readback mismatch", ErrSessionOwnerMismatch)
	}
	return nil
}

func (c *AOHTTPClient) Send(ctx context.Context, sessionID, deliveryID, message string) (DeliveryReceipt, error) {
	return c.steerOrSend(ctx, sessionID, steerOrSendRequest{Text: message, ClientMessageID: deliveryID}, false)
}

func (c *AOHTTPClient) RecoverOnly(ctx context.Context, sessionID, deliveryID string) (DeliveryReceipt, error) {
	return c.steerOrSend(ctx, sessionID, steerOrSendRequest{ClientMessageID: deliveryID, RecoverOnly: true}, true)
}

func (c *AOHTTPClient) Stop(ctx context.Context, sessionID, expectedTurnID string) error {
	obs, err := c.Observe(ctx, sessionID, expectedTurnID)
	if err != nil {
		return err
	}
	if !obs.SessionExists || obs.Delivery != DeliveryAccepted {
		return fmt.Errorf("%w: expected turn is not the active delivery", ErrSessionOwnerMismatch)
	}
	live, err := c.liveTurnIDs(ctx, sessionID)
	if err != nil {
		return err
	}
	if len(live) != 1 || live[0] != expectedTurnID {
		return fmt.Errorf("%w: active turn ownership is ambiguous", ErrSessionOwnerMismatch)
	}
	status, _, err := c.request(ctx, http.MethodPost, "/api/v1/sessions/"+url.PathEscape(sessionID)+"/conversation/interrupt", []byte(`{}`))
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("AO interrupt returned HTTP %d", status)
	}
	return nil
}

func (c *AOHTTPClient) liveTurnIDs(ctx context.Context, sessionID string) ([]string, error) {
	base := "/api/v1/sessions/" + url.PathEscape(sessionID) + "/conversation?limit=500"
	var before int64
	var live []string
	for page := 0; page < 1000; page++ {
		path := base
		if before > 0 {
			path += "&beforeSequence=" + fmt.Sprint(before)
		}
		status, body, err := c.request(ctx, http.MethodGet, path, nil)
		if err != nil {
			return nil, err
		}
		if status != http.StatusOK {
			return nil, fmt.Errorf("AO conversation read returned HTTP %d", status)
		}
		var snapshot conversationSnapshot
		if err := json.Unmarshal(body, &snapshot); err != nil {
			return nil, err
		}
		for _, turn := range snapshot.Turns {
			if mapTurnState(turn.State) == DeliveryAccepted {
				live = append(live, turn.ID)
			}
		}
		if !snapshot.HasMoreBefore {
			return live, nil
		}
		if snapshot.OldestSequence <= 0 || (before > 0 && snapshot.OldestSequence >= before) {
			return nil, errors.New("AO conversation pagination did not advance")
		}
		before = snapshot.OldestSequence
	}
	return nil, errors.New("AO conversation pagination limit exceeded")
}

func (c *AOHTTPClient) steerOrSend(ctx context.Context, sessionID string, payload steerOrSendRequest, recoverOnly bool) (DeliveryReceipt, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return DeliveryReceipt{}, fmt.Errorf("%w: encode AO request: %v", ErrRequestNotAccepted, err)
	}
	path := "/api/v1/sessions/" + url.PathEscape(sessionID) + "/conversation/steer-or-send"
	status, body, err := c.request(ctx, http.MethodPost, path, data)
	if err != nil {
		return DeliveryReceipt{}, err // unknown outcome: dispatcher must recover-only
	}
	if status < 200 || status >= 300 {
		if !recoverOnly && (status == http.StatusBadRequest || status == http.StatusNotFound) {
			return DeliveryReceipt{}, fmt.Errorf("%w: AO returned HTTP %d", ErrRequestNotAccepted, status)
		}
		return DeliveryReceipt{}, fmt.Errorf("AO delivery outcome uncertain: HTTP %d", status)
	}
	var response steerOrSendResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return DeliveryReceipt{}, fmt.Errorf("decode AO delivery response: %w", err)
	}
	if response.Outcome == "steered" {
		return DeliveryReceipt{}, ErrSessionContaminated
	}
	if response.Outcome != "sent" {
		return DeliveryReceipt{}, fmt.Errorf("unknown AO delivery outcome %q", response.Outcome)
	}
	turnID := response.TurnID
	if turnID == "" {
		return DeliveryReceipt{}, errors.New("AO accepted delivery without a turn handle")
	}
	state := mapTurnState(response.State)
	if state == DeliveryNotFound || state == DeliveryRejected {
		state = DeliveryAccepted
	}
	return DeliveryReceipt{State: state, TurnID: turnID}, nil
}

func (c *AOHTTPClient) request(ctx context.Context, method, path string, body []byte) (int, []byte, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, reader)
	if err != nil {
		return 0, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, data, err
}

func mapTurnState(state string) DeliveryState {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "queued", "running", "recovered":
		return DeliveryAccepted
	case "completed":
		return DeliveryCompleted
	case "failed":
		return DeliveryRejected
	case "cancelled", "interrupted":
		return DeliveryStopped
	default:
		return DeliveryNotFound
	}
}

func receiptForTurn(snapshot conversationSnapshot, turnID string) *TaskReceipt {
	for i := len(snapshot.Messages) - 1; i >= 0; i-- {
		message := snapshot.Messages[i]
		if message.TurnID != turnID || message.Role != "assistant" {
			continue
		}
		for _, line := range strings.Split(message.Text, "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, taskReceiptPrefix) {
				continue
			}
			var receipt TaskReceipt
			if json.Unmarshal([]byte(strings.TrimPrefix(line, taskReceiptPrefix)), &receipt) == nil {
				return &receipt
			}
		}
	}
	return nil
}
