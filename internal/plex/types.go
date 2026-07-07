package plex

import "time"

type Programme struct {
	Title                 string
	Subtitle              string
	EpisodeTitle          string
	Description           string
	GUID                  string
	RatingKey             string
	Type                  string
	Year                  int
	ChannelName           string
	ChannelID             string
	AiringChannel         string
	AiringID              string
	ProgrammeID           string
	Premiere              bool
	OriginallyAvailableAt time.Time
	StartAt               time.Time
	EndAt                 time.Time
}

type ScheduledRecording struct {
	AiringID     string
	ProgrammeID  string
	Title        string
	Subtitle     string
	EpisodeTitle string
	ChannelName  string
	StartAt      time.Time
	EndAt        time.Time
}

type RecordingRequest struct {
	Title         string
	Subtitle      string
	EpisodeTitle  string
	GUID          string
	RatingKey     string
	AiringID      string
	ProgrammeID   string
	ChannelID     string
	ChannelName   string
	AiringChannel string
	StartAt       time.Time
	EndAt         time.Time
	PaddingBefore time.Duration
	PaddingAfter  time.Duration
	RuleName      string
}
