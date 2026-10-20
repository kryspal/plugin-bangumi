package main

import (
	"context"
	"slices"
	"strconv"
	"testing"

	pluginv1 "github.com/kryspal/plugin-sdk/pluginv1"
)

// Cases found by searching the real Bangumi with the names a library uses (all names here are made up).

func TestFoldNormalizesWidthAndNumerals(t *testing.T) {
	for in, want := range map[string]string{"Ｓｈｏｗ　１２": "show12", "Show Ⅱ": "showii", "【Title】": "title", "Re:Zero": "rezero"} {
		if got := fold(in); got != want {
			t.Errorf("fold(%q) = %q, want %q", in, got, want)
		}
	}
}

func searchWith(t *testing.T, subjects []*bgmSubject, kind pluginv1.ItemType, name string, year int) []*pluginv1.SearchCandidate {
	t.Helper()
	f := fixture()
	f.searchFor = func(string) []*bgmSubject { return subjects }
	p := newTest(t, f)
	res, err := p.Search(context.Background(), &pluginv1.SearchRequest{Type: kind, Name: name, Year: int32(year), Languages: &pluginv1.Languages{Language: "zh-CN"}})
	if err != nil {
		t.Fatal(err)
	}
	return res.GetCandidates()
}

func TestSearchRemakeNamedByYear(t *testing.T) {
	// Bangumi names a remake "Title 2009" while the original is "Title": with the year asked for, the remake is the identical match.
	old := subj(1, "ハーバー", "星灯港", "TV", "2006-04-02", 14)
	remake := subj(2, "ハーバー", "星灯港 2009", "TV", "2009-04-03", 28)
	cs := searchWith(t, []*bgmSubject{old, remake}, pluginv1.ItemType_ITEM_TYPE_SERIES, "星灯港", 2009)
	if cs[0].GetId() != "2" || cs[0].GetScore() != 1 || !slices.Contains(cs[0].GetReasons(), "title.exact_year") || cs[0].GetScore()-cs[1].GetScore() < 0.1 {
		t.Errorf("remake year: %v", cs)
	}
	if cs := searchWith(t, []*bgmSubject{old, remake}, pluginv1.ItemType_ITEM_TYPE_SERIES, "星灯港", 2006); cs[0].GetId() != "1" || cs[0].GetScore() != 1 {
		t.Errorf("original year: %v", cs)
	}
}

func TestSearchMovieWithoutTheatricalWord(t *testing.T) {
	// Bangumi names a film "theatrical version: Title"; the library folder is "Title". A spin-off film named "Title side story" must not win.
	film := subj(1, "劇場版 ハーバー", "剧场版 星灯港", "剧场版", "2020-09-18", 1)
	side := subj(2, "ハーバー 外伝", "星灯港 外传", "剧场版", "2019-09-06", 1)
	cs := searchWith(t, []*bgmSubject{side, film}, pluginv1.ItemType_ITEM_TYPE_MOVIE, "星灯港", 2020)
	if cs[0].GetId() != "1" || cs[0].GetScore() != 1 || cs[0].GetScore()-cs[1].GetScore() < 0.1 {
		t.Errorf("film: %v", cs)
	}
	// The query may carry the word too.
	if cs := searchWith(t, []*bgmSubject{side, film}, pluginv1.ItemType_ITEM_TYPE_MOVIE, "剧场版 星灯港", 2020); cs[0].GetId() != "1" || cs[0].GetScore() != 1 {
		t.Errorf("film with the word in the query: %v", cs)
	}
	// Series are compared as written: the word is part of a series name only when it really is.
	if cs := searchWith(t, []*bgmSubject{subj(3, "劇場版 ハーバー", "剧场版 星灯港", "TV", "2020-01-01", 12)}, pluginv1.ItemType_ITEM_TYPE_SERIES, "星灯港", 2020); cs[0].GetScore() >= 0.9 {
		t.Errorf("series keeps the word: %v", cs)
	}
}

