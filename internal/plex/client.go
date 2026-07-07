package plex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/GregSharpe1/plex-series-scheduler/internal/config"
)

type Client interface {
	Guide(ctx context.Context, lookahead time.Duration) ([]Programme, error)
	ScheduledRecordings(ctx context.Context) ([]ScheduledRecording, error)
	CreateRecording(ctx context.Context, req RecordingRequest) error
}

type HTTPClient struct {
	baseURL                    *url.URL
	token                      string
	httpClient                 *http.Client
	recordingLibrarySectionID  string
	recordingSectionLocationID string
	clientIdentifier           string
	product                    string
	version                    string
	deviceName                 string
	platform                   string
}

type channel struct {
	ID           jsonScalar `json:"id"`
	Key          jsonScalar `json:"key"`
	GridKey      jsonScalar `json:"gridKey"`
	Title        string     `json:"title"`
	CallSign     string     `json:"callSign"`
	ChannelVCN   jsonScalar `json:"channelVcn"`
	ChannelID    jsonScalar `json:"channelID"`
	Identifier   jsonScalar `json:"channelIdentifier"`
	VideoQuality string     `json:"videoResolution"`
	IsHD         bool       `json:"isHd"`
}

type mediaProvider struct {
	Identifier         string         `json:"identifier"`
	ProviderIdentifier string         `json:"providerIdentifier"`
	Protocols          jsonValueSlice `json:"protocols"`
}

type gridMetadata struct {
	Key                   jsonScalar  `json:"key"`
	GUID                  jsonScalar  `json:"guid"`
	RatingKey             jsonScalar  `json:"ratingKey"`
	Title                 string      `json:"title"`
	Summary               string      `json:"summary"`
	Type                  string      `json:"type"`
	Year                  int         `json:"year"`
	ParentTitle           string      `json:"parentTitle"`
	GrandparentTitle      string      `json:"grandparentTitle"`
	OriginallyAvailableAt string      `json:"originallyAvailableAt"`
	Media                 []gridMedia `json:"Media"`
	Channel               []channel   `json:"Channel"`
}

type gridMedia struct {
	ID                jsonScalar     `json:"id"`
	Key               jsonScalar     `json:"key"`
	BeginsAt          int64          `json:"beginsAt"`
	EndsAt            int64          `json:"endsAt"`
	Premiere          jsonScalar     `json:"premiere"`
	ChannelID         jsonScalar     `json:"channelID"`
	ChannelIdentifier jsonScalar     `json:"channelIdentifier"`
	ChannelTitle      string         `json:"channelTitle"`
	ChannelCallSign   string         `json:"channelCallSign"`
	VideoResolution   string         `json:"videoResolution"`
	Protocol          string         `json:"protocol"`
	ChannelKey        jsonScalar     `json:"channelKey"`
	GridKey           jsonScalar     `json:"gridKey"`
	Metadata          []gridMetadata `json:"Metadata"`
}

type scheduledOperation struct {
	Metadata gridMetadata `json:"Metadata"`
}

type subscriptionTemplate struct {
	TargetLibrarySectionID  jsonScalar            `json:"targetLibrarySectionID"`
	TargetSectionLocationID jsonScalar            `json:"targetSectionLocationID"`
	Type                    jsonScalar            `json:"type"`
	Parameters              templateParameters    `json:"parameters"`
	Setting                 []subscriptionSetting `json:"Setting"`
	Metadata                *templateMetadata     `json:"Metadata"`
	Title                   string                `json:"title"`
}

type subscriptionSetting struct {
	ID      string     `json:"id"`
	Value   jsonScalar `json:"value"`
	Default jsonScalar `json:"default"`
}

type templateMetadata struct {
	GUID      string `json:"guid"`
	RatingKey string `json:"ratingKey"`
	Title     string `json:"title"`
	Year      int    `json:"year"`
}

type mediaContainer[T any] struct {
	MediaContainer T `json:"MediaContainer"`
}

