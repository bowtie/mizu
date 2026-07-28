package matcher

import (
	"context"
	"sort"
	"strings"
	"unicode"

	"github.com/rs/zerolog/log"
	"mizu/internal/models"
)

// MatchResult represents a potential match between a Sonarr series and an AFL show.
type MatchResult struct {
	AFLShow    models.AFLShow `json:"afl_show"`
	Confidence float64        `json:"confidence"` // 0.0 - 1.0
	Method     string         `json:"method"`     // "exact", "normalized", "fuzzy", "alternate"
}

// Matcher handles matching Sonarr series to AnimeFillerList shows.
type Matcher struct {
	minConfidence float64
}

// New creates a new Matcher with the given minimum confidence threshold.
func New(minConfidence float64) *Matcher {
	if minConfidence <= 0 {
		minConfidence = 0.82
	}
	return &Matcher{minConfidence: minConfidence}
}

// FindMatches returns ranked match candidates for a Sonarr series against the AFL show list.
func (m *Matcher) FindMatches(ctx context.Context, series models.SonarrSeries, aflShows []models.AFLShow) []MatchResult {
	var results []MatchResult

	sonarrTitles := []string{series.Title}
	for _, alt := range series.AlternateTitles {
		if alt.Title != "" {
			sonarrTitles = append(sonarrTitles, alt.Title)
		}
	}

	// Pre-normalize AFL titles once to avoid re-normalizing per Sonarr title.
	type normAFL struct {
		show models.AFLShow
		norm string
	}
	normAFLShows := make([]normAFL, len(aflShows))
	for i, s := range aflShows {
		normAFLShows[i] = normAFL{show: s, norm: normalize(s.Title)}
	}

	for _, na := range normAFLShows {
		best := MatchResult{AFLShow: na.show}

		for _, sonarrTitle := range sonarrTitles {
			if strings.EqualFold(sonarrTitle, na.show.Title) {
				best = MatchResult{AFLShow: na.show, Confidence: 1.0, Method: "exact"}
				break
			}

			normSonarr := normalize(sonarrTitle)
			if normSonarr == na.norm {
				if best.Confidence < 0.95 {
					best = MatchResult{AFLShow: na.show, Confidence: 0.95, Method: "normalized"}
				}
				continue
			}

			sharedTokens := SharedTokenCount(normSonarr, na.norm)
			if sharedTokens == 0 {
				continue
			}

			containScore := ContainmentScore(normSonarr, na.norm)
			if containScore > best.Confidence {
				best = MatchResult{AFLShow: na.show, Confidence: containScore, Method: "containment"}
			}

			// One shared franchise token like "Digimon" is too weak unless the
			// remaining tokens are also strongly aligned (for example typos).
			if !hasStrongFuzzyTokenAgreement(normSonarr, na.norm, sharedTokens) {
				continue
			}

			jw := jaroWinkler(normSonarr, na.norm)
			if jw >= 0.88 && jw > best.Confidence {
				best = MatchResult{AFLShow: na.show, Confidence: jw, Method: "fuzzy"}
			}
		}

		if best.Confidence >= m.minConfidence {
			results = append(results, best)
		}
	}

	sortResults(results)
	if len(results) > 10 {
		results = results[:10]
	}

	log.Debug().
		Str("sonarr_title", series.Title).
		Int("candidates", len(results)).
		Msg("match results")

	return results
}

// BestMatch returns the single best match if it exceeds the confidence threshold.
func (m *Matcher) BestMatch(ctx context.Context, series models.SonarrSeries, aflShows []models.AFLShow) *MatchResult {
	matches := m.FindMatches(ctx, series, aflShows)
	if len(matches) == 0 {
		return nil
	}
	if matches[0].Confidence >= m.minConfidence {
		return &matches[0]
	}
	return nil
}

// MapEpisodeNumber maps a Sonarr episode to an AFL episode number,
// accounting for the episode offset and absolute vs seasonal numbering.
func MapEpisodeNumber(sonarrEp models.SonarrEpisode, offset int) int {
	if sonarrEp.AbsoluteEpisodeNumber != nil && *sonarrEp.AbsoluteEpisodeNumber > 0 {
		return *sonarrEp.AbsoluteEpisodeNumber + offset
	}
	return sonarrEp.EpisodeNumber + offset
}

// --- String Normalization ---

func (m *Matcher) Normalize(s string) string {
	return normalize(s)
}

