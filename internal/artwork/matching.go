package artwork

import (
	"mizu/internal/matcher"
	"mizu/internal/models"
	"strings"
)

// searchTitles returns the unique, non-empty title candidates from a Sonarr series.
func searchTitles(series models.SonarrSeries) []string {
	titles := []string{
		series.Title,
		series.SortTitle,
	}
	for _, alt := range series.AlternateTitles {
		titles = append(titles, alt.Title)
	}
	return uniqueStrings(titles...)
}

// bestTitleMatch compares series titles against candidate titles from an API
// response and returns the best confidence score (0-1) and the matched title.
func bestTitleMatch(seriesTitles, candidates []string) (float64, string) {
	m := matcher.New(0.80)
	bestConf := 0.0
	bestTitle := ""
	for _, st := range seriesTitles {
		for _, ct := range candidates {
			conf := titleScore(m, st, ct)
			if conf > bestConf {
				bestConf = conf
				bestTitle = ct
			}
		}
	}
	return bestConf, bestTitle
}

func titleScore(m *matcher.Matcher, seriesTitle, mediaTitle string) float64 {
	if strings.EqualFold(seriesTitle, mediaTitle) {
		return 1.0
	}
	normSeries := m.Normalize(seriesTitle)
	normMedia := m.Normalize(mediaTitle)
	if normSeries == "" || normMedia == "" {
		return 0
	}
	if normSeries == normMedia {
		return 0.96
	}
	if matcher.SharedTokenCount(normSeries, normMedia) == 0 {
		return 0
	}
	conf := matcher.ContainmentScore(normSeries, normMedia)
	if jw := matcher.JaroWinkler(normSeries, normMedia); jw >= 0.9 && jw > conf {
		conf = jw
	}
	return conf
}

func uniqueStrings(values ...string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if t := strings.TrimSpace(v); t != "" {
			return t
		}
	}
	return ""
}