type metadataContainer struct {
	Metadata []gridMetadata `json:"Metadata"`
}

type channelContainer struct {
	Channel []channel `json:"Channel"`
}

type providerContainer struct {
	MediaProvider []mediaProvider `json:"MediaProvider"`
}

type scheduledContainer struct {
	MediaGrabOperation []scheduledOperation `json:"MediaGrabOperation"`
}

type templateContainer struct {
	MediaSubscription    []subscriptionTemplate    `json:"MediaSubscription"`
	SubscriptionTemplate []nestedTemplateContainer `json:"SubscriptionTemplate"`
}

type nestedTemplateContainer struct {
	MediaSubscription []subscriptionTemplate `json:"MediaSubscription"`
}

type jsonValueSlice []string

type jsonScalar string

type templateParameters url.Values

func NewHTTPClient(cfg config.PlexConfig) (*HTTPClient, error) {
	return NewHTTPClientWithHTTPClient(cfg, &http.Client{Timeout: 30 * time.Second})
}

func NewHTTPClientWithHTTPClient(cfg config.PlexConfig, httpClient *http.Client) (*HTTPClient, error) {
	baseURL, err := url.Parse(strings.TrimRight(cfg.URL, "/"))
	if err != nil {
		return nil, fmt.Errorf("parse plex url: %w", err)
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	return &HTTPClient{
		baseURL:                    baseURL,
		token:                      cfg.Token,
		httpClient:                 httpClient,
		recordingLibrarySectionID:  strings.TrimSpace(cfg.RecordingLibrarySectionID),
		recordingSectionLocationID: strings.TrimSpace(cfg.RecordingSectionLocationID),
		clientIdentifier:           "plex-series-scheduler",
		product:                    "Plex Series Scheduler",
		version:                    "0.1.0",
		deviceName:                 "plex-series-scheduler",
		platform:                   "linux",
	}, nil
}

func (c *HTTPClient) Guide(ctx context.Context, lookahead time.Duration) ([]Programme, error) {
	providerID, err := c.providerIdentifier(ctx)
	if err != nil {
		return nil, err
	}

	channels, err := c.channels(ctx, providerID)
	if err != nil {
		return nil, err
	}
	if len(channels) == 0 {
		return nil, nil
	}

	programmes := make([]Programme, 0, len(channels)*4)
	seen := make(map[string]struct{})
	now := time.Now().UTC()
	end := now.Add(lookahead)
	startDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	endDay := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)
	days := int(endDay.Sub(startDay)/(24*time.Hour)) + 1

	for _, ch := range channels {
		channelGridKey := firstNonEmpty(ch.GridKey.String(), ch.ID.String(), ch.Key.String())
		if channelGridKey == "" {
			continue
		}

		for day := 0; day < days; day++ {
			date := startDay.AddDate(0, 0, day).Format("2006-01-02")
			items, err := c.grid(ctx, providerID, channelGridKey, date)
			if err != nil {
				return nil, err
			}

			for _, item := range items {
				for _, programme := range programmesFromGrid(item, ch) {
					key := programme.AiringID + "|" + strconv.FormatInt(programme.StartAt.Unix(), 10)
					if _, ok := seen[key]; ok {
						continue
					}
					seen[key] = struct{}{}
					programmes = append(programmes, programme)
				}
			}
		}
	}

	return programmes, nil
}

func (c *HTTPClient) ScheduledRecordings(ctx context.Context) ([]ScheduledRecording, error) {
	var payload mediaContainer[scheduledContainer]
	if err := c.get(ctx, "/media/subscriptions/scheduled", nil, &payload); err != nil {
		return nil, fmt.Errorf("fetch scheduled recordings: %w", err)
	}

	items := make([]ScheduledRecording, 0, len(payload.MediaContainer.MediaGrabOperation))
	for _, op := range payload.MediaContainer.MediaGrabOperation {
		metadata := op.Metadata
		for _, media := range metadata.Media {
			startAt := unixTime(media.BeginsAt)
			items = append(items, ScheduledRecording{
				AiringID:     firstNonEmpty(media.ID.String(), media.Key.String(), metadata.RatingKey.String(), metadata.Key.String()),
				ProgrammeID:  firstNonEmpty(metadata.RatingKey.String(), metadata.GUID.String(), metadata.Key.String()),
				Title:        titleFor(metadata),
				Subtitle:     subtitleFor(metadata),
				EpisodeTitle: episodeTitleFor(metadata),
				ChannelName:  channelNameFor(media, channel{}),
				StartAt:      startAt.UTC(),
				EndAt:        unixTime(media.EndsAt).UTC(),
			})
		}
	}

	return items, nil
}

