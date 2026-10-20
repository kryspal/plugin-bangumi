package main

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	pluginv1 "github.com/kryspal/plugin-sdk/pluginv1"
	"github.com/kryspal/plugin-sdk/sdk"
)

const (
	episodePageSize = 200 // Bangumi's maximum
	maxEpisodePages = 15  // 3000 episodes: enough for the longest-running series
	episodeMain     = 0
	episodeSpecial  = 1
	// sequelRelation is the relation name Bangumi uses between a subject and the next one in the story.
	sequelRelation = "续集"
	// maxSequelCandidates bounds the subjects fetched to pick one sequel.
	maxSequelCandidates = 3
)

// mapErr converts a client error into a protocol error.
func mapErr(err error, what string) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, errNotFound):
		return sdk.NotFound(what + " not found on Bangumi")
	}
	if _, ok := errors.AsType[*sdk.RateLimitError](err); ok {
		return err
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return err
	}
	return sdk.Unavailable(err.Error())
}

// subject reads one subject by ID, using the cache. A search result in the cache is complete enough for scoring but not for a series.
func (p *plugin) subject(ctx context.Context, set settings, id int) (*bgmSubject, error) {
	if s, ok := p.cache.subject(id); ok && s.full {
		return s, nil
	}
	var s bgmSubject
	if err := p.api.get(ctx, set.baseURL, "/v0/subjects/"+strconv.Itoa(id), nil, &s); err != nil {
		return nil, err
	}
	s.full = true
	p.cache.putSubject(&s)
	return &s, nil
}

// episodes lists a subject's episodes of one type (main or special), in order, using the cache.
func (p *plugin) episodes(ctx context.Context, set settings, id, typ int) ([]bgmEpisode, error) {
	if l, ok := p.cache.episodeList(id, typ); ok {
		return l, nil
	}
	var all []bgmEpisode
	for page := 0; page < maxEpisodePages; page++ {
		var res bgmEpisodes
		q := url.Values{"subject_id": {strconv.Itoa(id)}, "type": {strconv.Itoa(typ)}, "limit": {strconv.Itoa(episodePageSize)}, "offset": {strconv.Itoa(page * episodePageSize)}}
		if err := p.api.get(ctx, set.baseURL, "/v0/episodes", q, &res); err != nil {
			return nil, err
		}
		all = append(all, res.Data...)
		if len(res.Data) < episodePageSize || len(all) >= res.Total {
			break
		}
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].Sort < all[j].Sort })
	p.cache.putEpisodes(id, typ, all)
	return all, nil
}

// GetItem implements pluginv1.MetadataProviderServer.
func (p *plugin) GetItem(ctx context.Context, req *pluginv1.GetItemRequest) (*pluginv1.ItemMetadata, error) {
	set := p.settings()
	switch req.GetType() {
	case pluginv1.ItemType_ITEM_TYPE_MOVIE, pluginv1.ItemType_ITEM_TYPE_SERIES:
		return p.getTitle(ctx, set, req)
	case pluginv1.ItemType_ITEM_TYPE_SEASON:
		if req.GetSeriesId() == "" || req.SeasonNumber == nil {
			return nil, sdk.InvalidArgument("season needs series_id and season_number")
		}
		n := int(req.GetSeasonNumber())
		if n == 0 {
			if _, err := p.rootSubject(ctx, set, req.GetSeriesId()); err != nil {
				return nil, err
			}
			zero := int32(0)
			return &pluginv1.ItemMetadata{Type: pluginv1.ItemType_ITEM_TYPE_SEASON, Name: messages.For(req.GetLanguages().GetLanguage()).T("specials"), IndexNumber: &zero}, nil
		}
		s, err := p.seasonSubject(ctx, set, req.GetSeriesId(), n)
		if err != nil {
			return nil, err
		}
		return seasonMetadata(s, n, set, req.GetLanguages()), nil
	case pluginv1.ItemType_ITEM_TYPE_EPISODE:
		return p.getEpisode(ctx, set, req)
	}
	return nil, sdk.InvalidArgument("unsupported item type")
}

