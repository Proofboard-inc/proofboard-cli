package phase4

import (
	"math"
	"sort"
	"time"

	"github.com/proofboard/proofboard/internal/model"
)

// minMilestoneClusters/maxMilestoneClusters bound the dynamic per-sync
// cluster cap computed by clusterCap: a MAX, not a target — a small
// incremental sync of a few commits yields only one or two clusters and
// never hits the cap. The cap bites mainly on a large sync (e.g. a first
// full sync of a long-lived repo), where one-cluster-per-merge-commit (a
// repo with dozens of PRs) would otherwise produce dozens of trivial
// "milestones". When natural boundaries exceed the cap, consolidate() merges
// the least-significant adjacent ones until the count fits — see consolidate.
//
// The cap used to be a flat 6 regardless of how much history a sync covered,
// which squeezed a full year of work into 6 buckets on a first full sync.
// clusterCap instead scales the cap with the sync's real time span, floored
// at minMilestoneClusters (so a short sync still behaves exactly as before)
// and capped at maxMilestoneClusters (so a huge, multi-year history doesn't
// produce an unbounded number of milestones).
const (
	minMilestoneClusters = 6
	maxMilestoneClusters = 20
	// clusterCapWeeksPerCluster is how many weeks of commit history "earn"
	// one additional cluster above the floor.
	clusterCapWeeksPerCluster = 5.0
)

// clusterCap computes the per-sync milestone cap from the real chronological
// span (in weeks) of the commits being processed, bounded to
// [minMilestoneClusters, maxMilestoneClusters].
func clusterCap(spanWeeks float64) int {
	limit := int(math.Ceil(spanWeeks / clusterCapWeeksPerCluster))
	if limit < minMilestoneClusters {
		limit = minMilestoneClusters
	}
	if limit > maxMilestoneClusters {
		limit = maxMilestoneClusters
	}
	return limit
}

// linearSegmentGapHours is the time gap (between consecutive commits, in a
// repo with no merge signal) that forces a new segment even when the category
// hasn't changed — a multi-day pause is a natural work-session boundary.
const linearSegmentGapHours = 72

func Detect(result model.ScoredResult, mergeTimestamps []int64) []model.Cluster {
	if len(result.Commits) == 0 {
		return nil
	}

	// 1. Sort commits chronologically (ascending)
	commits := make([]model.CommitSignal, len(result.Commits))
	copy(commits, result.Commits)
	sort.Slice(commits, func(i, j int) bool {
		return commits[i].Timestamp.Before(commits[j].Timestamp)
	})

	clusterGroups := groupCommits(commits, mergeTimestamps)

	// No merge-commit signal (rebase/squash workflow, or a brand-new repo):
	// fall back to content-aware segmentation by primary-category runs and
	// large time gaps, rather than an arbitrary even split. groupCommits
	// returns a single group in this case.
	if len(clusterGroups) == 1 && len(commits) >= 5 {
		clusterGroups = segmentLinear(commits)
	}

	// Cap the number of milestones per sync by consolidating the least
	// significant adjacent groups. Same-category neighbours collapse first
	// (the same feature split across several PRs), then the smallest
	// neighbours, always preserving chronological order. The cap itself
	// scales with how much real time this sync's commits span — see
	// clusterCap — rather than a flat constant, so a sync covering a year
	// of history isn't squeezed into the same handful of buckets as a
	// sync covering a week.
	spanWeeks := 0.0
	if len(commits) > 1 {
		spanWeeks = commits[len(commits)-1].Timestamp.Sub(commits[0].Timestamp).Hours() / (24 * 7)
	}
	clusterGroups = consolidate(clusterGroups, clusterCap(spanWeeks))

	clusters := make([]model.Cluster, 0, len(clusterGroups))
	for idx, clusterCommits := range clusterGroups {
		if len(clusterCommits) == 0 {
			continue
		}
		clusters = append(clusters, buildCluster(clusterCommits, idx))
	}

	return clusters
}

func groupCommits(commits []model.CommitSignal, mergeTimestamps []int64) [][]model.CommitSignal {
	groups := make(map[int][]model.CommitSignal)
	var groupKeys []int

	for _, commit := range commits {
		tUnix := commit.Timestamp.Unix()
		clusterIdx := len(mergeTimestamps)
		for idx, mt := range mergeTimestamps {
			if tUnix <= mt {
				clusterIdx = idx
				break
			}
		}
		if _, ok := groups[clusterIdx]; !ok {
			groupKeys = append(groupKeys, clusterIdx)
		}
		groups[clusterIdx] = append(groups[clusterIdx], commit)
	}

	sort.Ints(groupKeys)
	ordered := make([][]model.CommitSignal, 0, len(groupKeys))
	for _, key := range groupKeys {
		ordered = append(ordered, groups[key])
	}
	return ordered
}