func (c *HTTPClient) CreateRecording(ctx context.Context, req RecordingRequest) error {
	if req.GUID == "" {
		return fmt.Errorf("recording request is missing programme guid")
	}

	template, err := c.subscriptionTemplate(ctx, req.GUID)
	if err != nil {
		return err
	}

	values := url.Values{}
	values.Set("includeGrabs", "1")
	if target := firstNonEmpty(c.recordingLibrarySectionID, template.TargetLibrarySectionID.String()); target != "" {
		values.Set("targetLibrarySectionID", target)
	}
	if target := firstNonEmpty(c.recordingSectionLocationID, template.TargetSectionLocationID.String()); target != "" {
		values.Set("targetSectionLocationID", target)
	}
	if subscriptionType := template.Type.String(); subscriptionType != "" {
		values.Set("type", subscriptionType)
	}

	for key, entries := range template.Parameters.Values() {
		for _, entry := range entries {
			values.Set(key, entry)
		}
	}
	for _, setting := range template.Setting {
		value := setting.Value.String()
		if value == "" {
			value = setting.Default.String()
		}
		if value == "" {
			continue
		}
		values.Set(fmt.Sprintf("prefs[%s]", setting.ID), value)
	}

	if req.PaddingBefore != 0 {
		values.Set("prefs[startOffsetMinutes]", strconv.FormatInt(int64(req.PaddingBefore/time.Minute), 10))
		values.Set("prefs[oneShot]", "true")
	}
	if req.PaddingAfter != 0 {
		values.Set("prefs[endOffsetMinutes]", strconv.FormatInt(int64(req.PaddingAfter/time.Minute), 10))
		values.Set("prefs[oneShot]", "true")
	}
	if req.StartAt.Unix() > 0 {
		values.Set("params[airingTimes]", strconv.FormatInt(req.StartAt.Unix(), 10))
	}
	if req.AiringChannel != "" {
		values.Set("params[airingChannels]", req.AiringChannel)
	}
	if req.GUID != "" {
		values.Set("hints[guid]", req.GUID)
	}
	if req.RatingKey != "" {
		values.Set("hints[ratingKey]", req.RatingKey)
	}

	if err := c.post(ctx, "/media/subscriptions", values, nil); err != nil {
		return fmt.Errorf("create recording subscription: %w", err)
	}
	return nil
}

func (c *HTTPClient) providerIdentifier(ctx context.Context) (string, error) {
	var payload mediaContainer[providerContainer]
	if err := c.get(ctx, "/media/providers", nil, &payload); err != nil {
		return "", fmt.Errorf("fetch media providers: %w", err)
	}

	for _, provider := range payload.MediaContainer.MediaProvider {
		if strings.Contains(provider.Identifier, "tv.plex.providers.epg.") && provider.Protocols.Contains("livetv") {
			return provider.Identifier, nil
		}
	}
	for _, provider := range payload.MediaContainer.MediaProvider {
		if strings.Contains(provider.Identifier, "tv.plex.providers.epg.") {
			return provider.Identifier, nil
		}
		if strings.Contains(provider.ProviderIdentifier, "tv.plex.providers.epg.") {
			return provider.ProviderIdentifier, nil
		}
	}

	return "", fmt.Errorf("no live tv provider found")
}

