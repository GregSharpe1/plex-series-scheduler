package plex

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"testing"
	"time"

	"github.com/GregSharpe1/plex-series-scheduler/internal/config"
)

func TestGuideFetchesProviderChannelsAndGrid(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/media/providers", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, map[string]any{
			"MediaContainer": map[string]any{
				"MediaProvider": []map[string]any{{
					"identifier": "tv.plex.providers.epg.xmltv:46",
					"protocols":  []string{"livetv"},
				}},
			},
		})
	})
	mux.HandleFunc("/tv.plex.providers.epg.xmltv:46/lineups/dvr/channels", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, map[string]any{
			"MediaContainer": map[string]any{
				"Channel": []map[string]any{{
					"id":      "chan-1",
					"key":     "030%253D030%2525205STAR",
					"gridKey": "grid-1",
					"title":   "Sky Sports F1 HD",
				}},
			},
		})
	})
	mux.HandleFunc("/tv.plex.providers.epg.xmltv:46/grid", func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Query().Get("channelGridKey"), "grid-1"; got != want {
			t.Fatalf("channelGridKey = %q, want %q", got, want)
		}
		writeJSON(t, w, map[string]any{
			"MediaContainer": map[string]any{
				"Metadata": []map[string]any{{
					"guid":             "plex://episode/episode-1",
					"ratingKey":        "episode-1",
					"grandparentTitle": "Formula 1",
					"title":            "British Grand Prix - Practice 1",
					"summary":          "Live from Silverstone",
					"type":             "episode",
					"Media": []map[string]any{{
						"id":                "airing-1",
						"beginsAt":          time.Date(2026, 7, 5, 13, 30, 0, 0, time.UTC).Unix(),
						"endsAt":            time.Date(2026, 7, 5, 14, 30, 0, 0, time.UTC).Unix(),
						"premiere":          "1",
						"channelID":         "chan-1",
						"channelIdentifier": "channel://sky-sports-f1-hd",
						"channelKey":        "030%253D030%2525205STAR",
					}},
				}},
			},
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client, err := NewHTTPClient(config.PlexConfig{URL: server.URL, Token: "token"})
	if err != nil {
		t.Fatalf("NewHTTPClient() error = %v", err)
	}

	programmes, err := client.Guide(context.Background(), 24*time.Hour)
	if err != nil {
		t.Fatalf("Guide() error = %v", err)
	}
	if got, want := len(programmes), 1; got != want {
		t.Fatalf("programme count = %d, want %d", got, want)
	}
	if got, want := programmes[0].GUID, "plex://episode/episode-1"; got != want {
		t.Fatalf("guid = %q, want %q", got, want)
	}
	if got, want := programmes[0].AiringChannel, "030%253D030%2525205STAR"; got != want {
		t.Fatalf("airingChannel = %q, want %q", got, want)
	}
	if !programmes[0].Premiere {
		t.Fatal("expected premiere flag to be parsed")
	}
}

func TestGuideIncludesNextCalendarDayFor24HourLookahead(t *testing.T) {
	requestedDates := make([]string, 0, 2)
	mux := http.NewServeMux()
	mux.HandleFunc("/media/providers", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, map[string]any{
			"MediaContainer": map[string]any{
				"MediaProvider": []map[string]any{{
					"identifier": "tv.plex.providers.epg.xmltv:46",
					"protocols":  []string{"livetv"},
				}},
			},
		})
	})
	mux.HandleFunc("/tv.plex.providers.epg.xmltv:46/lineups/dvr/channels", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, map[string]any{
			"MediaContainer": map[string]any{
				"Channel": []map[string]any{{
					"id":      "chan-1",
					"gridKey": "grid-1",
					"title":   "Sky Sports F1 HD",
				}},
			},
		})
	})
	mux.HandleFunc("/tv.plex.providers.epg.xmltv:46/grid", func(w http.ResponseWriter, r *http.Request) {
		requestedDates = append(requestedDates, r.URL.Query().Get("date"))
		writeJSON(t, w, map[string]any{
			"MediaContainer": map[string]any{
				"Metadata": []map[string]any{},
			},
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client, err := NewHTTPClient(config.PlexConfig{URL: server.URL, Token: "token"})
	if err != nil {
		t.Fatalf("NewHTTPClient() error = %v", err)
	}

	if _, err := client.Guide(context.Background(), 24*time.Hour); err != nil {
		t.Fatalf("Guide() error = %v", err)
	}

	sort.Strings(requestedDates)
	if got := len(requestedDates); got != 2 {
		t.Fatalf("requested date count = %d, want 2 (%v)", got, requestedDates)
	}
}

func TestScheduledRecordingsParsesMediaGrabOperations(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/media/subscriptions/scheduled", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, map[string]any{
			"MediaContainer": map[string]any{
				"MediaGrabOperation": []map[string]any{{
					"Metadata": map[string]any{
						"guid":             "plex://episode/episode-1",
						"ratingKey":        "episode-1",
						"grandparentTitle": "Formula 1",
						"title":            "British Grand Prix - Practice 1",
						"Media": []map[string]any{{
							"id":                "airing-1",
							"beginsAt":          time.Date(2026, 7, 5, 13, 30, 0, 0, time.UTC).Unix(),
							"channelTitle":      "Sky Sports F1 HD",
							"channelIdentifier": "channel://sky-sports-f1-hd",
						}},
					},
				}},
			},
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client, err := NewHTTPClient(config.PlexConfig{URL: server.URL, Token: "token"})
	if err != nil {
		t.Fatalf("NewHTTPClient() error = %v", err)
	}

	items, err := client.ScheduledRecordings(context.Background())
	if err != nil {
		t.Fatalf("ScheduledRecordings() error = %v", err)
	}
	if got, want := len(items), 1; got != want {
		t.Fatalf("scheduled count = %d, want %d", got, want)
	}
	if got, want := items[0].ChannelName, "Sky Sports F1 HD"; got != want {
		t.Fatalf("channelName = %q, want %q", got, want)
	}
}

func TestCreateRecordingUsesTemplateSubscription(t *testing.T) {
	var postedQuery url.Values
	mux := http.NewServeMux()
	mux.HandleFunc("/media/subscriptions/template", func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Query().Get("guid"), "plex://episode/episode-1"; got != want {
			t.Fatalf("guid = %q, want %q", got, want)
		}
		writeJSON(t, w, map[string]any{
			"MediaContainer": map[string]any{
				"SubscriptionTemplate": []map[string]any{{
					"MediaSubscription": []map[string]any{{
						"targetLibrarySectionID":  "10",
						"targetSectionLocationID": "9",
						"type":                    "4",
						"parameters":              "params%5BlibraryType%5D=2&params%5BmediaProviderID%5D=5",
						"Setting": []map[string]any{{
							"id":      "startOffsetMinutes",
							"default": "0",
						}, {
							"id":      "endOffsetMinutes",
							"default": "3",
						}},
					}},
				}},
			},
		})
	})
	mux.HandleFunc("/media/subscriptions", func(w http.ResponseWriter, r *http.Request) {
		postedQuery = r.URL.Query()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"MediaContainer":{}}`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client, err := NewHTTPClient(config.PlexConfig{URL: server.URL, Token: "token"})
	if err != nil {
		t.Fatalf("NewHTTPClient() error = %v", err)
	}

	err = client.CreateRecording(context.Background(), RecordingRequest{
		GUID:          "plex://episode/episode-1",
		RatingKey:     "episode-1",
		AiringChannel: "030%253D030%2525205STAR",
		StartAt:       time.Unix(1604066400, 0).UTC(),
		PaddingBefore: 2 * time.Minute,
		PaddingAfter:  45 * time.Minute,
	})
	if err != nil {
		t.Fatalf("CreateRecording() error = %v", err)
	}

	if got, want := postedQuery.Get("targetLibrarySectionID"), "10"; got != want {
		t.Fatalf("targetLibrarySectionID = %q, want %q", got, want)
	}
	if got, want := postedQuery.Get("prefs[startOffsetMinutes]"), "2"; got != want {
		t.Fatalf("prefs[startOffsetMinutes] = %q, want %q", got, want)
	}
	if got, want := postedQuery.Get("prefs[endOffsetMinutes]"), "45"; got != want {
		t.Fatalf("prefs[endOffsetMinutes] = %q, want %q", got, want)
	}
	if got, want := postedQuery.Get("params[airingChannels]"), "030%253D030%2525205STAR"; got != want {
		t.Fatalf("params[airingChannels] = %q, want %q", got, want)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, payload any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		t.Fatalf("encode json: %v", err)
	}
}