// segmentLinear splits a chronologically-sorted commit list (with no
// merge-commit boundaries to work from) into runs. A new run starts whenever
// the primary category changes or a large time gap opens up — so "two weeks
// on auth, then a week on payments" becomes two segments rather than an
// arbitrary N-way even split. Runs that are too granular are re-merged later
// by consolidate().
func segmentLinear(commits []model.CommitSignal) [][]model.CommitSignal {
	if len(commits) == 0 {
		return nil
	}
	var groups [][]model.CommitSignal
	current := []model.CommitSignal{commits[0]}
	runCategory := commits[0].PrimaryCategory
	for i := 1; i < len(commits); i++ {
		c := commits[i]
		prev := current[len(current)-1]
		gap := c.Timestamp.Sub(prev.Timestamp)
		if c.PrimaryCategory != runCategory || gap > linearSegmentGapHours*time.Hour {
			groups = append(groups, current)
			current = []model.CommitSignal{c}
			runCategory = c.PrimaryCategory
		} else {
			current = append(current, c)
		}
	}
	groups = append(groups, current)
	return groups
}

// consolidate reduces groups to at most max entries by repeatedly merging the
// adjacent pair with the lowest merge cost, preserving chronological order.
// Every commit is retained (groups are only combined, never dropped), so the
// Σ cluster.CommitCount == len(commits) invariant the backend relies on holds.
func consolidate(groups [][]model.CommitSignal, max int) [][]model.CommitSignal {
	if max < 1 {
		max = 1
	}
	for len(groups) > max {
		bestIdx := 0
		bestCost := mergeCost(groups[0], groups[1])
		for i := 1; i < len(groups)-1; i++ {
			if cost := mergeCost(groups[i], groups[i+1]); cost < bestCost {
				bestCost = cost
				bestIdx = i
			}
		}
		merged := make([]model.CommitSignal, 0, len(groups[bestIdx])+len(groups[bestIdx+1]))
		merged = append(merged, groups[bestIdx]...)
		merged = append(merged, groups[bestIdx+1]...)

		next := make([][]model.CommitSignal, 0, len(groups)-1)
		next = append(next, groups[:bestIdx]...)
		next = append(next, merged)
		next = append(next, groups[bestIdx+2:]...)
		groups = next
	}
	return groups
}

// crossCategoryPenalty makes any same-category merge strictly cheaper than any
// cross-category merge, so adjacent runs of the same feature always collapse
// before two different features are combined. Large enough that no realistic
// commit count can bridge the gap.
const crossCategoryPenalty = 1_000_000

// mergeCost ranks how desirable it is to merge two adjacent groups — lower is
// more desirable. Same-category neighbours are cheapest (they are the same
// area of work), and within each tier the smallest combined pair merges first
// so large, meaningful milestones are preserved while trivial ones collapse.
func mergeCost(a, b []model.CommitSignal) int {
	combined := len(a) + len(b)
	if dominantCategoryOf(a) == dominantCategoryOf(b) {
		return combined
	}
	return crossCategoryPenalty + combined
}

type rankedCategory struct {
	name  string
	score int
}

// categoryRanking scores every category across a group of commits and
// returns them ranked highest-first, ties broken alphabetically — the single
// source of truth for both a cluster's headline category (rank 0) and its
// secondary category (rank 1, if significant — see buildCluster). Never
// returns "Unclassified"; a group with no real signal falls back to
// "Feature Development".
func categoryRanking(commits []model.CommitSignal) []rankedCategory {
	categorySums := make(map[string]int)
	for _, commit := range commits {
		for cat, score := range commit.CategoryScores {
			if cat == "Unclassified" {
				continue
			}
			categorySums[cat] += score
		}
	}
	if len(categorySums) == 0 {
		return []rankedCategory{{name: "Feature Development", score: 0}}
	}
	names := sortedKeys(categorySums)
	ranked := make([]rankedCategory, len(names))
	for i, name := range names {
		ranked[i] = rankedCategory{name: name, score: categorySums[name]}
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		return ranked[i].score > ranked[j].score
	})
	return ranked
}

