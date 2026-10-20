package main

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	pluginv1 "github.com/kryspal/plugin-sdk/pluginv1"
	"github.com/kryspal/plugin-sdk/sdk"
)

const (
	defaultLimit = 10
	maxQueries   = 3
	goodMatch    = 0.9
	// subjectTypeAnime is Bangumi's subject type for anime.
	subjectTypeAnime = 2
)

// fold lowercases and reduces a title to letters and digits, so punctuation, spacing and width do not matter. NFKC first, so a Roman numeral
// character (Ⅱ) becomes letters and full-width letters and digits become plain ones.
func fold(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(norm.NFKC.String(s)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// movieMarkerRE is the word Bangumi puts in front of a film's name ("theatrical version" in Chinese or Japanese, "movie" in Japanese); a
// library folder rarely has it, so films are also compared without it.
var movieMarkerRE = regexp.MustCompile(`^(?:剧场版|劇場版|映画|电影|電影)\s*[:：]?\s*`)

func stripMovieMarker(s string) string {
	return strings.TrimSpace(movieMarkerRE.ReplaceAllString(strings.TrimSpace(s), ""))
}

func bigrams(s string) map[string]int {
	r := []rune(s)
	out := map[string]int{}
	for i := 0; i+1 < len(r); i++ {
		out[string(r[i:i+2])]++
	}
	return out
}

// titleScore compares a query with one candidate title (both folded): identical 1, the query extends the candidate 0.95 (when the
// candidate is long enough to be a real title), the candidate extends the query 0.85, contained 0.8, otherwise bigram similarity up to 0.5.
func titleScore(q, c string) (float64, string) {
	switch {
	case q == "" || c == "":
		return 0, "title.weak"
	case q == c:
		return 1, "title.exact"
	case strings.HasPrefix(c, q):
		return 0.85, "title.prefix"
	case strings.HasPrefix(q, c) && len([]rune(c)) >= 3:
		return 0.95, "title.prefix"
	case strings.Contains(c, q) || (len([]rune(c)) >= 3 && strings.Contains(q, c)):
		return 0.8, "title.partial"
	}
	a, b := bigrams(q), bigrams(c)
	var common, total int
	for k, n := range a {
		total += n
		common += min(n, b[k])
	}
	for _, n := range b {
		total += n
	}
	if total == 0 {
		return 0, "title.weak"
	}
	return 0.5 * 2 * float64(common) / float64(total), "title.weak"
}

// yearAdjust is the deduction for a year that differs from the local one; exact is whether the title matched exactly. A candidate without a
// date cannot confirm the year the library gives, so it loses a little: enough to keep a partial title match from being applied on its own.
func yearAdjust(want, have int, exact bool) (float64, string) {
	if want == 0 {
		return 0, "year.unknown"
	}
	if have == 0 {
		return -0.1, "year.missing"
	}
	d := max(want-have, have-want)
	switch {
	case d == 0:
		return 0, "year.exact"
	case d == 1:
		return -0.03, "year.diff1"
	case d == 2 && exact:
		return -0.06, "year.diff2"
	case d == 2:
		return -0.15, "year.diff2"
	}
	return -0.3, "year.far"
}

type scored struct {
	s       *bgmSubject
	score   float64
	matched string
	reasons []string
}

// scoreSubject scores one subject against the queries. A film's names are also compared without Bangumi's leading "theatrical version"
// word, and a name that is the query followed by the requested year (how Bangumi names a remake, "Title 2009") counts as identical.
func scoreSubject(s *bgmSubject, queries []string, year int, movie bool) scored {
	best := scored{s: s}
	suffix := ""
	if year > 0 {
		suffix = strconv.Itoa(year)
	}
	for _, q := range queries {
		fq := fold(q)
		if movie {
			fq = fold(stripMovieMarker(q))
		}
		for _, t := range s.allTitles() {
			ft := fold(t)
			if movie {
				ft = fold(stripMovieMarker(t))
			}
			sc, why := titleScore(fq, ft)
			if suffix != "" && ft == fq+suffix && fq != "" {
				sc, why = 1, "title.exact_year"
			}
			if sc > best.score {
				best.score, best.matched, best.reasons = sc, t, []string{why}
			}
		}
	}
	adj, why := yearAdjust(year, s.year(), best.score >= 1)
	best.score = max(0, min(1, best.score+adj))
	best.reasons = append(best.reasons, why)
	return best
}

func isExactReason(r string) bool { return strings.HasPrefix(r, "title.exact") }

// queryTitles are the titles to search with: the name, then the other hinted titles by weight (variants excluded).
func queryTitles(req *pluginv1.SearchRequest) []string {
	var out []string
	seen := map[string]bool{}
	add := func(s string) {
		s = strings.TrimSpace(s)
		if k := fold(s); k != "" && !seen[k] && len(out) < maxQueries {
			seen[k] = true
			out = append(out, s)
		}
	}
	add(req.GetName())
	for _, h := range req.GetTitles() {
		if h.GetRole() != pluginv1.TitleRole_TITLE_ROLE_VARIANT {
			add(h.GetText())
		}
	}
	return out
}

// Search implements pluginv1.MetadataProviderServer.
func (p *plugin) Search(ctx context.Context, req *pluginv1.SearchRequest) (*pluginv1.SearchResponse, error) {
	set := p.settings()
	movie := req.GetType() == pluginv1.ItemType_ITEM_TYPE_MOVIE
	if !movie && req.GetType() != pluginv1.ItemType_ITEM_TYPE_SERIES {
		return nil, sdk.InvalidArgument("search supports movies and series")
	}
	limit := int(req.GetLimit())
	if limit <= 0 || limit > 25 {
		limit = defaultLimit
	}
	if c, err := p.searchByID(ctx, set, req, movie); err != nil || c != nil {
		if err != nil {
			return nil, err
		}
		return &pluginv1.SearchResponse{Candidates: []*pluginv1.SearchCandidate{c}}, nil
	}
	queries := queryTitles(req)
	pool := map[int]scored{}
	for i, q := range queries {
		var data struct {
			Data []*bgmSubject `json:"data"`
		}
		// Bangumi cannot filter by format, so ask for a few more than needed and keep the right kind below.
		body := map[string]any{"keyword": q, "sort": "match", "filter": map[string]any{"type": []int{subjectTypeAnime}}}
		err := p.api.do(ctx, "POST", set.baseURL+"/v0/search/subjects?"+url.Values{"limit": {strconv.Itoa(min(limit*2, 25))}}.Encode(), body, &data)
		if err != nil {
			if i > 0 && !errors.Is(err, context.Canceled) {
				break // keep what the earlier queries found
			}
			return nil, mapErr(err, "search")
		}
		for _, s := range data.Data {
			if s.isMovie() != movie {
				continue
			}
			p.cache.putSubject(s)
			pool[s.ID] = scoreSubject(s, queries, int(req.GetYear()), movie)
		}
		if bestScore(pool) >= goodMatch {
			break
		}
	}
	capShadowed(pool)
	demoteContinuations(pool)
	if !movie {
		demoteLaterSeasons(pool)
	}
	liftUniqueYearPrefix(pool, int(req.GetYear()))
	list := make([]scored, 0, len(pool))
	for _, s := range pool {
		list = append(list, s)
	}
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].score != list[j].score {
			return list[i].score > list[j].score
		}
		return list[i].s.Collection.Collect > list[j].s.Collection.Collect
	})
	if len(list) > limit {
		list = list[:limit]
	}
	resp := &pluginv1.SearchResponse{}
	for _, s := range list {
		resp.Candidates = append(resp.Candidates, candidate(s, req.GetType(), set, req.GetLanguages()))
	}
	return resp, nil
}