func TestSearchDemotesLaterParts(t *testing.T) {
	// The first cour and the second cour both extend the searched name and aired in the same year: the second cour is a later part of the
	// season, so the first cour is the subject to match, and the year singles it out.
	first := subj(1, "ハーバー ～港の灯り～", "星灯港～港口的灯火～", "TV", "2021-01-10", 11)
	second := subj(2, "ハーバー ～港の灯り～ 第2クール", "星灯港～港口的灯火～ 第2部分", "TV", "2021-10-03", 12)
	cs := searchWith(t, []*bgmSubject{second, first}, pluginv1.ItemType_ITEM_TYPE_SERIES, "星灯港", 2021)
	if cs[0].GetId() != "1" || cs[0].GetScore() != liftedPrefix || !slices.Contains(cs[0].GetReasons(), "title.prefix_year") {
		t.Errorf("first cour: %v", cs)
	}
	if cs[1].GetId() != "2" || cs[1].GetScore() != continuationCap || !slices.Contains(cs[1].GetReasons(), "continuation") || cs[0].GetScore()-cs[1].GetScore() < 0.1-1e-9 {
		t.Errorf("second cour: %v", cs)
	}
	// Searched by its own name, the second cour is the identical match and is not demoted.
	if cs := searchWith(t, []*bgmSubject{second, first}, pluginv1.ItemType_ITEM_TYPE_SERIES, "星灯港～港口的灯火～ 第2部分", 2021); cs[0].GetId() != "2" || cs[0].GetScore() != 1 {
		t.Errorf("second cour by name: %v", cs)
	}
}

func TestSearchLiftsTheOnlyPrefixMatchOfTheYear(t *testing.T) {
	long := subj(1, "ハーバー", "星灯港", "TV", "2019-04-06", 26)
	long.Infobox = []bgmInfo{{Key: "别名", Value: []any{map[string]any{"v": "Harbor Lights: Lantern Keepers"}}}}
	arc := subj(2, "ハーバー 遊郭編", "星灯港 游郭篇", "TV", "2021-12-05", 11)
	arc.Infobox = []bgmInfo{{Key: "别名", Value: []any{map[string]any{"v": "Harbor Lights: Lantern Keepers Entertainment District Arc"}}}}
	cs := searchWith(t, []*bgmSubject{arc, long}, pluginv1.ItemType_ITEM_TYPE_SERIES, "Harbor Lights", 2019)
	if cs[0].GetId() != "1" || cs[0].GetScore() != liftedPrefix || !slices.Contains(cs[0].GetReasons(), "title.prefix_year") || cs[0].GetScore()-cs[1].GetScore() < 0.1 {
		t.Errorf("lifted: %v", cs)
	}
	// Two films of one year that both extend the name stay at 0.85: the host asks the user.
	one := subj(3, "ハーバー 黄金時代篇I", "星灯港 黄金时代篇I", "剧场版", "2012-02-04", 1)
	two := subj(4, "ハーバー 黄金時代篇II", "星灯港 黄金时代篇II", "剧场版", "2012-06-23", 1)
	cs = searchWith(t, []*bgmSubject{one, two}, pluginv1.ItemType_ITEM_TYPE_MOVIE, "星灯港 黄金时代篇", 2012)
	for _, c := range cs {
		if c.GetScore() != 0.85 || slices.Contains(c.GetReasons(), "title.prefix_year") {
			t.Errorf("two of a year: %v", cs)
		}
	}
	// Without a year nothing is lifted.
	if cs := searchWith(t, []*bgmSubject{arc, long}, pluginv1.ItemType_ITEM_TYPE_SERIES, "Harbor Lights", 0); cs[0].GetScore() != 0.85 {
		t.Errorf("no year: %v", cs)
	}
}

func TestSeasonMarks(t *testing.T) {
	for name, want := range map[string]string{"星灯港 第三季 袭击篇": "s3", "ハーバー 2nd season": "s2", "Harbor Season 4": "s4", "星灯港 第2期": "s2", "星灯港 最终季 Part.2": "final",
		"Harbor The Final Season": "final", "星灯港 第十二季": "s12", "星灯港 游郭篇": "", "星灯港": ""} {
		if got := seasonMark(&bgmSubject{NameCN: name}); got != want {
			t.Errorf("seasonMark(%q) = %q, want %q", name, got, want)
		}
	}
}

