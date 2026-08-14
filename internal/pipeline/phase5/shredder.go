package phase5

import (
	"github.com/proofboard/proofboard/internal/crypto"
	"github.com/proofboard/proofboard/internal/model"
)

func Shred(commits []model.RawCommit, signals []model.CommitSignal) []model.SafeCommit {
	for i := range commits {
		crypto.ZeroBytes(commits[i].Subject)
		commits[i].Subject = nil
		// Defensive second pass, mirroring Subject: Body is already
		// zeroed in phase2.Classify, but this guards against any future path
		// that reaches phase5 without going through Classify first.
		crypto.ZeroBytes(commits[i].Body)
		commits[i].Body = nil
		commits[i].FilePaths = crypto.DropStrings(commits[i].FilePaths)
		commits[i].AuthorEmail = ""
		commits[i].Repository = ""
		commits[i].Organization = ""
	}

	safe := make([]model.SafeCommit, 0, len(signals))
	for _, signal := range signals {
		safe = append(safe, model.SafeCommit{
			SHA:             signal.SHA,
			TimestampUnix:   signal.Timestamp.Unix(),
			Additions:       signal.Additions,
			Deletions:       signal.Deletions,
			FilesChanged:    signal.FilesChanged,
			Category:        signal.PrimaryCategory,
			ImpactType:      signal.ImpactType,
			NoiseScore:      signal.NoiseScore,
			AuthorEmailHash: signal.AuthorEmailHash,
			SignatureValid:  signal.SignatureValid,
		})
	}
	return safe
}