func (c *HTTPClient) channels(ctx context.Context, providerID string) ([]channel, error) {
	var payload mediaContainer[channelContainer]
	endpoint := "/" + providerID + "/lineups/dvr/channels"
	if err := c.get(ctx, endpoint, nil, &payload); err != nil {
		return nil, fmt.Errorf("fetch provider channels: %w", err)
	}
	return payload.MediaContainer.Channel, nil
}

func (c *HTTPClient) grid(ctx context.Context, providerID, channelGridKey, date string) ([]gridMetadata, error) {
	var payload mediaContainer[metadataContainer]
	endpoint := "/" + providerID + "/grid"
	query := url.Values{}
	query.Set("channelGridKey", channelGridKey)
	query.Set("date", date)
	if err := c.get(ctx, endpoint, query, &payload); err != nil {
		return nil, fmt.Errorf("fetch guide grid for %s on %s: %w", channelGridKey, date, err)
	}
	return payload.MediaContainer.Metadata, nil
}

func (c *HTTPClient) subscriptionTemplate(ctx context.Context, guid string) (subscriptionTemplate, error) {
	var payload mediaContainer[templateContainer]
	query := url.Values{}
	query.Set("guid", guid)
	if err := c.get(ctx, "/media/subscriptions/template", query, &payload); err != nil {
		return subscriptionTemplate{}, fmt.Errorf("fetch subscription template: %w", err)
	}
	if len(payload.MediaContainer.MediaSubscription) > 0 {
		return payload.MediaContainer.MediaSubscription[0], nil
	}
	if len(payload.MediaContainer.SubscriptionTemplate) > 0 && len(payload.MediaContainer.SubscriptionTemplate[0].MediaSubscription) > 0 {
		return payload.MediaContainer.SubscriptionTemplate[0].MediaSubscription[0], nil
	}
	if len(payload.MediaContainer.MediaSubscription) == 0 {
		return subscriptionTemplate{}, fmt.Errorf("subscription template for guid %q returned no media subscriptions", guid)
	}
	return payload.MediaContainer.MediaSubscription[0], nil
}

func (c *HTTPClient) get(ctx context.Context, endpoint string, query url.Values, out any) error {
	return c.do(ctx, http.MethodGet, endpoint, query, out)
}

func (c *HTTPClient) post(ctx context.Context, endpoint string, query url.Values, out any) error {
	return c.do(ctx, http.MethodPost, endpoint, query, out)
}

func (c *HTTPClient) do(ctx context.Context, method, endpoint string, query url.Values, out any) error {
	requestURL := *c.baseURL
	requestURL.Path = path.Join(c.baseURL.Path, endpoint)
	requestURL.RawPath = requestURL.Path
	if query != nil {
		requestURL.RawQuery = query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, requestURL.String(), nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	c.addHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("perform %s %s: %w", method, endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
		return fmt.Errorf("unexpected status %d from %s %s: %s", resp.StatusCode, method, endpoint, strings.TrimSpace(string(body)))
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response for %s %s: %w", method, endpoint, err)
	}
	return nil
}

func (c *HTTPClient) addHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Plex-Token", c.token)
	req.Header.Set("X-Plex-Client-Identifier", c.clientIdentifier)
	req.Header.Set("X-Plex-Product", c.product)
	req.Header.Set("X-Plex-Version", c.version)
	req.Header.Set("X-Plex-Device-Name", c.deviceName)
	req.Header.Set("X-Plex-Platform", c.platform)
}