// shadowedCap is the highest score a candidate can have while another candidate matches the query exactly. The host applies a result on its
// own only when the best score leads the next one by 0.1; a query that extends a shorter title ("Title Brotherhood" against "Title") would
// otherwise score 0.95 and make the exact match look ambiguous.
const shadowedCap = 0.85

// capShadowed lowers the candidates whose only claim is that the query extends their title, when another candidate matches exactly.
func capShadowed(pool map[int]scored) {
	exact := false
	for _, c := range pool {
		exact = exact || isExactReason(c.reasons[0]) && c.score >= 1
	}
	if !exact {
		return
	}
	for id, c := range pool {
		if c.reasons[0] == "title.prefix" && c.score > shadowedCap {
			c.score = shadowedCap
			pool[id] = c
		}
	}
}

// continuationCap is the most a later part of a season (part 2, the second cour) may score when its name is not what was searched for: the
// plugin folds it into the season of the part before it, so the earlier part is the subject to match.
const continuationCap = 0.8

// demoteLaterSeasons holds a series subject named as a second or later season ("Title season 2", "Title part 2" in Chinese, "Final Season")
// at continuationCap when its name is not what was searched for: a library asks for the series by its plain name, and the series is its
// first season, which the plugin walks from; a later season would make the seasons after it unreachable and the numbering wrong.
func demoteLaterSeasons(pool map[int]scored) {
	for id, c := range pool {
		if m := seasonMark(c.s); m != "" && m != "s1" && c.reasons[0] != "title.exact" && c.score > continuationCap {
			c.score = continuationCap
			c.reasons = append(c.reasons, "later_season")
			pool[id] = c
		}
	}
}

