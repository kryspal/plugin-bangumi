package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode"

	pluginv1 "github.com/kryspal/plugin-sdk/pluginv1"
	"github.com/kryspal/plugin-sdk/sdk"
)

// fake is a fake Bangumi API server.
type fake struct {
	subjects  map[int]*bgmSubject
	relations map[int][]bgmRelation
	persons   map[int][]bgmRelatedPerson
	chars     map[int][]bgmCharacter
	episodes  map[[2]int][]bgmEpisode
	people    map[int]map[string]any
	searchFor func(keyword string) []*bgmSubject
	calls     atomic.Int32
	badUA     atomic.Int32
	status    int
	headers   map[string]string
	lastQuery string
	lastBody  map[string]any
}

func (f *fake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.calls.Add(1)
	if ua := r.Header.Get("User-Agent"); !strings.HasPrefix(ua, "kryspal/plugin-bangumi/") || !strings.Contains(ua, "https://github.com/") {
		f.badUA.Add(1)
	}
	for k, v := range f.headers {
		w.Header().Set(k, v)
	}
	if f.status != 0 {
		w.WriteHeader(f.status)
		return
	}
	notFound := func() { w.WriteHeader(http.StatusNotFound); _, _ = w.Write([]byte(`{"title":"Not Found"}`)) }
	reply := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/") // v0, kind, id, sub
	f.lastQuery = r.URL.RawQuery
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/v0/search/subjects":
		_ = json.NewDecoder(r.Body).Decode(&f.lastBody)
		kw, _ := f.lastBody["keyword"].(string)
		reply(map[string]any{"data": f.searchFor(kw), "total": 0})
	case len(parts) >= 3 && parts[1] == "subjects":
		id, _ := strconv.Atoi(parts[2])
		s := f.subjects[id]
		if s == nil {
			notFound()
			return
		}
		switch {
		case len(parts) == 3:
			reply(s)
		case parts[3] == "persons":
			reply(f.persons[id])
		case parts[3] == "characters":
			reply(f.chars[id])
		case parts[3] == "subjects":
			reply(f.relations[id])
		}
	case len(parts) == 2 && parts[1] == "episodes":
		id, _ := strconv.Atoi(r.URL.Query().Get("subject_id"))
		typ, _ := strconv.Atoi(r.URL.Query().Get("type"))
		off, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		lim, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		all := f.episodes[[2]int{id, typ}]
		end := min(off+lim, len(all))
		page := []bgmEpisode{}
		if off < len(all) {
			page = all[off:end]
		}
		reply(bgmEpisodes{Total: len(all), Data: page})
	case len(parts) == 3 && parts[1] == "persons":
		id, _ := strconv.Atoi(parts[2])
		if m := f.people[id]; m != nil {
			reply(m)
			return
		}
		notFound()
	default:
		notFound()
	}
}

func newTest(t *testing.T, f *fake) *plugin {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	p := newPlugin()
	p.api.interval = 0
	p.baseURL = srv.URL
	t.Cleanup(func() {
		if n := f.badUA.Load(); n != 0 {
			t.Errorf("%d requests had no proper User-Agent", n)
		}
	})
	return p
}

func subj(id int, name, cn, platform, date string, eps int) *bgmSubject {
	return &bgmSubject{ID: id, Type: 2, Name: name, NameCN: cn, Platform: platform, Date: date, Eps: eps, Rating: bgmRating{Score: 7.8, Total: 1000}}
}

func epList(n int, firstAir string) []bgmEpisode {
	var out []bgmEpisode
	for i := 1; i <= n; i++ {
		out = append(out, bgmEpisode{ID: 1000 + i, Name: "ep" + strconv.Itoa(i), NameCN: "第" + strconv.Itoa(i) + "话", Ep: float64(i), Sort: float64(i),
			Airdate: firstAir, DurationSeconds: 1440, Desc: "　　Story " + strconv.Itoa(i) + ".\r\n"})
	}
	return out
}

// cour2Episodes are the six episodes of the second cour: numbered 1 to 6 inside the subject but 13 to 18 across the series, as Bangumi does.
func cour2Episodes() []bgmEpisode {
	var out []bgmEpisode
	for i := 1; i <= 6; i++ {
		out = append(out, bgmEpisode{ID: 2000 + i, Name: "c2ep" + strconv.Itoa(i), NameCN: "二期第" + strconv.Itoa(i) + "话", Ep: float64(i), Sort: float64(12 + i), Airdate: "2020-07-05"})
	}
	return out
}

