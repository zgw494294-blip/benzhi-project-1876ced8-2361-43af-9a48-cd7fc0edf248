package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"rigging-readiness-desk/internal/domain"
	"time"
)

type selfcheckClient struct {
	base    string
	client  *http.Client
	session *domain.RiggingSession
}
type responseEnvelope struct {
	Data  json.RawMessage `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func runSelfcheck(ctx context.Context, base string) error {
	c := &selfcheckClient{base: base, client: &http.Client{Timeout: 3 * time.Second}}
	performance := time.Now().UTC().Add(24 * time.Hour)
	if err := c.create(ctx, map[string]any{"title": "自检演出", "venue": "自检剧场", "performanceAt": performance, "operatorId": "operator-selfcheck", "ruleSetVersion": "RIG-2026.1"}); err != nil {
		return err
	}
	steps := []struct {
		path string
		body func() map[string]any
	}{
		{"/baseline", func() map[string]any { return map[string]any{"baselineRef": "SELF-CHECK-BASELINE"} }},
		{"/lines", func() map[string]any {
			return map[string]any{"code": "SC-LX-01", "ratedLoadGram": 500000, "spanMillimeter": 10000, "maxMomentNewtonMillimeter": 30000000}
		}},
		{"/points", func() map[string]any {
			return map[string]any{"lineId": c.session.Lines[0].ID, "code": "SC-LX-01-P1", "hoistRatedLoadGram": 500000, "positionMillimeter": 5000}
		}},
		{"/loads", func() map[string]any {
			return map[string]any{"lineId": c.session.Lines[0].ID, "pointId": c.session.Points[0].ID, "componentCode": "SC-LOAD-01", "description": "自检景片", "weightGram": 600000, "positionMillimeter": 5000, "quantity": 1, "submittedBy": "operator-selfcheck"}
		}},
		{"/model/finalize", func() map[string]any { return map[string]any{} }},
		{"/calculate", func() map[string]any { return map[string]any{} }},
	}
	for _, step := range steps {
		if err := c.mutate(ctx, step.path, step.body()); err != nil {
			return err
		}
	}
	if len(c.session.Findings) == 0 {
		return fmt.Errorf("自检未生成预期的超载阻断项")
	}
	finding := c.session.Findings[0]
	if err := c.mutate(ctx, "/findings/remediation", map[string]any{"findingId": finding.ID, "assigneeId": "operator-selfcheck", "note": "移除替代配重并重新测量"}); err != nil {
		return err
	}
	if err := c.mutate(ctx, "/loads/revise", map[string]any{"loadId": c.session.Loads[0].ID, "weightGram": 200000, "positionMillimeter": 5000}); err != nil {
		return err
	}
	if err := c.mutate(ctx, "/calculate", map[string]any{}); err != nil {
		return err
	}
	for _, kind := range domain.RequiredChecks {
		if err := c.mutate(ctx, "/checks", map[string]any{"lineId": c.session.Lines[0].ID, "kind": string(kind), "passed": true, "measurement": "基线内", "evidence": "自检现场证据", "inspectorId": "inspector-selfcheck"}); err != nil {
			return err
		}
	}
	if err := c.mutate(ctx, "/inspection/complete", map[string]any{}); err != nil {
		return err
	}
	var confirmation struct {
		ID string `json:"id"`
	}
	if err := c.request(ctx, http.MethodGet, "/api/v1/rigging-sessions/"+c.session.ID+"/review-confirmation", nil, &confirmation); err != nil {
		return err
	}
	for _, step := range []struct {
		path string
		body map[string]any
	}{{"/review", map[string]any{"reviewerId": "reviewer-selfcheck", "decision": "APPROVE", "reason": "独立复核通过", "confirmationId": confirmation.ID}}, {"/freeze", map[string]any{}}, {"/release", map[string]any{}}} {
		if err := c.mutate(ctx, step.path, step.body); err != nil {
			return err
		}
	}
	if c.session.Status != domain.StatusReleased || c.session.Certificate == nil {
		return fmt.Errorf("自检未到达 RELEASED")
	}
	var verified struct {
		Valid bool `json:"valid"`
	}
	if err := c.request(ctx, http.MethodGet, "/api/v1/certificates/"+c.session.Certificate.ID+"/verify", nil, &verified); err != nil {
		return err
	}
	if !verified.Valid {
		return fmt.Errorf("自检凭据校验失败")
	}
	return nil
}
func (c *selfcheckClient) create(ctx context.Context, body map[string]any) error {
	var session domain.RiggingSession
	if err := c.request(ctx, http.MethodPost, "/api/v1/rigging-sessions", body, &session); err != nil {
		return err
	}
	c.session = &session
	return nil
}
func (c *selfcheckClient) mutate(ctx context.Context, path string, body map[string]any) error {
	body["expectedVersion"] = c.session.Version
	var session domain.RiggingSession
	if err := c.request(ctx, http.MethodPost, "/api/v1/rigging-sessions/"+c.session.ID+path, body, &session); err != nil {
		return err
	}
	c.session = &session
	return nil
}
func (c *selfcheckClient) request(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", fmt.Sprintf("selfcheck-%d", time.Now().UnixNano()))
	}
	response, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	var envelope responseEnvelope
	if err = json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if envelope.Error != nil {
			return fmt.Errorf("%s: %s", envelope.Error.Code, envelope.Error.Message)
		}
		return fmt.Errorf("HTTP %d", response.StatusCode)
	}
	return json.Unmarshal(envelope.Data, out)
}
