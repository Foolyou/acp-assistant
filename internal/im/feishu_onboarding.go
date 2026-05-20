package im

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const feishuRegistrationPath = "/oauth/v1/app/registration"

type FeishuRegistrationOptions struct {
	Domain         string
	TimeoutSeconds int
}

type FeishuRegistrationClient struct {
	AccountsBaseURL string
	OpenBaseURL     string
	HTTPClient      *http.Client
}

type FeishuRegistrationResult struct {
	AppID     string
	AppSecret string
	Domain    string
	OpenID    string

	BotName   string
	BotOpenID string

	DeviceCode string
	UserCode   string
	QRURL      string
	Interval   int
	ExpireIn   int
}

func (c FeishuRegistrationClient) Register(ctx context.Context, opts FeishuRegistrationOptions) (FeishuRegistrationResult, error) {
	begin, err := c.Begin(ctx, opts)
	if err != nil {
		return FeishuRegistrationResult{}, err
	}
	return c.Poll(ctx, begin)
}

func (c FeishuRegistrationClient) Begin(ctx context.Context, opts FeishuRegistrationOptions) (FeishuRegistrationResult, error) {
	if opts.Domain == "" {
		opts.Domain = "feishu"
	}
	if opts.TimeoutSeconds <= 0 {
		opts.TimeoutSeconds = 600
	}
	if err := c.init(ctx, opts.Domain); err != nil {
		return FeishuRegistrationResult{}, err
	}
	begin, err := c.begin(ctx, opts.Domain)
	if err != nil {
		return FeishuRegistrationResult{}, err
	}
	begin.ExpireIn = minInt(begin.ExpireIn, opts.TimeoutSeconds)
	return begin, nil
}

func (c FeishuRegistrationClient) Poll(ctx context.Context, begin FeishuRegistrationResult) (FeishuRegistrationResult, error) {
	if begin.Domain == "" {
		begin.Domain = "feishu"
	}
	if begin.ExpireIn <= 0 {
		begin.ExpireIn = 600
	}
	result, err := c.poll(ctx, begin.Domain, begin.DeviceCode, begin.Interval, begin.ExpireIn)
	if err != nil {
		return FeishuRegistrationResult{}, err
	}
	result.DeviceCode = begin.DeviceCode
	result.UserCode = begin.UserCode
	result.QRURL = begin.QRURL
	result.Interval = begin.Interval
	result.ExpireIn = begin.ExpireIn
	if bot, err := c.ProbeBot(ctx, result.AppID, result.AppSecret, result.Domain); err == nil {
		result.BotName = bot.BotName
		result.BotOpenID = bot.BotOpenID
	}
	return result, nil
}