func fixture() *fake {
	s1 := subj(1, "アルファ", "阿尔法", "TV", "2020-04-05", 12)
	s1.Infobox = []bgmInfo{{Key: "别名", Value: []any{map[string]any{"v": "Alpha Quest"}, map[string]any{"v": "阿尔法传说"}}},
		{Key: "动画制作", Value: "Studio One"}, {Key: "播放结束", Value: "2020年6月21日"}, {Key: "单集长度", Value: "24分钟"}}
	s1.Summary = "　　A tale.\r\n　　More."
	s1.MetaTags = []string{"TV", "日本", "奇幻"}
	s1.Images = bgmImages{Large: "https://img.example/1.jpg"}
	s2 := subj(2, "アルファ 第2期", "阿尔法 第二季", "TV", "2021-04-04", 13)
	s3 := subj(3, "アルファ 第3期", "阿尔法 第三季", "WEB", "2022-04-03", 0)
	movie := subj(4, "アルファ劇場版", "阿尔法剧场版", "剧场版", "2023-01-01", 1)
	ova := subj(5, "アルファ OVA", "", "OVA", "2021-01-01", 2)
	cour2 := subj(6, "アルファ 第2クール", "阿尔法 第2部分", "TV", "2020-07-05", 6)
	return &fake{
		subjects: map[int]*bgmSubject{1: s1, 2: s2, 3: s3, 4: movie, 5: ova, 6: cour2},
		relations: map[int][]bgmRelation{
			1: {{ID: 90, Type: 1, Relation: "书籍"}, {ID: 4, Type: 2, Relation: "续集"}, {ID: 5, Type: 2, Relation: "番外篇"}, {ID: 6, Type: 2, Relation: "续集"}},
			6: {{ID: 1, Type: 2, Relation: "前传"}, {ID: 2, Type: 2, Relation: "续集"}},
			2: {{ID: 6, Type: 2, Relation: "前传"}, {ID: 3, Type: 2, Relation: "续集"}},
			3: {{ID: 2, Type: 2, Relation: "前传"}},
		},
		episodes: map[[2]int][]bgmEpisode{{1, 0}: epList(12, "2020-04-05"), {6, 0}: cour2Episodes(), {2, 0}: epList(13, "2021-04-04"), {3, 0}: epList(30, "2022-04-03"),
			{1, 1}: {{ID: 1, Name: "sp-a", NameCN: "特别篇A", Sort: 0, Type: 1}, {ID: 2, Name: "sp-b", NameCN: "特别篇B", Sort: 1, Type: 1}}},
		persons: map[int][]bgmRelatedPerson{}, chars: map[int][]bgmCharacter{}, people: map[int]map[string]any{},
		searchFor: func(string) []*bgmSubject { return nil },
	}
}

func TestDescribe(t *testing.T) {
	p := newPlugin()
	for _, lang := range []string{"en", "zh-TW", "zh-CN", "fr"} {
		m, err := p.Describe(context.Background(), &pluginv1.DescribeRequest{Language: lang})
		if err != nil {
			t.Fatal(err)
		}
		if m.GetId() != "bangumi" || !strings.Contains(m.GetName(), "unofficial") || len(m.GetFeatures()) != 0 || m.GetAttribution().GetLink() == "" {
			t.Errorf("%s: manifest %q %v", lang, m.GetName(), m.GetFeatures())
		}
		if !strings.Contains(m.GetAttribution().GetText(), "CC BY-SA") || !strings.Contains(m.GetDescription(), "CC BY-SA") || m.GetAttribution().GetLogoUrl() != "" {
			t.Errorf("%s: the CC BY-SA source line must be in the attribution and the description, and there is no logo: %q", lang, m.GetAttribution().GetText())
		}
		for _, f := range m.GetConfigFields() {
			if f.GetRequired() {
				t.Errorf("%s: field %s is required, but Bangumi needs no key", lang, f.GetKey())
			}
		}
	}
	m, _ := p.Describe(context.Background(), &pluginv1.DescribeRequest{Language: "en"})
	for _, r := range m.GetDescription() + m.GetConfigFields()[0].GetLabel() + m.GetConfigFields()[0].GetDescription() {
		if unicode.Is(unicode.Han, r) {
			t.Fatalf("English text has Han characters: %q", m.GetDescription())
		}
	}
}

func TestMessagesComplete(t *testing.T) {
	for lang, tbl := range messageTables {
		for k := range messageTables["en"] {
			if tbl[k] == "" {
				t.Errorf("%s misses %s", lang, k)
			}
		}
	}
}