// chainOf builds a fake with root 1 and the given sequels in a straight line, and returns the season groups.
func chainOf(t *testing.T, subjects ...*bgmSubject) [][]int {
	t.Helper()
	f := fixture()
	f.subjects = map[int]*bgmSubject{}
	f.relations = map[int][]bgmRelation{}
	for i, s := range subjects {
		f.subjects[s.ID] = s
		if i > 0 {
			prev := subjects[i-1]
			f.relations[prev.ID] = append(f.relations[prev.ID], bgmRelation{ID: s.ID, Type: 2, Relation: "续集"})
		}
	}
	p := newTest(t, f)
	groups, err := p.seasons(context.Background(), p.settings(), subjects[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	return groups
}

func TestSeasonsJoinPartsNamedAlike(t *testing.T) {
	// "Final season", "Final season Part.2" and the two one-episode "final chapter" films aired on TV are one season; "season 3" and
	// "season 3 Part.2" are one season; "season 3, attack arc" and "season 3, counterattack arc" are one season.
	groups := chainOf(t,
		subj(1, "ハーバー", "星灯港", "TV", "2013-04-06", 25),
		subj(2, "ハーバー Season 2", "星灯港 第二季", "TV", "2017-04-01", 12),
		subj(3, "ハーバー Season 3", "星灯港 第三季", "TV", "2018-07-22", 12),
		subj(4, "ハーバー Season 3 Part.2", "星灯港 第三季 Part.2", "TV", "2019-04-28", 10),
		subj(5, "ハーバー The Final Season", "星灯港 最终季", "TV", "2020-12-06", 16),
		subj(6, "ハーバー The Final Season Part.2", "星灯港 最终季 Part.2", "TV", "2022-01-09", 12),
		subj(7, "ハーバー The Final Season 完結編 前編", "星灯港 最终季 完结篇 前篇", "TV", "2023-03-03", 1),
		subj(8, "ハーバー The Final Season 完結編 後編", "星灯港 最终季 完结篇 后篇", "TV", "2023-11-04", 1),
	)
	want := [][]int{{1}, {2}, {3, 4}, {5, 6, 7, 8}}
	if !equalGroups(groups, want) {
		t.Errorf("groups = %v, want %v", groups, want)
	}
	groups = chainOf(t,
		subj(1, "ハーバー", "星灯港", "TV", "2016-04-03", 25),
		subj(2, "ハーバー 2nd season", "星灯港 第二季", "TV", "2020-07-08", 13),
		subj(3, "ハーバー 2nd season 後半クール", "星灯港 第二季 后半部分", "TV", "2021-01-06", 12),
		subj(4, "ハーバー 3rd season 襲撃編", "星灯港 第三季 袭击篇", "TV", "2024-10-02", 8),
		subj(5, "ハーバー 3rd season 反撃編", "星灯港 第三季 反击篇", "TV", "2025-02-05", 8),
	)
	if want := [][]int{{1}, {2, 3}, {4, 5}}; !equalGroups(groups, want) {
		t.Errorf("groups = %v, want %v", groups, want)
	}
}

func TestSeasonsArcNamedCoursAndSpecials(t *testing.T) {
	// An arc-named cour that starts within the window of the season's first part extends it; an arc that starts years later is a new season;
	// a one-episode TV sequel is a special and is passed over; "latter part" joins "former part".
	groups := chainOf(t,
		subj(1, "ハーバー", "星灯港", "TV", "2019-04-06", 26),
		subj(2, "ハーバー 無限列車編", "星灯港 无限列车篇", "TV", "2021-10-10", 7),
		subj(3, "ハーバー 遊郭編", "星灯港 游郭篇", "TV", "2021-12-05", 11),
		subj(4, "ハーバー スペシャル", "星灯港 特别放送", "TV", "2022-05-01", 1),
		subj(5, "ハーバー 死滅回游 前編", "星灯港 死灭回游 前篇", "TV", "2026-01-08", 12),
		subj(6, "ハーバー 死滅回游 後編", "星灯港 死灭回游 后篇", "TV", "2026-07-02", 0),
	)
	if want := [][]int{{1}, {2}, {3}, {5, 6}}; !equalGroups(groups, want) {
		t.Errorf("groups = %v, want %v", groups, want)
	}
	// A sequel with its own subtitle and no season wording is a new season even when it follows within a year (a second series named
	// "Title: After Story" is season 2, as on TMDB).
	groups = chainOf(t,
		subj(1, "ハーバー", "星灯港", "TV", "2021-04-10", 11),
		subj(2, "ハーバー 第2クール", "星灯港 第2部分", "TV", "2021-10-02", 12),
		subj(3, "ハーバー 〜AFTER STORY〜", "星灯港 港口的灯火", "TV", "2022-01-08", 12),
	)
	if want := [][]int{{1, 2}, {3}}; !equalGroups(groups, want) {
		t.Errorf("groups = %v, want %v", groups, want)
	}
}

func TestSearchYearAliasDoesNotProtectALaterPart(t *testing.T) {
	// Both cours carry the alias "Title (2022)" style names; only the first cour is the match.
	first := subj(1, "SPY×HARBOR", "间谍港", "TV", "2022-04-09", 12)
	first.Infobox = []bgmInfo{{Key: "别名", Value: []any{map[string]any{"v": "Spy x Harbor"}}}}
	second := subj(2, "SPY×HARBOR 第2クール", "间谍港 第2部分", "TV", "2022-10-01", 13)
	second.Infobox = []bgmInfo{{Key: "别名", Value: []any{map[string]any{"v": "Spy x Harbor (2022)"}}}}
	cs := searchWith(t, []*bgmSubject{second, first}, pluginv1.ItemType_ITEM_TYPE_SERIES, "Spy x Harbor", 2022)
	if cs[0].GetId() != "1" || cs[0].GetScore() != 1 || cs[1].GetScore() != continuationCap {
		t.Errorf("got %v", cs)
	}
}

func TestLaterPartEpisodesNumberByPosition(t *testing.T) {
	// Bangumi sometimes numbers a later cour across the whole series (sort 51 to 58 for the second half of season 3): the season still runs 1 to 16.
	f := fixture()
	var wide []bgmEpisode
	for i := 1; i <= 6; i++ {
		wide = append(wide, bgmEpisode{ID: 3000 + i, Name: "w" + strconv.Itoa(i), Ep: float64(50 + i), Sort: float64(50 + i)})
	}
	f.episodes[[2]int{6, 0}] = wide
	p := newTest(t, f)
	n := func(v int32) *int32 { return &v }
	for e, want := range map[int32]string{13: "w1", 18: "w6"} {
		ep, err := p.GetItem(context.Background(), &pluginv1.GetItemRequest{Type: pluginv1.ItemType_ITEM_TYPE_EPISODE, SeriesId: "1", SeasonNumber: n(1), EpisodeNumber: n(e)})
		if err != nil || ep.GetName() != want {
			t.Errorf("S1E%d: %v %v, want %q", e, ep, err, want)
		}
	}
	if _, err := p.GetItem(context.Background(), &pluginv1.GetItemRequest{Type: pluginv1.ItemType_ITEM_TYPE_EPISODE, SeriesId: "1", SeasonNumber: n(1), EpisodeNumber: n(51)}); err == nil {
		t.Error("the series-wide number must not be an episode of the season")
	}
}

func TestSeasonsFollowThroughAFilm(t *testing.T) {
	// Season 2's only sequel is a film; the film's sequel is season 3. The film is not a season, but the walk goes on through it.
	f := fixture()
	s1 := subj(1, "ハーバー", "星灯港", "TV", "2012-07-07", 25)
	s2 := subj(2, "ハーバー II", "星灯港 第二季", "TV", "2014-07-05", 24)
	film := subj(3, "劇場版 ハーバー", "剧场版 星灯港", "剧场版", "2017-02-18", 1)
	s3 := subj(4, "ハーバー アリシゼーション", "星灯港 爱丽丝篇", "TV", "2018-10-06", 24)
	f.subjects = map[int]*bgmSubject{1: s1, 2: s2, 3: film, 4: s3}
	f.relations = map[int][]bgmRelation{1: {{ID: 2, Type: 2, Relation: "续集"}}, 2: {{ID: 3, Type: 2, Relation: "续集"}}, 3: {{ID: 4, Type: 2, Relation: "续集"}}, 4: {{ID: 3, Type: 2, Relation: "前传"}}}
	p := newTest(t, f)
	groups, err := p.seasons(context.Background(), p.settings(), 1)
	if err != nil || !equalGroups(groups, [][]int{{1}, {2}, {4}}) {
		t.Errorf("groups = %v, %v", groups, err)
	}
	// Two films in a row are too far: the chain ends (maxSequelDepth).
	f.relations[3] = []bgmRelation{{ID: 5, Type: 2, Relation: "续集"}}
	f.subjects[5] = subj(5, "劇場版 ハーバー 2", "剧场版 星灯港 2", "剧场版", "2018-01-01", 1)
	f.relations[5] = []bgmRelation{{ID: 4, Type: 2, Relation: "续集"}}
	p = newTest(t, f)
	if groups, err := p.seasons(context.Background(), p.settings(), 1); err != nil || !equalGroups(groups, [][]int{{1}, {2}}) {
		t.Errorf("two films: groups = %v, %v", groups, err)
	}
}

func TestEpisodePagingBeyondAThousand(t *testing.T) {
	f := fixture()
	long := subj(1, "コナン", "柯南", "TV", "1996-01-08", 1340)
	f.subjects = map[int]*bgmSubject{1: long}
	f.relations = map[int][]bgmRelation{}
	f.episodes[[2]int{1, 0}] = epList(1340, "1996-01-08")
	p := newTest(t, f)
	abs := int32(1340)
	ep, err := p.GetItem(context.Background(), &pluginv1.GetItemRequest{Type: pluginv1.ItemType_ITEM_TYPE_EPISODE, SeriesId: "1", AbsoluteEpisodeNumber: &abs})
	if err != nil || ep.GetName() != "ep1340" {
		t.Errorf("episode 1340: %v %v", ep, err)
	}
}

func equalGroups(a, b [][]int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !slices.Equal(a[i], b[i]) {
			return false
		}
	}
	return true
}

var _ = strconv.Itoa

func TestSearchCandidateWithoutADateCannotBeAppliedOnAPartialMatch(t *testing.T) {
	// The library says 2004 and names a film "Title: Subtitle"; the only candidate is a short "Title" with no date at all (an old foreign
	// short of the same name). A partial match that the year cannot confirm stays below the bar.
	short := subj(1, "Винни-Пух", "小熊港", "剧场版", "", 1)
	cs := searchWith(t, []*bgmSubject{short}, pluginv1.ItemType_ITEM_TYPE_MOVIE, "小熊港：春天的森林", 2004)
	if cs[0].GetScore() >= 0.9 || !slices.Contains(cs[0].GetReasons(), "year.missing") {
		t.Errorf("undated partial match: %v", cs)
	}
	// An identical name without a date is still applied, and without a year in the request nothing is deducted.
	if cs := searchWith(t, []*bgmSubject{short}, pluginv1.ItemType_ITEM_TYPE_MOVIE, "小熊港", 2004); cs[0].GetScore() != 0.9 {
		t.Errorf("undated exact match: %v", cs)
	}
	if cs := searchWith(t, []*bgmSubject{short}, pluginv1.ItemType_ITEM_TYPE_MOVIE, "小熊港：春天的森林", 0); cs[0].GetScore() != 0.95 {
		t.Errorf("no year asked: %v", cs)
	}
}

func TestSearchPrefersTheFirstSeasonAsTheSeries(t *testing.T) {
	// Bangumi files a show season by season ("Title season 1", "Title season 2"); two seasons aired in the library's year. The series is its
	// first season, so the later one is held back and the first is the one match of the year.
	s1 := subj(1, "Futurport (Season 1)", "星灯港 第1季", "TV", "1999-03-28", 13)
	s2 := subj(2, "Futurport (Season 2)", "星灯港 第2季", "TV", "1999-11-21", 19)
	cs := searchWith(t, []*bgmSubject{s2, s1}, pluginv1.ItemType_ITEM_TYPE_SERIES, "星灯港", 1999)
	if cs[0].GetId() != "1" || cs[0].GetScore() != liftedPrefix || cs[1].GetScore() != continuationCap || !slices.Contains(cs[1].GetReasons(), "later_season") {
		t.Errorf("seasons: %v", cs)
	}
	// Named by its own season wording, the later season is the identical match and is not held back; the Chinese "part two" counts as season wording too.
	if cs := searchWith(t, []*bgmSubject{s2, s1}, pluginv1.ItemType_ITEM_TYPE_SERIES, "星灯港 第2季", 1999); cs[0].GetId() != "2" || cs[0].GetScore() != 1 {
		t.Errorf("by season name: %v", cs)
	}
	p2 := subj(3, "", "星灯港 第二部", "TV", "2000-01-01", 39)
	if cs := searchWith(t, []*bgmSubject{p2}, pluginv1.ItemType_ITEM_TYPE_SERIES, "星灯港", 2000); cs[0].GetScore() != continuationCap {
		t.Errorf("part two: %v", cs)
	}
	// Films are not held back by season wording.
	film := subj(4, "", "星灯港 第二部", "剧场版", "2000-01-01", 1)
	if cs := searchWith(t, []*bgmSubject{film}, pluginv1.ItemType_ITEM_TYPE_MOVIE, "星灯港", 2000); cs[0].GetScore() != liftedPrefix {
		t.Errorf("film: %v", cs)
	}
}