func JaroWinkler(s1, s2 string) float64 {
	return jaroWinkler(s1, s2)
}

func normalize(s string) string {
	s = strings.ToLower(s)

	for _, sep := range []string{"-", "_", ":", "~", ".", ",", "!", "?", "'", "\"", "(", ")"} {
		s = strings.ReplaceAll(s, sep, " ")
	}

	var sb strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' {
			sb.WriteRune(r)
		}
	}
	s = sb.String()

	noise := []string{" the ", " a ", " an "}
	for _, n := range noise {
		s = strings.ReplaceAll(" "+s+" ", n, " ")
	}

	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}

func ContainmentScore(a, b string) float64 {
	if a == "" || b == "" {
		return 0
	}

	if strings.Contains(a, b) {
		return float64(len(b)) / float64(len(a)) * 0.9
	}
	if strings.Contains(b, a) {
		return float64(len(a)) / float64(len(b)) * 0.9
	}

	return 0
}

func SharedTokenCount(a, b string) int {
	if a == "" || b == "" {
		return 0
	}

	seen := make(map[string]struct{})
	for _, token := range strings.Fields(a) {
		if len(token) < 2 {
			continue
		}
		seen[token] = struct{}{}
	}

	shared := 0
	used := make(map[string]struct{})
	for _, token := range strings.Fields(b) {
		if len(token) < 2 {
			continue
		}
		if _, ok := used[token]; ok {
			continue
		}
		if _, ok := seen[token]; ok {
			shared++
			used[token] = struct{}{}
		}
	}

	return shared
}

func hasStrongFuzzyTokenAgreement(a, b string, sharedTokens int) bool {
	if sharedTokens >= 2 {
		return true
	}

	aTokens := strings.Fields(a)
	bTokens := strings.Fields(b)
	if len(aTokens) < 2 || len(bTokens) < 2 || len(aTokens) != len(bTokens) {
		return false
	}

	used := make([]bool, len(bTokens))
	similar := 0

	for _, aToken := range aTokens {
		bestIndex := -1
		bestScore := 0.0

		for i, bToken := range bTokens {
			if used[i] {
				continue
			}

			score := jaroWinkler(aToken, bToken)
			if score > bestScore {
				bestScore = score
				bestIndex = i
			}
		}

		if bestIndex >= 0 && bestScore >= 0.92 {
			used[bestIndex] = true
			similar++
		}
	}

	return similar == len(aTokens)
}

// --- Jaro-Winkler Similarity ---

func jaroWinkler(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}
	r1 := []rune(s1)
	r2 := []rune(s2)
	if len(r1) == 0 || len(r2) == 0 {
		return 0.0
	}

	jaro := jaroDistance(r1, r2)

	prefixLen := 0
	for i := 0; i < len(r1) && i < len(r2) && i < 4; i++ {
		if r1[i] == r2[i] {
			prefixLen++
		} else {
			break
		}
	}

	p := 0.1
	return jaro + float64(prefixLen)*p*(1.0-jaro)
}

func jaroDistance(r1, r2 []rune) float64 {
	if string(r1) == string(r2) {
		return 1.0
	}

	l1, l2 := len(r1), len(r2)
	if l1 == 0 || l2 == 0 {
		return 0.0
	}

	matchDist := 0
	if l1 > l2 {
		matchDist = l1/2 - 1
	} else {
		matchDist = l2/2 - 1
	}
	if matchDist < 0 {
		matchDist = 0
	}

	s1Matches := make([]bool, l1)
	s2Matches := make([]bool, l2)

	matches := 0
	transpositions := 0

	for i := 0; i < l1; i++ {
		start := i - matchDist
		if start < 0 {
			start = 0
		}
		end := i + matchDist + 1
		if end > l2 {
			end = l2
		}

		for j := start; j < end; j++ {
			if s2Matches[j] || r1[i] != r2[j] {
				continue
			}
			s1Matches[i] = true
			s2Matches[j] = true
			matches++
			break
		}
	}

	if matches == 0 {
		return 0.0
	}

	k := 0
	for i := 0; i < l1; i++ {
		if !s1Matches[i] {
			continue
		}
		for !s2Matches[k] {
			k++
		}
		if r1[i] != r2[k] {
			transpositions++
		}
		k++
	}

	m := float64(matches)
	return (m/float64(l1) + m/float64(l2) + (m-float64(transpositions)/2.0)/m) / 3.0
}

func sortResults(results []MatchResult) {
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Confidence > results[j].Confidence
	})
}