func TestConfigure(t *testing.T) {
	p := newPlugin()
	resp, _ := p.Configure(context.Background(), &pluginv1.ConfigureRequest{Values: map[string]string{keyBaseURL: "http://x", keyMaxCast: "999", keyTitleLang: "klingon"}})
	for _, k := range []string{keyBaseURL, keyMaxCast, keyTitleLang} {
		if resp.GetFieldErrors()[k] == "" {
			t.Errorf("no error for %s", k)
		}
	}
	resp, _ = p.Configure(context.Background(), &pluginv1.ConfigureRequest{Values: map[string]string{keyTitleLang: "native", keyMaxCast: "5"}})
	if len(resp.GetFieldErrors()) != 0 {
		t.Fatalf("errors: %v", resp.GetFieldErrors())
	}
	if s := p.settings(); s.titleLang != "native" || s.maxCast != 5 || s.baseURL != defaultBaseURL {
		t.Errorf("settings %+v", s)
	}
}

func TestPickTitle(t *testing.T) {
	s := &bgmSubject{Name: "進撃", NameCN: "进击", Infobox: []bgmInfo{{Key: "别名", Value: []any{map[string]any{"v": "进击别名"}, map[string]any{"v": "Attack"}}}}}
	cases := []struct{ pref, lang, want string }{{titleAuto, "zh-CN", "进击"}, {titleAuto, "ja", "進撃"}, {titleAuto, "en", "Attack"}, {titleAuto, "", "Attack"},
		{titleChinese, "en", "进击"}, {titleNative, "zh", "進撃"}}
	for _, c := range cases {
		if got, _ := pickTitle(s, c.pref, &pluginv1.Languages{Language: c.lang}); got != c.want {
			t.Errorf("%s/%s = %q, want %q", c.pref, c.lang, got, c.want)
		}
	}
	bare := &bgmSubject{Name: "進撃"}
	if got, _ := pickTitle(bare, titleAuto, &pluginv1.Languages{Language: "zh"}); got != "進撃" {
		t.Errorf("no Chinese name: %q", got)
	}
	if got, _ := pickTitle(bare, titleAuto, &pluginv1.Languages{Language: "en"}); got != "進撃" {
		t.Errorf("no English alias: %q", got)
	}
}

func TestTextHelpers(t *testing.T) {
	if got := cleanText("　　First.\r\n　　Second.\r\n\r\n\r\n\r\n　　Third."); got != "First.\nSecond.\n\nThird." {
		t.Errorf("cleanText %q", got)
	}
	if cnDate("2016年9月26日") != "2016-09-26" || cnDate("2016年9月") != "" || isoDate("2016-09-26") != "2016-09-26" || isoDate("2016-09") != "" || isoDate("0000-00-00") != "" {
		t.Error("date parsing")
	}
	if minutes("24分钟") != 24 || minutes("107 min") != 107 || minutes("") != 0 || atoiFirst("182cm") != 182 {
		t.Error("numbers")
	}
}

func TestSearchFiltersKindAndScores(t *testing.T) {
	f := fixture()
	f.searchFor = func(string) []*bgmSubject {
		return []*bgmSubject{f.subjects[2], f.subjects[4], f.subjects[1], subj(7, "Other", "无关", "TV", "2020-01-01", 12)}
	}
	p := newTest(t, f)
	resp, err := p.Search(context.Background(), &pluginv1.SearchRequest{Type: pluginv1.ItemType_ITEM_TYPE_SERIES, Name: "阿尔法", Year: 2020, Languages: &pluginv1.Languages{Language: "zh-CN"}})
	if err != nil {
		t.Fatal(err)
	}
	c := resp.GetCandidates()
	if len(c) != 3 || c[0].GetId() != "1" || c[0].GetScore() != 1 || c[0].GetName() != "阿尔法" || c[0].GetOriginalName() != "アルファ" || c[1].GetId() != "2" {
		t.Fatalf("candidates %v", c)
	}
	for _, x := range c {
		if x.GetId() == "4" {
			t.Error("a film must not be a series candidate")
		}
	}
	if c[2].GetScore() > 0.5 {
		t.Errorf("unrelated score %v", c[2].GetScore())
	}
	if n := f.calls.Load(); n != 1 {
		t.Errorf("a good match must stop after the first query, made %d", n)
	}
	if filter, _ := f.lastBody["filter"].(map[string]any); filter == nil || f.lastBody["sort"] != "match" {
		t.Errorf("body %v", f.lastBody)
	}
	mv, err := p.Search(context.Background(), &pluginv1.SearchRequest{Type: pluginv1.ItemType_ITEM_TYPE_MOVIE, Name: "阿尔法剧场版"})
	if err != nil || len(mv.GetCandidates()) != 1 || mv.GetCandidates()[0].GetId() != "4" {
		t.Errorf("movie search: %v %v", mv, err)
	}
}

