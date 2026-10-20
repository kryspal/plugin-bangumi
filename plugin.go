package main

import (
	"context"
	_ "embed"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"google.golang.org/protobuf/proto"

	pluginv1 "github.com/kryspal/plugin-sdk/pluginv1"
)

const (
	pluginID = "bangumi"

	keyBaseURL   = "base_url"
	keyTitleLang = "title_language"
	keyMaxCast   = "max_cast"

	defaultBaseURL = "https://api.bgm.tv"
	defaultMaxCast = 25
	maxCastLimit   = 200

	titleAuto    = "auto"
	titleChinese = "chinese"
	titleNative  = "native"

	attributionText = "Metadata from Bangumi (bgm.tv), licensed CC BY-SA. This plugin is unofficial and not endorsed by Bangumi."
	attributionLink = "https://bgm.tv"
)

// versionFile is the single source of the plugin version: the manifest's version.
//
//go:embed VERSION
var versionFile string

func version() string { return strings.TrimSpace(versionFile) }

type plugin struct {
	pluginv1.UnimplementedPluginServer
	pluginv1.UnimplementedMetadataProviderServer

	mu        sync.Mutex
	baseURL   string
	titleLang string
	maxCast   int
	api       *api // replaceable in tests
	cache     cache
}

func newPlugin() *plugin {
	return &plugin{baseURL: defaultBaseURL, titleLang: titleAuto, maxCast: defaultMaxCast, api: newAPI()}
}

var capabilities = []string{
	pluginv1.CapabilityMetadataMovie, pluginv1.CapabilityMetadataSeries, pluginv1.CapabilityMetadataSeriesAbsolute,
	pluginv1.CapabilityMetadataSearchHints, pluginv1.CapabilityMetadataPerson,
}

// Describe declares the capabilities. Bangumi needs no key, so the plugin works as soon as it is enabled.
func (p *plugin) Describe(_ context.Context, req *pluginv1.DescribeRequest) (*pluginv1.Manifest, error) {
	t := messages.For(req.GetLanguage())
	return &pluginv1.Manifest{
		Id: pluginID, Name: "Bangumi (unofficial)", Description: t.T("desc"), Version: version(), ProtocolVersion: pluginv1.ProtocolVersion,
		Capabilities: capabilities,
		ConfigGroups: []*pluginv1.ConfigGroup{{Key: "advanced", Title: t.T("groupAdvanced")}},
		ConfigFields: []*pluginv1.ConfigField{
			{Key: keyTitleLang, Type: pluginv1.ConfigFieldType_CONFIG_FIELD_TYPE_SELECT, DefaultValue: titleAuto,
				Label: t.T("titleLabel"), Description: t.T("titleHelp"), Options: []*pluginv1.ConfigOption{
					{Value: titleAuto, Label: t.T("titleAuto")},
					{Value: titleChinese, Label: t.T("titleChinese")},
					{Value: titleNative, Label: t.T("titleNative")},
				}},
			{Key: keyMaxCast, Type: pluginv1.ConfigFieldType_CONFIG_FIELD_TYPE_INT, DefaultValue: strconv.Itoa(defaultMaxCast), Group: "advanced",
				Min: proto.Int64(0), Max: proto.Int64(maxCastLimit), Label: t.T("maxCastLabel"), Description: t.T("maxCastHelp")},
			{Key: keyBaseURL, Type: pluginv1.ConfigFieldType_CONFIG_FIELD_TYPE_STRING, Group: "advanced", Label: t.T("baseURLLabel"),
				Description: t.T("baseURLHelp"), Placeholder: defaultBaseURL},
		},
		Attribution:          &pluginv1.Attribution{Text: attributionText, Link: attributionLink},
		SuggestedConcurrency: 1,
	}, nil
}

// Configure validates the settings. It makes no network call: Bangumi needs no credentials.
func (p *plugin) Configure(_ context.Context, req *pluginv1.ConfigureRequest) (*pluginv1.ConfigureResponse, error) {
	t := messages.For(req.GetLanguage())
	v := req.GetValues()
	resp := &pluginv1.ConfigureResponse{FieldErrors: map[string]string{}}
	base := strings.TrimSpace(v[keyBaseURL])
	if base == "" {
		base = defaultBaseURL
	}
	if u, err := url.Parse(base); err != nil || u.Scheme != "https" || u.Host == "" {
		resp.FieldErrors[keyBaseURL] = t.T("errHTTPS")
	}
	lang := strings.TrimSpace(v[keyTitleLang])
	switch lang {
	case "":
		lang = titleAuto
	case titleAuto, titleChinese, titleNative:
	default:
		resp.FieldErrors[keyTitleLang] = t.T("errTitleLang")
	}
	maxCast := defaultMaxCast
	if raw := strings.TrimSpace(v[keyMaxCast]); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 || n > maxCastLimit {
			resp.FieldErrors[keyMaxCast] = t.T("errMaxCast")
		}
		maxCast = n
	}
	if len(resp.FieldErrors) > 0 {
		return resp, nil
	}
	p.mu.Lock()
	p.baseURL, p.titleLang, p.maxCast = strings.TrimRight(base, "/"), lang, maxCast
	p.mu.Unlock()
	return resp, nil
}

type settings struct {
	baseURL, titleLang string
	maxCast            int
}

func (p *plugin) settings() settings {
	p.mu.Lock()
	defer p.mu.Unlock()
	return settings{p.baseURL, p.titleLang, p.maxCast}
}