func demoteContinuations(pool map[int]scored) {
	for id, c := range pool {
		// Only a name that is identical as written protects a later part; "Title (2022)" among its aliases does not (the first part has the same year).
		if isContinuation(c.s) && c.reasons[0] != "title.exact" && c.score > continuationCap {
			c.score = continuationCap
			c.reasons = append(c.reasons, "continuation")
			pool[id] = c
		}
	}
}

// liftedPrefix is the score of a prefix match that the year singles out (see liftUniqueYearPrefix): enough for the host to apply it, with a
// clear 0.1 lead over a demoted later part at continuationCap.
const liftedPrefix = 0.91

// liftUniqueYearPrefix: when nothing matched well and exactly one candidate extends the searched name and aired in the requested year
// ("Demon Slayer" against "Demon Slayer: Kimetsu no Yaiba", 2019), the year singles it out, so it is lifted to liftedPrefix. Two such
// candidates (parts I and II of a film series in one year) stay as they are: the host then asks the user.
func liftUniqueYearPrefix(pool map[int]scored, year int) {
	if year == 0 {
		return
	}
	only, n := 0, 0
	for id, c := range pool {
		if c.score >= liftedPrefix {
			return
		}
		if c.reasons[0] == "title.prefix" && c.score > 0.849 && c.score < 0.851 && len(c.reasons) == 2 && c.reasons[1] == "year.exact" {
			only, n = id, n+1
		}
	}
	if n != 1 {
		return
	}
	c := pool[only]
	c.score = liftedPrefix
	c.reasons = append(c.reasons, "title.prefix_year")
	pool[only] = c
}

func bestScore(pool map[int]scored) float64 {
	var best float64
	for _, s := range pool {
		best = max(best, s.score)
	}
	return best
}

// searchByID looks the subject up directly when the request carries a bangumi ID. An ID match is certain, so it scores 1.
func (p *plugin) searchByID(ctx context.Context, set settings, req *pluginv1.SearchRequest, movie bool) (*pluginv1.SearchCandidate, error) {
	id, err := strconv.Atoi(req.GetProviderIds()[pluginID])
	if err != nil || id <= 0 {
		return nil, nil
	}
	s, err := p.subject(ctx, set, id)
	if errors.Is(err, errNotFound) {
		return nil, nil // fall back to the name search
	}
	if err != nil {
		return nil, mapErr(err, "subject")
	}
	if s.Type != subjectTypeAnime || s.isMovie() != movie {
		return nil, nil
	}
	c := candidate(scored{s: s, score: 1, matched: firstNonEmpty(s.NameCN, s.Name), reasons: []string{"id.exact"}}, req.GetType(), set, req.GetLanguages())
	c.Confirmed = true // the host applies a single confirmed candidate on its own (SDK v0.4.0; older hosts ignore the field)
	return c, nil
}

func candidate(sc scored, t pluginv1.ItemType, set settings, l *pluginv1.Languages) *pluginv1.SearchCandidate {
	title, _ := pickTitle(sc.s, set.titleLang, l)
	return &pluginv1.SearchCandidate{
		Id: strconv.Itoa(sc.s.ID), Type: t, Name: title, OriginalName: strings.TrimSpace(sc.s.Name), Year: int32(sc.s.year()),
		Overview: truncate(cleanText(sc.s.Summary), 400), ImageUrl: firstNonEmpty(sc.s.Images.Common, sc.s.Images.Large, sc.s.Images.Medium),
		ProviderIds: providerIDs(sc.s), Score: sc.score, MatchedTitle: sc.matched, Reasons: sc.reasons,
	}
}
