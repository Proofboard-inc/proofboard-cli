package model

import "time"

type RawCommit struct {
	SHA          string
	Timestamp    time.Time
	Additions    int
	Deletions    int
	FilesChanged int
	Subject        []byte
	FilePaths      []string
	AuthorEmail    string
	Repository     string
	Organization   string
	SignatureValid bool
}

type CommitSignal struct {
	SHA             string
	Timestamp       time.Time
	Additions       int
	Deletions       int
	FilesChanged    int
	AuthorEmailHash string
	CategoryScores  map[string]int
	PrimaryCategory string
	ImpactType      string
	NoiseScore      float64
	SignatureValid  bool
}

type SafeCommit struct {
	SHA             string
	TimestampUnix   int64
	Additions       int
	Deletions       int
	FilesChanged    int
	Category        string
	ImpactType      string
	NoiseScore      float64
	AuthorEmailHash string
	SignatureValid  bool
}
