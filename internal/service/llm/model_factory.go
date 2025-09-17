package llm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"

	// плагины
	"github.com/firebase/genkit/go/plugins/compat_oai"
	"github.com/firebase/genkit/go/plugins/compat_oai/anthropic"
	"github.com/firebase/genkit/go/plugins/compat_oai/openai"

	// опции клиента OpenAI-совместимых API
	"github.com/openai/openai-go/option"
)

type initModel struct {
	provider string
}

type InitModel interface {
	Init(ctx context.Context) (*genkit.Genkit, error)
}

func NewInitModel(_ /*modelName*/ string, provider string) InitModel {
	return &initModel{provider: provider}
}

func (m *initModel) Init(ctx context.Context) (*genkit.Genkit, error) {
	switch strings.ToLower(strings.TrimSpace(m.provider)) {
	case "openai":
		return m.openAi(ctx)
	case "anthropic":
		return m.anthropic(ctx)
	case "deepseek":
		return m.deepseek(ctx)
	default:
		return nil, fmt.Errorf("unsupported provider: %q", m.provider)
	}
}

func (m *initModel) openAi(ctx context.Context) (*genkit.Genkit, error) {
	apiKey, err := requireEnv("OPENAI_API_KEY")
	if err != nil {
		return nil, err
	}

	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}
	if base := strings.TrimSpace(os.Getenv("OPENAI_BASE_URL")); base != "" {
		if _, err := validateBaseURL(base); err != nil {
			return nil, fmt.Errorf("OPENAI_BASE_URL: %w", err)
		}
		opts = append(opts, option.WithBaseURL(base))
	}

	// genkit.Init не возвращает ошибку — валидируем критичное заранее (опционально).
	// Для официального OpenAI preflight обычно не нужен, но оставить можно для единообразия.
	if base := getenvOr("OPENAI_BASE_URL", "https://api.openai.com/v1"); base != "" {
		if err := preflightOpenAICompatible(ctx, base, apiKey); err != nil {
			return nil, fmt.Errorf("openai preflight failed: %w", err)
		}
	}

	g := genkit.Init(ctx, genkit.WithPlugins(&openai.OpenAI{Opts: opts}))
	return g, nil
}

func (m *initModel) anthropic(ctx context.Context) (*genkit.Genkit, error) {
	apiKey, err := requireEnv("ANTHROPIC_API_KEY")
	if err != nil {
		return nil, err
	}
	opts := []option.RequestOption{option.WithAPIKey(apiKey)}

	// Anthropic официально в compat_oai плагине; base URL по умолчанию внутри плагина.
	g := genkit.Init(ctx,
		genkit.WithPlugins(&anthropic.Anthropic{Opts: opts}),
	)
	return g, nil
}

func (m *initModel) deepseek(ctx context.Context) (*genkit.Genkit, error) {
	apiKey, err := requireEnv("DEEPSEEK_API_KEY")
	if err != nil {
		return nil, err
	}

	// DeepSeek: базовый URL + /v1 совместимы с OpenAI; /models существует → удобно для preflight.
	// Источники: оф. дока DeepSeek (base_url и /models).
	base := getenvOr("DEEPSEEK_BASE_URL", "https://api.deepseek.com/v1")
	if _, err := validateBaseURL(base); err != nil {
		return nil, fmt.Errorf("DEEPSEEK_BASE_URL: %w", err)
	}
	if err := preflightOpenAICompatible(ctx, base, apiKey); err != nil {
		return nil, fmt.Errorf("deepseek preflight failed: %w", err)
	}

	ds := &compat_oai.OpenAICompatible{
		Provider: "deepseek", // префикс в имени модели: deepseek/<model>
		Opts: []option.RequestOption{
			option.WithAPIKey(apiKey),
			option.WithBaseURL(base),
			option.WithMaxRetries(2), // разумный дефолт
		},
	}

	g := genkit.Init(ctx, genkit.WithPlugins(ds))

	// Регистрируем модели DeepSeek (описание возможностей — указатель!)
	ds.DefineModel(ds.Provider, "deepseek-chat", ai.ModelOptions{
		Supports: &compat_oai.BasicText,
		Label:    "DeepSeek Chat",
	})
	ds.DefineModel(ds.Provider, "deepseek-reasoner", ai.ModelOptions{
		Supports: &compat_oai.BasicText,
		Label:    "DeepSeek Reasoner",
	})

	// sanity-check: модели действительно видны реестру (из плагина)
	if !ds.IsDefinedModel(g, "deepseek/deepseek-chat") || !ds.IsDefinedModel(g, "deepseek/deepseek-reasoner") {
		return nil, errors.New("deepseek models are not registered in Genkit registry")
	}

	return g, nil
}

/* ------------------------ helpers ------------------------ */

func requireEnv(key string) (string, error) {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return "", fmt.Errorf("missing required env %s", key)
	}
	return val, nil
}

func getenvOr(key, fallback string) string {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback
	}
	return val
}

func validateBaseURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("invalid URL: %q", raw)
	}
	return u, nil
}

// Лёгкая проверка доступности OpenAI-совместимого API: GET <base>/models
func preflightOpenAICompatible(parent context.Context, baseURL, apiKey string) error {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()

	u := strings.TrimRight(baseURL, "/") + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { io.Copy(io.Discard, res.Body); res.Body.Close() }()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d from %s", res.StatusCode, u)
	}
	return nil
}