func programmesFromGrid(item gridMetadata, ch channel) []Programme {
	out := make([]Programme, 0, len(item.Media))
	if len(item.Media) == 0 {
		item.Media = []gridMedia{{}}
	}
	for _, media := range item.Media {
		startAt := unixTime(media.BeginsAt)
		endAt := unixTime(media.EndsAt)
		channelName := channelNameFor(media, ch)
		airingID := firstNonEmpty(media.ID.String(), media.Key.String(), item.RatingKey.String(), item.Key.String())
		programmeID := firstNonEmpty(item.RatingKey.String(), item.GUID.String(), item.Key.String())
		airingChannel := firstNonEmpty(media.ChannelKey.String(), media.ChannelIdentifier.String(), ch.Key.String(), ch.ID.String())
		originallyAvailableAt := parseOriginalAvailability(item.OriginallyAvailableAt)

		out = append(out, Programme{
			Title:                 titleFor(item),
			Subtitle:              subtitleFor(item),
			EpisodeTitle:          episodeTitleFor(item),
			Description:           item.Summary,
			GUID:                  item.GUID.String(),
			RatingKey:             item.RatingKey.String(),
			Type:                  item.Type,
			Year:                  item.Year,
			ChannelName:           channelName,
			ChannelID:             firstNonEmpty(media.ChannelID.String(), ch.ChannelID.String(), ch.ID.String()),
			AiringChannel:         airingChannel,
			AiringID:              airingID,
			ProgrammeID:           programmeID,
			Premiere:              media.Premiere.Bool(),
			OriginallyAvailableAt: originallyAvailableAt,
			StartAt:               startAt.UTC(),
			EndAt:                 endAt.UTC(),
		})
	}
	return out
}

func titleFor(metadata gridMetadata) string {
	if metadata.GrandparentTitle != "" {
		return metadata.GrandparentTitle
	}
	return metadata.Title
}

func subtitleFor(metadata gridMetadata) string {
	if metadata.GrandparentTitle != "" {
		return metadata.Title
	}
	return metadata.ParentTitle
}

func episodeTitleFor(metadata gridMetadata) string {
	if metadata.GrandparentTitle != "" {
		return metadata.Title
	}
	return ""
}

func channelNameFor(media gridMedia, ch channel) string {
	return firstNonEmpty(media.ChannelTitle, media.ChannelCallSign, ch.Title, ch.CallSign, ch.ChannelVCN.String())
}

func unixTime(value int64) time.Time {
	if value <= 0 {
		return time.Time{}
	}
	return time.Unix(value, 0)
}

func parseOriginalAvailability(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	t, err := time.ParseInLocation("2006-01-02 15:04:05", value, time.UTC)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func stringifyAny(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprint(value)
	}
}

func (s *jsonValueSlice) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	var list []string
	if err := json.Unmarshal(data, &list); err == nil {
		*s = list
		return nil
	}
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		*s = []string{single}
		return nil
	}
	return fmt.Errorf("unsupported protocols payload: %s", string(data))
}

func (p *templateParameters) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*p = nil
		return nil
	}
	var encoded string
	if err := json.Unmarshal(data, &encoded); err == nil {
		values, err := url.ParseQuery(encoded)
		if err != nil {
			return fmt.Errorf("parse template parameters query: %w", err)
		}
		*p = templateParameters(values)
		return nil
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err == nil {
		values := url.Values{}
		for key, value := range raw {
			values.Set(key, stringifyAny(value))
		}
		*p = templateParameters(values)
		return nil
	}

	return fmt.Errorf("unsupported template parameters payload: %s", string(data))
}

func (p templateParameters) Values() url.Values {
	return url.Values(p)
}

func (s jsonValueSlice) Contains(value string) bool {
	for _, candidate := range s {
		if strings.EqualFold(candidate, value) {
			return true
		}
	}
	return false
}

func (s *jsonScalar) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*s = ""
		return nil
	}
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		*s = jsonScalar(str)
		return nil
	}
	var num json.Number
	if err := json.Unmarshal(data, &num); err == nil {
		*s = jsonScalar(num.String())
		return nil
	}
	var boolean bool
	if err := json.Unmarshal(data, &boolean); err == nil {
		if boolean {
			*s = "true"
		} else {
			*s = "false"
		}
		return nil
	}
	return fmt.Errorf("unsupported scalar payload: %s", string(data))
}

func (s jsonScalar) String() string {
	return string(s)
}

func (s jsonScalar) Bool() bool {
	value := strings.TrimSpace(strings.ToLower(string(s)))
	switch value {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}