// A query that extends a shorter title must not make the exact match look ambiguous: "Title Brotherhood" matches the second work exactly and
// only extends the first one's title, so the first must stay clearly behind.
func TestSearchExactMatchLeadsThePrefixMatch(t *testing.T) {
	f := fixture()
	first := subj(70, "ハガネ", "钢炼", "TV", "2003-10-04", 51)
	first.Infobox = []bgmInfo{{Key: "别名", Value: []any{map[string]any{"v": "Steel Alchemist"}}}}
	second := subj(71, "ハガネ二", "钢炼 二期", "TV", "2009-04-05", 64)
	second.Infobox = []bgmInfo{{Key: "别名", Value: []any{map[string]any{"v": "Steel Alchemist: Brotherhood"}}}}
	f.searchFor = func(string) []*bgmSubject { return []*bgmSubject{first, second} }
	p := newTest(t, f)
	resp, err := p.Search(context.Background(), &pluginv1.SearchRequest{Type: pluginv1.ItemType_ITEM_TYPE_SERIES, Name: "Steel Alchemist Brotherhood"})
	if err != nil || len(resp.GetCandidates()) != 2 {
		t.Fatalf("%v %v", resp, err)
	}
	c := resp.GetCandidates()
	if c[0].GetId() != "71" || c[0].GetScore() != 1 || c[1].GetScore() > 0.85 || c[0].GetScore()-c[1].GetScore() < 0.1 {
		t.Errorf("scores %v (%s) and %v (%s)", c[0].GetScore(), c[0].GetId(), c[1].GetScore(), c[1].GetId())
	}
	// Without an exact match the extended-title score stays: the directory name may just carry extra words.
	resp, _ = p.Search(context.Background(), &pluginv1.SearchRequest{Type: pluginv1.ItemType_ITEM_TYPE_SERIES, Name: "Steel Alchemist Extra Words"})
	if len(resp.GetCandidates()) == 0 || resp.GetCandidates()[0].GetScore() != 0.95 {
		t.Errorf("no exact match: %v", resp.GetCandidates())
	}
}

func TestSearchMatchesAliasesAndHints(t *testing.T) {
	f := fixture()
	f.searchFor = func(q string) []*bgmSubject {
		if q == "Alpha Quest" {
			return []*bgmSubject{f.subjects[1]}
		}
		return nil
	}
	p := newTest(t, f)
	resp, err := p.Search(context.Background(), &pluginv1.SearchRequest{Type: pluginv1.ItemType_ITEM_TYPE_SERIES, Name: "zzzz qqqq",
		Titles: []*pluginv1.TitleHint{{Text: "zzzz qqqq"}, {Text: "Alpha Quest"}, {Text: "variant", Role: pluginv1.TitleRole_TITLE_ROLE_VARIANT}}})
	if err != nil || len(resp.GetCandidates()) != 1 || resp.GetCandidates()[0].GetScore() != 1 || resp.GetCandidates()[0].GetMatchedTitle() != "Alpha Quest" {
		t.Fatalf("%v %v", resp, err)
	}
	if n := f.calls.Load(); n != 2 {
		t.Errorf("expected two queries, made %d", n)
	}
}

func TestSearchByID(t *testing.T) {
	f := fixture()
	p := newTest(t, f)
	resp, err := p.Search(context.Background(), &pluginv1.SearchRequest{Type: pluginv1.ItemType_ITEM_TYPE_SERIES, Name: "whatever", ProviderIds: map[string]string{"bangumi": "2"}})
	if err != nil || len(resp.GetCandidates()) != 1 || resp.GetCandidates()[0].GetId() != "2" || resp.GetCandidates()[0].GetScore() != 1 || !resp.GetCandidates()[0].GetConfirmed() {
		t.Errorf("%v %v", resp, err)
	}
	resp, err = p.Search(context.Background(), &pluginv1.SearchRequest{Type: pluginv1.ItemType_ITEM_TYPE_MOVIE, Name: "x", ProviderIds: map[string]string{"bangumi": "2"}})
	if err != nil || len(resp.GetCandidates()) != 0 {
		t.Errorf("series ID in a movie search: %v %v", resp, err)
	}
	resp, err = p.Search(context.Background(), &pluginv1.SearchRequest{Type: pluginv1.ItemType_ITEM_TYPE_SERIES, Name: "x", ProviderIds: map[string]string{"bangumi": "999"}})
	if err != nil || len(resp.GetCandidates()) != 0 {
		t.Errorf("unknown ID: %v %v", resp, err)
	}
}