func (c FeishuRegistrationClient) ProbeBot(ctx context.Context, appID, appSecret, domain string) (FeishuRegistrationResult, error) {
	base := c.openBaseURL(domain)
	tokenBody, _ := json.Marshal(map[string]string{"app_id": appID, "app_secret": appSecret})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/open-apis/auth/v3/tenant_access_token/internal", bytes.NewReader(tokenBody))
	if err != nil {
		return FeishuRegistrationResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient(c.HTTPClient).Do(req)
	if err != nil {
		return FeishuRegistrationResult{}, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if resp.StatusCode >= 300 {
		return FeishuRegistrationResult{}, fmt.Errorf("tenant token request failed: %s", strings.TrimSpace(string(data)))
	}
	var tokenRes struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
	}
	if err := json.Unmarshal(data, &tokenRes); err != nil {
		return FeishuRegistrationResult{}, err
	}
	if tokenRes.Code != 0 || tokenRes.TenantAccessToken == "" {
		return FeishuRegistrationResult{}, fmt.Errorf("tenant token request failed: %s", tokenRes.Msg)
	}
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, base+"/open-apis/bot/v3/info", nil)
	if err != nil {
		return FeishuRegistrationResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+tokenRes.TenantAccessToken)
	resp, err = httpClient(c.HTTPClient).Do(req)
	if err != nil {
		return FeishuRegistrationResult{}, err
	}
	defer resp.Body.Close()
	data, _ = io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if resp.StatusCode >= 300 {
		return FeishuRegistrationResult{}, fmt.Errorf("bot info request failed: %s", strings.TrimSpace(string(data)))
	}
	var botRes struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Bot  struct {
			AppName string `json:"app_name"`
			BotName string `json:"bot_name"`
			OpenID  string `json:"open_id"`
		} `json:"bot"`
		Data struct {
			Bot struct {
				AppName string `json:"app_name"`
				BotName string `json:"bot_name"`
				OpenID  string `json:"open_id"`
			} `json:"bot"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &botRes); err != nil {
		return FeishuRegistrationResult{}, err
	}
	if botRes.Code != 0 {
		return FeishuRegistrationResult{}, fmt.Errorf("bot info request failed: %s", botRes.Msg)
	}
	name := botRes.Bot.AppName
	if name == "" {
		name = botRes.Bot.BotName
	}
	openID := botRes.Bot.OpenID
	if name == "" {
		name = botRes.Data.Bot.AppName
	}
	if name == "" {
		name = botRes.Data.Bot.BotName
	}
	if openID == "" {
		openID = botRes.Data.Bot.OpenID
	}
	return FeishuRegistrationResult{BotName: name, BotOpenID: openID}, nil
}

func (c FeishuRegistrationClient) init(ctx context.Context, domain string) error {
	res, err := c.postRegistration(ctx, domain, map[string]string{"action": "init"})
	if err != nil {
		return err
	}
	methods, _ := res["supported_auth_methods"].([]any)
	for _, method := range methods {
		if method == "client_secret" {
			return nil
		}
	}
	return fmt.Errorf("Feishu registration does not support client_secret auth")
}

func (c FeishuRegistrationClient) begin(ctx context.Context, domain string) (FeishuRegistrationResult, error) {
	res, err := c.postRegistration(ctx, domain, map[string]string{
		"action":            "begin",
		"archetype":         "PersonalAgent",
		"auth_method":       "client_secret",
		"request_user_info": "open_id",
	})
	if err != nil {
		return FeishuRegistrationResult{}, err
	}
	deviceCode, _ := res["device_code"].(string)
	if deviceCode == "" {
		return FeishuRegistrationResult{}, fmt.Errorf("Feishu registration did not return device_code")
	}
	qrURL, _ := res["verification_uri_complete"].(string)
	if qrURL != "" {
		separator := "?"
		if strings.Contains(qrURL, "?") {
			separator = "&"
		}
		qrURL += separator + "from=acpa&tp=acpa"
	}
	return FeishuRegistrationResult{
		DeviceCode: deviceCode,
		UserCode:   stringValue(res["user_code"]),
		QRURL:      qrURL,
		Interval:   intValue(res["interval"], 5),
		ExpireIn:   intValue(res["expire_in"], 600),
		Domain:     domain,
	}, nil
}

func (c FeishuRegistrationClient) poll(ctx context.Context, domain, deviceCode string, interval, expireIn int) (FeishuRegistrationResult, error) {
	if interval <= 0 {
		interval = 5
	}
	if expireIn <= 0 {
		expireIn = 600
	}
	pollCtx, cancel := context.WithTimeout(ctx, time.Duration(expireIn)*time.Second)
	defer cancel()
	currentDomain := domain
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()
	for {
		res, err := c.postRegistration(pollCtx, currentDomain, map[string]string{
			"action":      "poll",
			"device_code": deviceCode,
			"tp":          "ob_app",
		})
		if err == nil {
			userInfo, _ := res["user_info"].(map[string]any)
			if tenantBrand, _ := userInfo["tenant_brand"].(string); tenantBrand == "lark" {
				currentDomain = "lark"
			}
			appID, _ := res["client_id"].(string)
			appSecret, _ := res["client_secret"].(string)
			if appID != "" && appSecret != "" {
				openID, _ := userInfo["open_id"].(string)
				return FeishuRegistrationResult{AppID: appID, AppSecret: appSecret, Domain: currentDomain, OpenID: openID}, nil
			}
			if errorCode, _ := res["error"].(string); errorCode == "access_denied" || errorCode == "expired_token" {
				return FeishuRegistrationResult{}, fmt.Errorf("Feishu registration %s", errorCode)
			}
		}
		select {
		case <-pollCtx.Done():
			return FeishuRegistrationResult{}, fmt.Errorf("Feishu registration timed out")
		case <-ticker.C:
		}
	}
}

func (c FeishuRegistrationClient) postRegistration(ctx context.Context, domain string, body map[string]string) (map[string]any, error) {
	form := url.Values{}
	for key, value := range body {
		form.Set(key, value)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.accountsBaseURL(domain)+feishuRegistrationPath, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := httpClient(c.HTTPClient).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		if errText, _ := parsed["error"].(string); errText != "" {
			return parsed, nil
		}
		return parsed, fmt.Errorf("Feishu registration request failed: %s", strings.TrimSpace(string(data)))
	}
	return parsed, nil
}

func (c FeishuRegistrationClient) accountsBaseURL(domain string) string {
	if c.AccountsBaseURL != "" {
		return strings.TrimRight(c.AccountsBaseURL, "/")
	}
	if domain == "lark" {
		return "https://accounts.larksuite.com"
	}
	return "https://accounts.feishu.cn"
}

func (c FeishuRegistrationClient) openBaseURL(domain string) string {
	if c.OpenBaseURL != "" {
		return strings.TrimRight(c.OpenBaseURL, "/")
	}
	if domain == "lark" {
		return "https://open.larksuite.com"
	}
	return "https://open.feishu.cn"
}

func intValue(value any, fallback int) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	default:
		return fallback
	}
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