func (p *plugin) getTitle(ctx context.Context, set settings, req *pluginv1.GetItemRequest) (*pluginv1.ItemMetadata, error) {
	id, err := strconv.Atoi(req.GetId())
	if err != nil || id <= 0 {
		return nil, sdk.InvalidArgument("id must be a Bangumi subject ID")
	}
	s, err := p.subject(ctx, set, id)
	if err != nil {
		return nil, mapErr(err, "title")
	}
	out := baseMetadata(s, req.GetType(), set, req.GetLanguages())
	var chars []bgmCharacter
	if set.maxCast > 0 {
		if err := p.api.get(ctx, set.baseURL, "/v0/subjects/"+strconv.Itoa(id)+"/characters", nil, &chars); err != nil && !errors.Is(err, errNotFound) {
			return nil, mapErr(err, "cast")
		}
	}
	var staff []bgmRelatedPerson
	if err := p.api.get(ctx, set.baseURL, "/v0/subjects/"+strconv.Itoa(id)+"/persons", nil, &staff); err != nil && !errors.Is(err, errNotFound) {
		return nil, mapErr(err, "staff")
	}
	out.Credits = credits(chars, staff, set.maxCast)
	return out, nil
}

func (p *plugin) rootSubject(ctx context.Context, set settings, seriesID string) (*bgmSubject, error) {
	id, err := strconv.Atoi(seriesID)
	if err != nil || id <= 0 {
		return nil, sdk.InvalidArgument("series_id must be a Bangumi subject ID")
	}
	s, err := p.subject(ctx, set, id)
	if err != nil {
		return nil, mapErr(err, "series")
	}
	return s, nil
}

func isSeasonPlatform(pl string) bool { return pl == "TV" || pl == "WEB" }

// nextSeason is the subject that continues cur: its sequel with a TV or web platform. Bangumi's relation list has no platform, so
// the first few sequel candidates are fetched. A film or OVA among the sequels is not a season, but the story may go on from it (a TV
// season whose sequel relation points at the film before it), so its own sequels are looked at as well, one level down. It returns nil when there is none.
func (p *plugin) nextSeason(ctx context.Context, set settings, cur int, seen map[int]bool) (*bgmSubject, error) {
	return p.nextSeasonFrom(ctx, set, cur, seen, 1)
}

// maxSequelDepth is how many non-season subjects (films, OVAs) the walk may pass through in a row to reach the next season.
const maxSequelDepth = 2

func (p *plugin) nextSeasonFrom(ctx context.Context, set settings, cur int, seen map[int]bool, depth int) (*bgmSubject, error) {
	var rels []bgmRelation
	if err := p.api.get(ctx, set.baseURL, "/v0/subjects/"+strconv.Itoa(cur)+"/subjects", nil, &rels); err != nil {
		if errors.Is(err, errNotFound) {
			return nil, nil
		}
		return nil, err
	}
	tried := 0
	var through []*bgmSubject
	for _, r := range rels {
		if r.Relation != sequelRelation || r.Type != subjectTypeAnime || seen[r.ID] {
			continue
		}
		if tried == maxSequelCandidates {
			break
		}
		tried++
		s, err := p.subject(ctx, set, r.ID)
		if err != nil {
			return nil, err
		}
		if isSeasonPlatform(s.Platform) {
			return s, nil
		}
		through = append(through, s)
	}
	if depth >= maxSequelDepth {
		return nil, nil
	}
	for _, s := range through {
		seen[s.ID] = true
		if next, err := p.nextSeasonFrom(ctx, set, s.ID, seen, depth+1); err != nil || next != nil {
			return next, err
		}
	}
	return nil, nil
}

// continuationRE matches the names Bangumi gives a later part of the same season (a split cour): "part 2" (also written "Part.2"), the second
// cour, the second half, the latter part. A new season is called "season N" (or has its own title), which does not match.
var continuationRE = regexp.MustCompile(`(?i)第\s*[2-9二三四五六七八九]\s*(?:部分|クール|cour)|\bpart\s*[.．]?\s*[2-9]\b|[后後]\s*(?:半|篇|編|编)`)