func TestGetSeriesMetadata(t *testing.T) {
	f := fixture()
	f.chars[1] = []bgmCharacter{
		{ID: 11, Name: "Side", Relation: "配角", Actors: []struct {
			ID     int       `json:"id"`
			Name   string    `json:"name"`
			Images bgmImages `json:"images"`
		}{{ID: 71, Name: "Actor Two"}}},
		{ID: 10, Name: "Hero", Relation: "主角", Actors: []struct {
			ID     int       `json:"id"`
			Name   string    `json:"name"`
			Images bgmImages `json:"images"`
		}{{ID: 70, Name: "Actor One", Images: bgmImages{Large: "https://img.example/a.jpg"}}}},
		{ID: 12, Name: "Nobody", Relation: "配角"},
	}
	f.persons[1] = []bgmRelatedPerson{{ID: 80, Name: "Dir One", Relation: "导演"}, {ID: 81, Name: "Art", Relation: "美术监督"}, {ID: 82, Name: "Comp", Relation: "音乐"},
		{ID: 83, Name: "Author", Relation: "原作"}, {ID: 80, Name: "Dir One", Relation: "导演"}}
	p := newTest(t, f)
	got, err := p.GetItem(context.Background(), &pluginv1.GetItemRequest{Type: pluginv1.ItemType_ITEM_TYPE_SERIES, Id: "1", Languages: &pluginv1.Languages{Language: "zh-CN"}})
	if err != nil {
		t.Fatal(err)
	}
	if got.GetName() != "阿尔法" || got.GetOriginalTitle() != "アルファ" || got.GetOverview() != "A tale.\nMore." || got.GetPremiereDate() != "2020-04-05" || got.GetEndDate() != "2020-06-21" ||
		got.GetProductionYear() != 2020 || got.GetCommunityRating() != 7.8 || got.GetRuntimeSeconds() != 1440 || got.GetStatus() != pluginv1.SeriesStatus_SERIES_STATUS_ENDED ||
		got.GetLanguage() != "zh" {
		t.Errorf("fields: %v", got)
	}
	if got.GetProviderIds()["bangumi"] != "1" || len(got.GetStudios()) != 1 || got.GetStudios()[0] != "Studio One" || len(got.GetGenres()) != 1 || got.GetGenres()[0] != "奇幻" {
		t.Errorf("ids %v studios %v genres %v", got.GetProviderIds(), got.GetStudios(), got.GetGenres())
	}
	if len(got.GetRatings()) != 1 || got.GetRatings()[0].GetVotes() != 1000 || len(got.GetImages()) != 1 {
		t.Errorf("ratings %v images %v", got.GetRatings(), got.GetImages())
	}
	var order []string
	for _, c := range got.GetCredits() {
		order = append(order, c.GetName()+":"+c.GetKind().String()+":"+c.GetRole())
		if c.GetProviderIds()["bangumi"] != c.GetPersonId() {
			t.Errorf("credit %v: provider ID key must be the plugin ID", c)
		}
	}
	want := []string{"Actor One:PERSON_KIND_ACTOR:Hero", "Actor Two:PERSON_KIND_ACTOR:Side", "Dir One:PERSON_KIND_DIRECTOR:导演", "Comp:PERSON_KIND_COMPOSER:音乐", "Author:PERSON_KIND_WRITER:原作"}
	if strings.Join(order, "|") != strings.Join(want, "|") {
		t.Errorf("credits %v", order)
	}
	// With the cast off the characters are not requested, but the staff still are.
	p.maxCast = 0
	f.calls.Store(0)
	p.cache = cache{}
	got, err = p.GetItem(context.Background(), &pluginv1.GetItemRequest{Type: pluginv1.ItemType_ITEM_TYPE_SERIES, Id: "1"})
	if err != nil || f.calls.Load() != 2 {
		t.Fatalf("cast off: %v, %d requests", err, f.calls.Load())
	}
	for _, c := range got.GetCredits() {
		if c.GetKind() == pluginv1.PersonKind_PERSON_KIND_ACTOR {
			t.Errorf("cast off, but an actor was returned: %v", c)
		}
	}
}

