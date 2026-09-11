package providers

import "strings"

// ModelProviderOption describes a canonical provider entry exposed to the Web UI.
// It also serves as the backend-owned source of truth for shared provider metadata.
type ModelProviderOption struct {
	ID                  string   `json:"id"`
	DisplayName         string   `json:"display_name,omitempty"`
	IconSlug            string   `json:"icon_slug,omitempty"`
	Domain              string   `json:"domain,omitempty"`
	DefaultAPIBase      string   `json:"default_api_base"`
	EmptyAPIKeyAllowed  bool     `json:"empty_api_key_allowed"`
	CreateAllowed       bool     `json:"create_allowed"`
	DefaultModelAllowed bool     `json:"default_model_allowed"`
	SupportsFetch       bool     `json:"supports_fetch,omitempty"`
	DefaultAuthMethod   string   `json:"default_auth_method,omitempty"`
	AuthMethodLocked    bool     `json:"auth_method_locked,omitempty"`
	Local               bool     `json:"local,omitempty"`
	Priority            float64  `json:"priority,omitempty"`
	CommonModels        []string `json:"common_models,omitempty"`
	Aliases             []string `json:"aliases,omitempty"`

	httpAPI bool `json:"-"`
}

var modelProviderOptionsByName = map[string]ModelProviderOption{
	// labs speaks one protocol: an OpenAI-compatible chat completion POST. The
	// default endpoint is the Northlatch gateway; config.json may point elsewhere.
	"openai": {
		ID:                  "openai",
		DisplayName:         "OpenAI-compatible HTTP",
		DefaultAPIBase:      "https://api.weir.social/v1",
		CreateAllowed:       true,
		DefaultModelAllowed: true,
		SupportsFetch:       true,
		Priority:            100,
		Aliases:             []string{"openai-compatible", "gateway"},
		httpAPI:             true,
	},
}

var normalizedModelProviderAliasesByName = buildModelProviderAliasMap()

func buildModelProviderAliasMap() map[string]string {
	totalAliases := 0
	for _, option := range modelProviderOptionsByName {
		totalAliases += len(option.Aliases)
	}

	aliases := make(map[string]string, len(modelProviderOptionsByName)+totalAliases)
	for provider, option := range modelProviderOptionsByName {
		aliases[provider] = provider
		for _, alias := range option.Aliases {
			normalized := strings.ToLower(strings.TrimSpace(alias))
			if normalized == "" {
				continue
			}
			aliases[normalized] = provider
		}
	}
	return aliases
}

func modelProviderOptionForName(provider string) (ModelProviderOption, bool) {
	normalized := NormalizeProvider(provider)
	if normalized == "" {
		return ModelProviderOption{}, false
	}
	option, ok := modelProviderOptionsByName[normalized]
	return option, ok
}