// isContinuation reports whether the subject continues the season of the previous one instead of starting a new season.
func isContinuation(s *bgmSubject) bool {
	return continuationRE.MatchString(s.NameCN) || continuationRE.MatchString(s.Name)
}

// seasonMarkRE finds the season wording in a name: "season N" in Chinese (the character for season, for a Japanese-style period, or for a
// part of a Chinese series), Season N, Nth season, and "final season" in Chinese or English.
var seasonMarkRE = regexp.MustCompile(`(?i)第\s*([0-9一二三四五六七八九十]+)\s*[季期部]|season\s*(\d+)|(\d+)(?:st|nd|rd|th)\s*season|(最终季|最終季|final\s*season)`)

// seasonMark is the season a name says it is: "s2" for the second season, "final" for a final season, "" when the name says nothing.
func seasonMark(s *bgmSubject) string {
	for _, name := range []string{s.NameCN, s.Name} {
		m := seasonMarkRE.FindStringSubmatch(name)
		if m == nil {
			continue
		}
		if m[4] != "" {
			return "final"
		}
		if n := cnNumber(firstNonEmpty(m[1], m[2], m[3])); n > 0 {
			return "s" + strconv.Itoa(n)
		}
	}
	return ""
}

// cnNumber converts an Arabic or a simple Chinese numeral (one to ninety-nine) to an integer; 0 when it is neither.
func cnNumber(s string) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	digits := map[rune]int{'一': 1, '二': 2, '三': 3, '四': 4, '五': 5, '六': 6, '七': 7, '八': 8, '九': 9}
	total, cur := 0, 0
	for _, r := range s {
		if r == '十' {
			if cur == 0 {
				cur = 1
			}
			total, cur = total+cur*10, 0
			continue
		}
		d, ok := digits[r]
		if !ok {
			return 0
		}
		cur = cur*10 + d
	}
	return total + cur
}

// sameSeason reports whether next is a later part of the season that first opens: its name says so (isContinuation), or both names carry the
// same season wording ("season 3, attack arc" and "season 3, counterattack arc" are one season 3). A name with other season wording, or a
// sequel with its own subtitle and no season wording, is a new season: a cour named only after its arc cannot be told from a sequel by its name.
func sameSeason(first, next *bgmSubject) bool {
	if isContinuation(next) {
		return true
	}
	nm := seasonMark(next)
	return nm != "" && nm == seasonMark(first)
}

// seasons lists the subject IDs of each season, in order: the root and every sequel in turn. A sequel that is a later part of the same
// season (sameSeason) joins the group of the season before it, so a series released in two cours is one season, as it is on TMDB. A sequel
// with a single episode is a special that Bangumi files as a TV subject, not a season: it is passed over. At most maxChainSeas subjects are followed.
func (p *plugin) seasons(ctx context.Context, set settings, root int) ([][]int, error) {
	if g, ok := p.cache.chain(root); ok {
		return g, nil
	}
	rootSubj, err := p.subject(ctx, set, root)
	if err != nil {
		return nil, mapErr(err, "series")
	}
	groups := [][]int{{root}}
	first := rootSubj // the subject that opens the current season
	seen := map[int]bool{root: true}
	cur, total := root, 1
	for total < maxChainSeas {
		next, err := p.nextSeason(ctx, set, cur, seen)
		if err != nil {
			if total == 1 || ctx.Err() != nil {
				return nil, mapErr(err, "series")
			}
			return groups, nil // keep the seasons found; they are not cached, so the next call walks again
		}
		if next == nil || seen[next.ID] {
			break
		}
		seen[next.ID] = true
		total++
		switch {
		case sameSeason(first, next):
			groups[len(groups)-1] = append(groups[len(groups)-1], next.ID)
		case next.episodeCount() == 1:
			// a one-episode sequel (a recap or a special aired on TV) is not a season
		default:
			groups = append(groups, []int{next.ID})
			first = next
		}
		cur = next.ID
	}
	p.cache.putChain(root, groups)
	return groups, nil
}