func TestSeasonsFollowSequels(t *testing.T) {
	f := fixture()
	p := newTest(t, f)
	ctx := context.Background()
	n := func(v int32) *int32 { return &v }
	zh := &pluginv1.Languages{Language: "zh-CN"}
	s1, err := p.GetItem(ctx, &pluginv1.GetItemRequest{Type: pluginv1.ItemType_ITEM_TYPE_SEASON, SeriesId: "1", SeasonNumber: n(1), Languages: zh})
	if err != nil || s1.GetProviderIds()["bangumi"] != "1" || s1.GetIndexNumber() != 1 {
		t.Fatalf("season 1: %v %v", s1, err)
	}
	// Subject 4 (a film) comes first among the sequels but is skipped; 2 is the next TV season, then 3 (web).
	s2, err := p.GetItem(ctx, &pluginv1.GetItemRequest{Type: pluginv1.ItemType_ITEM_TYPE_SEASON, SeriesId: "1", SeasonNumber: n(2), Languages: zh})
	if err != nil || s2.GetProviderIds()["bangumi"] != "2" || s2.GetName() != "阿尔法 第二季" || s2.GetIndexNumber() != 2 {
		t.Fatalf("season 2: %v %v", s2, err)
	}
	s3, err := p.GetItem(ctx, &pluginv1.GetItemRequest{Type: pluginv1.ItemType_ITEM_TYPE_SEASON, SeriesId: "1", SeasonNumber: n(3)})
	if err != nil || s3.GetProviderIds()["bangumi"] != "3" {
		t.Fatalf("season 3: %v %v", s3, err)
	}
	if _, err := p.GetItem(ctx, &pluginv1.GetItemRequest{Type: pluginv1.ItemType_ITEM_TYPE_SEASON, SeriesId: "1", SeasonNumber: n(4)}); err == nil {
		t.Error("season 4 must be NOT_FOUND")
	}
	calls := f.calls.Load()
	if _, err := p.GetItem(ctx, &pluginv1.GetItemRequest{Type: pluginv1.ItemType_ITEM_TYPE_SEASON, SeriesId: "1", SeasonNumber: n(2)}); err != nil || f.calls.Load() != calls {
		t.Errorf("the chain should be cached: %v, %d more requests", err, f.calls.Load()-calls)
	}
	sp, err := p.GetItem(ctx, &pluginv1.GetItemRequest{Type: pluginv1.ItemType_ITEM_TYPE_SEASON, SeriesId: "1", SeasonNumber: n(0), Languages: zh})
	if err != nil || sp.GetName() != "特别篇" || sp.GetIndexNumber() != 0 {
		t.Errorf("season 0: %v %v", sp, err)
	}
	if _, err := p.GetItem(ctx, &pluginv1.GetItemRequest{Type: pluginv1.ItemType_ITEM_TYPE_SEASON, SeriesId: "999", SeasonNumber: n(1)}); err == nil {
		t.Error("unknown series must be an error")
	}
}

func TestEpisodes(t *testing.T) {
	f := fixture()
	p := newTest(t, f)
	ctx := context.Background()
	n := func(v int32) *int32 { return &v }
	get := func(s, e int32, lang string) (*pluginv1.ItemMetadata, error) {
		return p.GetItem(ctx, &pluginv1.GetItemRequest{Type: pluginv1.ItemType_ITEM_TYPE_EPISODE, SeriesId: "1", SeasonNumber: n(s), EpisodeNumber: n(e), Languages: &pluginv1.Languages{Language: lang}})
	}
	ep, err := get(1, 3, "zh-CN")
	if err != nil || ep.GetName() != "第3话" || ep.GetOverview() != "Story 3." || ep.GetPremiereDate() != "2020-04-05" || ep.GetRuntimeSeconds() != 1440 || ep.GetIndexNumber() != 3 || ep.GetParentIndexNumber() != 1 {
		t.Errorf("S1E3: %v %v", ep, err)
	}
	if ep, err := get(1, 3, "ja"); err != nil || ep.GetName() != "ep3" {
		t.Errorf("S1E3 ja: %v %v", ep, err)
	}
	if ep, err := get(2, 13, "zh"); err != nil || ep.GetParentIndexNumber() != 2 {
		t.Errorf("S2E13: %v %v", ep, err)
	}
	if _, err := get(1, 19, "zh"); err == nil {
		t.Error("episode 19 of an 18-episode season (12 + 6 in the second cour) must be NOT_FOUND")
	}
	if _, err := get(1, 0, "zh"); err == nil {
		t.Error("episode 0 must be NOT_FOUND")
	}
	sp, err := get(0, 2, "zh-CN")
	if err != nil || sp.GetName() != "特别篇B" || sp.GetParentIndexNumber() != 0 {
		t.Errorf("S0E2: %v %v", sp, err)
	}
	if _, err := get(0, 3, "zh"); err == nil {
		t.Error("special 3 must be NOT_FOUND")
	}
}

func TestContinuationNames(t *testing.T) {
	cases := map[string]bool{"间谍过家家 第2部分": true, "SPY×FAMILY 第2クール": true, "Show Part 2": true, "最终季 Part 3": true, "某作 后半": true, "某作 後半": true, "某作 第二部分": true,
		"间谍过家家 第二季": false, "Show 2nd Season": false, "Show Part 1": false, "某作 前半": false, "某作 第2期": false, "Partly Cloudy": false, "Show": false}
	for name, want := range cases {
		if got := isContinuation(&bgmSubject{NameCN: name}); got != want {
			t.Errorf("isContinuation(%q) = %v, want %v", name, got, want)
		}
	}
}