// dominantCategoryOf returns the highest-scoring category across a group of
// commits — the same rule buildCluster uses for a cluster's headline
// category, so consolidation and the final label always agree.
func dominantCategoryOf(commits []model.CommitSignal) string {
	return categoryRanking(commits)[0].name
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func buildCluster(clusterCommits []model.CommitSignal, clusterIndex int) model.Cluster {
	first := clusterCommits[0].Timestamp
	last := clusterCommits[len(clusterCommits)-1].Timestamp
	additions := 0
	deletions := 0
	filesChanged := 0
	impactScores := make(map[string]int)

	for _, commit := range clusterCommits {
		additions += commit.Additions
		deletions += commit.Deletions
		filesChanged += commit.FilesChanged
		impactScores[commit.ImpactType]++
	}

	return model.Cluster{
		Category:          dominantCategoryOf(clusterCommits),
		FeatureKeyword:    dominantFeatureKeyword(clusterCommits),
		ImpactType:        dominantImpact(impactScores),
		ImpactScale:       scale(len(clusterCommits)),
		CommitCount:       len(clusterCommits),
		TotalAdditions:    additions,
		TotalDeletions:    deletions,
		TotalFilesChanged: filesChanged,
		StartTimestamp:    first.Unix(),
		EndTimestamp:      last.Unix(),
		ClusterIndex:      clusterIndex,
		ReferenceShas:     sampleReferenceShas(clusterCommits),
	}
}

// minFeatureKeywordShare is the minimum fraction of a cluster's commits that
// must share the winning feature keyword before it's trusted enough to name
// the milestone. Without this floor, a single coincidental keyword match in
// an otherwise unrelated large cluster (e.g. one commit out of 45 mentioning
// "comments" in passing) could win by default and produce a nonsense
// milestone title like "comments feature" for work that wasn't about
// comments at all.
//
// Lowered from 0.4 to 0.2 after real dogfooding on a 188-commit repo: even
// after adding file-path matching in phase2 (Classify) raised the overall
// match rate from near-zero to 76% of commits, a 23-commit cluster whose
// real theme was checkout work still landed at 5/23 = 21.7% share for
// "checkout" — correct and non-coincidental (5 independent commits, not one
// lucky match), just short of the old 0.25 floor. The minFeatureKeywordCount
// floor below is what actually guards against coincidence at this share
// level; the percentage alone doesn't need to be as strict once count is
// enforced separately.
const minFeatureKeywordShare = 0.2

// minFeatureKeywordCount is an absolute floor alongside minFeatureKeywordShare:
// for a cluster of only 2-3 commits, 25% share could be satisfied by a single
// commit, which is exactly the "one coincidental match" case the share floor
// exists to reject. Requiring at least 2 commits agreeing (when the cluster
// has more than one commit at all) closes that gap.
const minFeatureKeywordCount = 2

// dominantFeatureKeyword returns the feature keyword(s) that appear on the
// most commits in the cluster — a mode, not a score sum, since a keyword
// hitting several distinct commits is a stronger "this cluster is really
// about X" signal than one commit matching it many times. Counts every
// keyword a commit matched (model.CommitSignal.FeatureKeywords — phase2
// keeps every keyword that scored > 0 per commit, not just the top one), so
// a cluster genuinely spanning two themes (e.g. "orders" and "delivery" both
// independently well-represented) can say so instead of reporting only
// whichever one edged out the other. Falls back to the single FeatureKeyword
// field for callers/tests that only set that one.
//
// Empty if no commit in the cluster matched any feature keyword, OR if the
// winning keyword doesn't clear both minFeatureKeywordShare and
// minFeatureKeywordCount (a weak, coincidental signal shouldn't drive the
// milestone's name — the AI summary falls back to describing the milestone
// generically by category + impact instead). A second keyword is appended
// ("orders & delivery") only if it independently clears the same bar on its
// own count — a weak runner-up dilutes the name rather than adding signal.
// Ties resolve alphabetically for determinism.
func dominantFeatureKeyword(commits []model.CommitSignal) string {
	if len(commits) == 0 {
		return ""
	}
	counts := make(map[string]int)
	for _, c := range commits {
		keywords := c.FeatureKeywords
		if len(keywords) == 0 && c.FeatureKeyword != "" {
			keywords = []string{c.FeatureKeyword}
		}
		for _, kw := range keywords {
			counts[kw]++
		}
	}

	type ranked struct {
		keyword string
		count   int
	}
	candidates := make([]ranked, 0, len(counts))
	for _, kw := range sortedKeys(counts) {
		candidates = append(candidates, ranked{kw, counts[kw]})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].count > candidates[j].count
	})

	qualifies := func(r ranked) bool {
		if r.count == 0 {
			return false
		}
		if len(commits) > 1 && r.count < minFeatureKeywordCount {
			return false
		}
		return float64(r.count)/float64(len(commits)) >= minFeatureKeywordShare
	}

	if len(candidates) == 0 || !qualifies(candidates[0]) {
		return ""
	}
	best := candidates[0].keyword
	if len(candidates) > 1 && candidates[1].keyword != best && qualifies(candidates[1]) {
		return best + " & " + candidates[1].keyword
	}
	return best
}

// sampleReferenceShas returns up to 3 of this cluster's OWN commit SHAs
// (first, middle, last, chronologically) as a representative sample — never
// commits from a different cluster, unlike the backend's previous
// index-arithmetic guess.
func sampleReferenceShas(clusterCommits []model.CommitSignal) []string {
	n := len(clusterCommits)
	switch {
	case n == 0:
		return nil
	case n <= 3:
		shas := make([]string, n)
		for i, c := range clusterCommits {
			shas[i] = c.SHA
		}
		return shas
	default:
		return []string{
			clusterCommits[0].SHA,
			clusterCommits[n/2].SHA,
			clusterCommits[n-1].SHA,
		}
	}
}

func scale(commitCount int) string {
	switch {
	case commitCount > 30:
		return "large"
	case commitCount >= 10:
		return "medium"
	default:
		return "small"
	}
}

func dominantImpact(scores map[string]int) string {
	best := "feature"
	bestScore := -1
	for impact, score := range scores {
		if score > bestScore || (score == bestScore && impact < best) {
			best = impact
			bestScore = score
		}
	}
	return best
}
