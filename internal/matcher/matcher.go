package matcher

import (
	"context"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"mizu/internal/models"

	"github.com/rs/zerolog/log"
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
		// names are the full title, the title without its bracketed
		// alternate name, and that name: AFL writes "Fire Force (Enen no
		// Shouboutai)", Sonarr "Fire Force".
		names []string
	}
	normAFLShows := make([]normAFL, len(aflShows))
	for i, s := range aflShows {
		normAFLShows[i] = normAFL{show: s, norm: normalize(s.Title), names: titleNames(s.Title)}
	}

	for _, na := range normAFLShows {
		best := MatchResult{AFLShow: na.show}
		// An AFL entry for another year is another series (the 1981
		// Urusei Yatsura isn't the 2022 remake), however alike the names.
		if yearsConflict(na.show.Title, series) {
			continue
		}

		for _, sonarrTitle := range sonarrTitles {
			if strings.EqualFold(sonarrTitle, na.show.Title) {
				best = MatchResult{AFLShow: na.show, Confidence: 1.0, Method: "exact"}
				break
			}

			normSonarr := normalize(sonarrTitle)
			for _, name := range na.names {
				if normSonarr == name {
					if best.Confidence < 0.95 {
						best = MatchResult{AFLShow: na.show, Confidence: 0.95, Method: "normalized"}
					}
					continue
				}

				sharedTokens := SharedTokenCount(normSonarr, name)
				if sharedTokens == 0 {
					continue
				}

				// A word the Sonarr title has and the AFL one doesn't ("Wing",
				// "II", "Brotherhood", "Kai") usually names another series of the
				// same franchise, so such a match stays below the threshold.
				ceiling := 1.0
				if hasDistinguishingWord(normSonarr, na.norm) {
					ceiling = distinguishedCeiling
				}

				containScore := min(ContainmentScore(normSonarr, name), ceiling)
				if containScore > best.Confidence {
					best = MatchResult{AFLShow: na.show, Confidence: containScore, Method: "containment"}
				}

				// One shared franchise token like "Digimon" is too weak unless the
				// remaining tokens are also strongly aligned (for example typos).
				if !hasStrongFuzzyTokenAgreement(normSonarr, name, sharedTokens) {
					continue
				}

				jw := jaroWinkler(normSonarr, name)
				if jw >= 0.88 && min(jw, ceiling) > best.Confidence {
					best = MatchResult{AFLShow: na.show, Confidence: min(jw, ceiling), Method: "fuzzy"}
				}
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

// distinguishedCeiling caps a match whose Sonarr title has a word the AFL
// title lacks, below the 0.82 the sync engine auto-matches at.
const distinguishedCeiling = 0.8

// yearPattern finds a year in a title, as in "Urusei Yatsura (1981)".
var yearPattern = regexp.MustCompile(`\b(19[5-9]\d|20\d\d)\b`)

// yearsConflict reports whether the AFL title names years and the series
// is from none of them (a year either way is allowed for late or early
// listings).
func yearsConflict(aflTitle string, series models.SonarrSeries) bool {
	aflYears := yearPattern.FindAllString(aflTitle, -1)
	if len(aflYears) == 0 {
		return false
	}
	seriesYears := []int{}
	if series.Year > 0 {
		seriesYears = append(seriesYears, series.Year)
	}
	for _, y := range yearPattern.FindAllString(series.Title, -1) {
		n, _ := strconv.Atoi(y)
		seriesYears = append(seriesYears, n)
	}
	if len(seriesYears) == 0 {
		return false
	}
	for _, a := range aflYears {
		ay, _ := strconv.Atoi(a)
		for _, sy := range seriesYears {
			if ay-sy <= 1 && sy-ay <= 1 {
				return false
			}
		}
	}
	return true
}

// hasDistinguishingWord reports whether the normalized Sonarr title has a
// word (other than a year) that the normalized AFL title has nothing close
// to; a spelling variant ("shippuuden", "shippuden") doesn't count.
func hasDistinguishingWord(sonarrNorm, aflNorm string) bool {
	aflWords := strings.Fields(aflNorm)
	for _, w := range strings.Fields(sonarrNorm) {
		// One-letter words ("x" in Hunter x Hunter, whose AFL title has
		// "×") and years say nothing about which series it is.
		if len(w) < 2 || yearPattern.MatchString(w) {
			continue
		}
		near := false
		for _, a := range aflWords {
			if a == w || jaroWinkler(a, w) >= 0.9 {
				near = true
				break
			}
		}
		if !near {
			return true
		}
	}
	return false
}

// titleNames is a title's normalized forms: whole, without a bracketed
// alternate name, and the alternate name alone.
func titleNames(title string) []string {
	names := []string{normalize(title)}
	if i := strings.Index(title, "("); i > 0 {
		if j := strings.LastIndex(title, ")"); j > i+1 {
			for _, n := range []string{normalize(title[:i]), normalize(title[i+1 : j])} {
				if n != "" && !yearPattern.MatchString(n) {
					names = append(names, n)
				}
			}
		}
	}
	return names
}