// A series released in two cours is one season: the second cour's episodes continue the numbering, and the next season is the one after it.
func TestSplitCourIsOneSeason(t *testing.T) {
	f := fixture()
	p := newTest(t, f)
	ctx := context.Background()
	n := func(v int32) *int32 { return &v }
	get := func(s, e int32) (*pluginv1.ItemMetadata, error) {
		return p.GetItem(ctx, &pluginv1.GetItemRequest{Type: pluginv1.ItemType_ITEM_TYPE_EPISODE, SeriesId: "1", SeasonNumber: n(s), EpisodeNumber: n(e), Languages: &pluginv1.Languages{Language: "zh-CN"}})
	}
	for e, want := range map[int32]string{12: "第12话", 13: "二期第1话", 18: "二期第6话"} {
		if ep, err := get(1, e); err != nil || ep.GetName() != want || ep.GetParentIndexNumber() != 1 || ep.GetIndexNumber() != e {
			t.Errorf("S1E%d: %v %v, want %q", e, ep, err, want)
		}
	}
	if _, err := get(1, 19); err == nil {
		t.Error("S1E19 must be NOT_FOUND: season 1 has 18 episodes")
	}
	// Season 2 is the next season, not the second cour.
	s2, err := p.GetItem(ctx, &pluginv1.GetItemRequest{Type: pluginv1.ItemType_ITEM_TYPE_SEASON, SeriesId: "1", SeasonNumber: n(2), Languages: &pluginv1.Languages{Language: "zh-CN"}})
	if err != nil || s2.GetProviderIds()["bangumi"] != "2" {
		t.Errorf("season 2: %v %v", s2, err)
	}
	if ep, err := get(2, 1); err != nil || ep.GetName() != "第1话" || ep.GetPremiereDate() != "2021-04-04" {
		t.Errorf("S2E1: %v %v", ep, err)
	}
	// When Bangumi numbers the second cour from 1 inside the subject (sort restarts too), the episodes still follow the first cour.
	f2 := fixture()
	var restart []bgmEpisode
	for i := 1; i <= 6; i++ {
		restart = append(restart, bgmEpisode{ID: i, Name: "r" + strconv.Itoa(i), Ep: float64(i), Sort: float64(i)})
	}
	f2.episodes[[2]int{6, 0}] = restart
	p2 := newTest(t, f2)
	if ep, err := p2.GetItem(ctx, &pluginv1.GetItemRequest{Type: pluginv1.ItemType_ITEM_TYPE_EPISODE, SeriesId: "1", SeasonNumber: n(1), EpisodeNumber: n(15)}); err != nil || ep.GetName() != "r3" {
		t.Errorf("restarted numbering, S1E15: %v %v", ep, err)
	}
}

func TestEpisodesUseInSubjectNumbering(t *testing.T) {
	f := fixture()
	// A split-cour subject whose episodes are numbered 13..24 across the series but 1..12 inside the subject.
	var list []bgmEpisode
	for i := 1; i <= 12; i++ {
		list = append(list, bgmEpisode{ID: i, Name: "x", Ep: float64(i), Sort: float64(12 + i)})
	}
	f.episodes[[2]int{1, 0}] = list
	p := newTest(t, f)
	one := int32(1)
	for _, e := range []int32{1, 12} {
		e := e
		if ep, err := p.GetItem(context.Background(), &pluginv1.GetItemRequest{Type: pluginv1.ItemType_ITEM_TYPE_EPISODE, SeriesId: "1", SeasonNumber: &one, EpisodeNumber: &e}); err != nil || ep.GetId() != strconv.Itoa(int(e)) {
			t.Errorf("episode %d: %v %v", e, ep, err)
		}
	}
}

func TestEpisodePaging(t *testing.T) {
	f := fixture()
	f.episodes[[2]int{1, 0}] = epList(250, "2020-04-05")
	f.subjects[1].Eps = 250
	p := newTest(t, f)
	one, last := int32(1), int32(250)
	ep, err := p.GetItem(context.Background(), &pluginv1.GetItemRequest{Type: pluginv1.ItemType_ITEM_TYPE_EPISODE, SeriesId: "1", SeasonNumber: &one, EpisodeNumber: &last})
	if err != nil || ep.GetIndexNumber() != 250 {
		t.Errorf("episode 250: %v %v", ep, err)
	}
}