// seasonGroup returns the subject IDs of season n (n >= 1) of the series whose first subject is seriesID; the first one describes the season.
func (p *plugin) seasonGroup(ctx context.Context, set settings, seriesID string, n int) ([]int, error) {
	root, err := p.rootSubject(ctx, set, seriesID)
	if err != nil {
		return nil, err
	}
	if n == 1 {
		// Season 1 may have later parts, but they are only worth finding when episodes are asked for; the caller that needs them asks for the chain.
		return []int{root.ID}, nil
	}
	groups, err := p.seasons(ctx, set, root.ID)
	if err != nil {
		return nil, err
	}
	if n > len(groups) {
		return nil, sdk.NotFound("season " + strconv.Itoa(n) + " not found on Bangumi")
	}
	return groups[n-1], nil
}

// seasonSubject returns the subject that describes season n (n >= 1).
func (p *plugin) seasonSubject(ctx context.Context, set settings, seriesID string, n int) (*bgmSubject, error) {
	ids, err := p.seasonGroup(ctx, set, seriesID, n)
	if err != nil {
		return nil, err
	}
	if n == 1 {
		return p.rootSubject(ctx, set, seriesID)
	}
	s, err := p.subject(ctx, set, ids[0])
	if err != nil {
		return nil, mapErr(err, "season")
	}
	return s, nil
}

func seasonMetadata(s *bgmSubject, n int, set settings, l *pluginv1.Languages) *pluginv1.ItemMetadata {
	out := baseMetadata(s, pluginv1.ItemType_ITEM_TYPE_SEASON, set, l)
	idx := int32(n)
	out.IndexNumber = &idx
	out.RuntimeSeconds = 0
	return out
}

func (p *plugin) getEpisode(ctx context.Context, set settings, req *pluginv1.GetItemRequest) (*pluginv1.ItemMetadata, error) {
	if req.GetSeriesId() == "" {
		return nil, sdk.InvalidArgument("episode needs series_id")
	}
	season, ep := 0, 0
	switch {
	case req.AbsoluteEpisodeNumber != nil && req.SeasonNumber == nil && req.EpisodeNumber == nil:
		if req.GetAbsoluteEpisodeNumber() < 1 {
			return nil, sdk.InvalidArgument("an absolute episode number is at least 1")
		}
		s, e, err := p.resolveAbsolute(ctx, set, req.GetSeriesId(), int(req.GetAbsoluteEpisodeNumber()))
		if err != nil {
			return nil, err
		}
		season, ep = s, e
	case req.SeasonNumber != nil && req.EpisodeNumber != nil:
		season, ep = int(req.GetSeasonNumber()), int(req.GetEpisodeNumber())
	default:
		return nil, sdk.InvalidArgument("episode needs season_number and episode_number")
	}
	if ep < 1 {
		return nil, sdk.NotFound("episode " + strconv.Itoa(ep) + " not found on Bangumi")
	}
	var list []numberedEpisode
	if season == 0 {
		root, err := p.rootSubject(ctx, set, req.GetSeriesId())
		if err != nil {
			return nil, err
		}
		raw, err := p.episodes(ctx, set, root.ID, episodeSpecial)
		if err != nil {
			return nil, mapErr(err, "episodes")
		}
		for i, e := range raw { // specials are numbered by position, because their own numbers are not reliable
			list = append(list, numberedEpisode{e, i + 1})
		}
	} else {
		ids, err := p.episodeGroup(ctx, set, req.GetSeriesId(), season)
		if err != nil {
			return nil, err
		}
		if list, err = p.seasonEpisodes(ctx, set, ids); err != nil {
			return nil, mapErr(err, "episodes")
		}
	}
	for _, e := range list {
		if e.num == ep {
			return episodeMetadata(e.bgmEpisode, season, ep, set, req.GetLanguages()), nil
		}
	}
	return nil, sdk.NotFound("episode " + strconv.Itoa(ep) + " not found on Bangumi")
}