func TestAbsoluteEpisode(t *testing.T) {
	f := fixture()
	p := newTest(t, f)
	abs := func(v int32) *pluginv1.GetItemRequest {
		return &pluginv1.GetItemRequest{Type: pluginv1.ItemType_ITEM_TYPE_EPISODE, SeriesId: "1", AbsoluteEpisodeNumber: &v}
	}
	// Season 1 is 12 + 6 episodes (two cours), season 2 has 13, season 3 is still airing.
	for _, c := range []struct{ abs, season, episode int32 }{{1, 1, 1}, {12, 1, 12}, {13, 1, 13}, {18, 1, 18}, {19, 2, 1}, {31, 2, 13}, {32, 3, 1}, {61, 3, 30}} {
		ep, err := p.GetItem(context.Background(), abs(c.abs))
		if err != nil || ep.GetParentIndexNumber() != c.season || ep.GetIndexNumber() != c.episode {
			t.Errorf("absolute %d: %v %v, want S%dE%d", c.abs, ep, err, c.season, c.episode)
		}
	}
}

func TestErrors(t *testing.T) {
	ctx := context.Background()
	p := newTest(t, fixture())
	if _, err := p.GetItem(ctx, &pluginv1.GetItemRequest{Type: pluginv1.ItemType_ITEM_TYPE_SERIES, Id: "999"}); err == nil {
		t.Error("missing title must be an error")
	}
	if _, err := p.GetItem(ctx, &pluginv1.GetItemRequest{Type: pluginv1.ItemType_ITEM_TYPE_SERIES, Id: "abc"}); err == nil {
		t.Error("bad ID must be an error")
	}
	limited := &fake{status: http.StatusTooManyRequests, headers: map[string]string{"Retry-After": "42"}, searchFor: func(string) []*bgmSubject { return nil }}
	_, err := newTest(t, limited).Search(ctx, &pluginv1.SearchRequest{Type: pluginv1.ItemType_ITEM_TYPE_SERIES, Name: "x"})
	rl, ok := errors.AsType[*sdk.RateLimitError](err)
	if !ok || rl.RetryAfter != 42*time.Second {
		t.Errorf("429: %v", err)
	}
	down := &fake{status: http.StatusBadGateway, searchFor: func(string) []*bgmSubject { return nil }}
	if _, err := newTest(t, down).Search(ctx, &pluginv1.SearchRequest{Type: pluginv1.ItemType_ITEM_TYPE_SERIES, Name: "x"}); err == nil {
		t.Error("502 must be an error")
	}
}

func TestGetPerson(t *testing.T) {
	f := fixture()
	f.people[70] = map[string]any{"id": 70, "name": "大塚明夫", "summary": "　　Voice actor.\r\n", "gender": "male", "birth_year": 1959, "birth_mon": 11, "birth_day": 24,
		"images": map[string]any{"large": "https://img.example/p.jpg"},
		"infobox": []any{
			map[string]any{"key": "简体中文名", "value": "大冢明夫"},
			map[string]any{"key": "别名", "value": []any{map[string]any{"k": "罗马字", "v": "Otsuka Akio"}}},
			map[string]any{"key": "血型", "value": "B型"}, map[string]any{"key": "身高", "value": "182cm"},
			map[string]any{"key": "出身地", "value": "東京都"}, map[string]any{"key": "事务所", "value": "Mausu Promotion"}}}
	p := newTest(t, f)
	got, err := p.GetPerson(context.Background(), &pluginv1.GetPersonRequest{Id: "70", Languages: &pluginv1.Languages{Language: "zh-CN"}})
	if err != nil {
		t.Fatal(err)
	}
	if got.GetName() != "大冢明夫" || got.GetBirthDate() != "1959-11-24" || got.GetBirthPlace() != "東京都" || got.GetGender() != pluginv1.Gender_GENDER_MALE || got.GetOverview() != "Voice actor." ||
		len(got.GetAliases()) != 2 || got.GetAliases()[0] != "大塚明夫" || got.GetProviderIds()["bangumi"] != "70" ||
		got.GetAttributes()["blood_type"] != "B" || got.GetAttributes()["height_cm"] != "182" || got.GetAttributes()["agency"] != "Mausu Promotion" || len(got.GetImages()) != 1 {
		t.Errorf("person %v", got)
	}
	ja, _ := p.GetPerson(context.Background(), &pluginv1.GetPersonRequest{Id: "70", Languages: &pluginv1.Languages{Language: "en"}})
	if ja.GetName() != "大塚明夫" {
		t.Errorf("non-Chinese name %q", ja.GetName())
	}
	if _, err := p.GetPerson(context.Background(), &pluginv1.GetPersonRequest{Id: "71"}); err == nil {
		t.Error("missing person must be an error")
	}
}