// episodeGroup is seasonGroup, but season 1 also walks the chain, because its later parts hold some of the episodes.
func (p *plugin) episodeGroup(ctx context.Context, set settings, seriesID string, n int) ([]int, error) {
	root, err := p.rootSubject(ctx, set, seriesID)
	if err != nil {
		return nil, err
	}
	groups, err := p.seasons(ctx, set, root.ID)
	if err != nil {
		return nil, err
	}
	if n > len(groups) {
		return nil, sdk.NotFound("season " + strconv.Itoa(n) + " not found on Bangumi")
	}
	return groups[n-1], nil
}

// numberedEpisode is an episode with the number it has in the season.
type numberedEpisode struct {
	bgmEpisode
	num int
}

// seasonEpisodes lists the main episodes of a season, numbered as the season is. The first subject numbers by ep (the number inside the
// subject), or by sort when ep is 0. A later part of the season numbers its episodes by position after the episodes before it: Bangumi's own
// numbers of a later cour vary (restarting at 1, continuing the season, or continuing the whole series), and the position is right in each case.
func (p *plugin) seasonEpisodes(ctx context.Context, set settings, ids []int) ([]numberedEpisode, error) {
	var out []numberedEpisode
	offset := 0
	for i, id := range ids {
		list, err := p.episodes(ctx, set, id, episodeMain)
		if err != nil {
			return nil, err
		}
		for j, e := range list {
			num := int(e.Ep)
			if num == 0 {
				num = int(e.Sort)
			}
			if i > 0 {
				num = offset + j + 1
			}
			out = append(out, numberedEpisode{e, num})
		}
		offset += len(list)
		if s, ok := p.cache.subject(id); ok && s.episodeCount() > len(list) {
			offset += s.episodeCount() - len(list)
		}
	}
	return out, nil
}

// resolveAbsolute maps an absolute episode number to (season, episode) by adding up the episode counts of the seasons in order (a season
// is all the parts of it). A season with an unknown count (a part still airing) takes every remaining number.
func (p *plugin) resolveAbsolute(ctx context.Context, set settings, seriesID string, abs int) (int, int, error) {
	root, err := p.rootSubject(ctx, set, seriesID)
	if err != nil {
		return 0, 0, err
	}
	groups, err := p.seasons(ctx, set, root.ID)
	if err != nil {
		return 0, 0, err
	}
	left := abs
	for i, ids := range groups {
		total, unknown := 0, false
		for _, id := range ids {
			s, err := p.subject(ctx, set, id)
			if err != nil {
				return 0, 0, mapErr(err, "season")
			}
			if n := s.episodeCount(); n == 0 {
				unknown = true
			} else {
				total += n
			}
		}
		if unknown || left <= total {
			return i + 1, left, nil
		}
		left -= total
	}
	return 0, 0, sdk.NotFound("absolute episode " + strconv.Itoa(abs) + " is beyond the series")
}

func episodeName(e bgmEpisode, l *pluginv1.Languages) string {
	if languageKind(l) == "zh" {
		return firstNonEmpty(e.NameCN, e.Name)
	}
	return firstNonEmpty(e.Name, e.NameCN)
}

func episodeMetadata(e bgmEpisode, season, n int, set settings, l *pluginv1.Languages) *pluginv1.ItemMetadata {
	idx, parent := int32(n), int32(season)
	name := episodeName(e, l)
	switch set.titleLang {
	case titleChinese:
		name = firstNonEmpty(e.NameCN, e.Name)
	case titleNative:
		name = firstNonEmpty(e.Name, e.NameCN)
	}
	return &pluginv1.ItemMetadata{
		Id: strconv.Itoa(e.ID), Type: pluginv1.ItemType_ITEM_TYPE_EPISODE, Name: strings.TrimSpace(name), Overview: cleanText(e.Desc),
		PremiereDate: isoDate(e.Airdate), RuntimeSeconds: int64(e.DurationSeconds), IndexNumber: &idx, ParentIndexNumber: &parent,
	}
}
