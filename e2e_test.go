package main

import (
	"context"
	"os"
	"testing"
	"time"

	pluginv1 "github.com/kryspal/plugin-sdk/pluginv1"
)

// TestRealBangumi runs a few read-only queries for a well-known public title against the real Bangumi. It is skipped unless
// BANGUMI_E2E=1, so CI and ordinary runs never touch the network.
func TestRealBangumi(t *testing.T) {
	if os.Getenv("BANGUMI_E2E") != "1" {
		t.Skip("BANGUMI_E2E not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	p := newPlugin()
	zh := &pluginv1.Languages{Language: "zh-CN"}
	res, err := p.Search(ctx, &pluginv1.SearchRequest{Type: pluginv1.ItemType_ITEM_TYPE_SERIES, Name: "灵能百分百", Year: 2016, Languages: zh})
	if err != nil || len(res.GetCandidates()) == 0 {
		t.Fatalf("search: %v %v", res, err)
	}
	for _, c := range res.GetCandidates()[:min(3, len(res.GetCandidates()))] {
		t.Logf("candidate %s %q (%d) score=%.2f matched=%q", c.GetId(), c.GetName(), c.GetYear(), c.GetScore(), c.GetMatchedTitle())
	}
	id := res.GetCandidates()[0].GetId()
	it, err := p.GetItem(ctx, &pluginv1.GetItemRequest{Type: pluginv1.ItemType_ITEM_TYPE_SERIES, Id: id, Languages: zh})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("series %q year=%d end=%s status=%v studios=%v genres=%v rating=%.1f credits=%d images=%d", it.GetName(), it.GetProductionYear(), it.GetEndDate(),
		it.GetStatus(), it.GetStudios(), it.GetGenres(), it.GetCommunityRating(), len(it.GetCredits()), len(it.GetImages()))
	zero, one, two := int32(0), int32(1), int32(2)
	s2, err := p.GetItem(ctx, &pluginv1.GetItemRequest{Type: pluginv1.ItemType_ITEM_TYPE_SEASON, SeriesId: id, SeasonNumber: &two, Languages: zh})
	if err != nil {
		t.Fatalf("season 2: %v", err)
	}
	t.Logf("season 2 = subject %s %q", s2.GetProviderIds()["bangumi"], s2.GetName())
	ep, err := p.GetItem(ctx, &pluginv1.GetItemRequest{Type: pluginv1.ItemType_ITEM_TYPE_EPISODE, SeriesId: id, SeasonNumber: &one, EpisodeNumber: &one, Languages: zh})
	if err != nil {
		t.Fatalf("episode: %v", err)
	}
	t.Logf("S1E1 name=%q date=%s runtime=%d overview=%d chars", ep.GetName(), ep.GetPremiereDate(), ep.GetRuntimeSeconds(), len([]rune(ep.GetOverview())))
	sp, err := p.GetItem(ctx, &pluginv1.GetItemRequest{Type: pluginv1.ItemType_ITEM_TYPE_EPISODE, SeriesId: id, SeasonNumber: &zero, EpisodeNumber: &one, Languages: zh})
	t.Logf("S0E1 %q %v", sp.GetName(), err)
	for _, c := range it.GetCredits() {
		if c.GetKind() == pluginv1.PersonKind_PERSON_KIND_ACTOR {
			per, err := p.GetPerson(ctx, &pluginv1.GetPersonRequest{Id: c.GetPersonId(), Languages: zh})
			if err != nil {
				t.Fatalf("person: %v", err)
			}
			t.Logf("person %q born=%s aliases=%v attrs=%v", per.GetName(), per.GetBirthDate(), per.GetAliases(), per.GetAttributes())
			break
		}
	}
	mv, err := p.Search(ctx, &pluginv1.SearchRequest{Type: pluginv1.ItemType_ITEM_TYPE_MOVIE, Name: "你的名字。", Languages: zh})
	if err != nil || len(mv.GetCandidates()) == 0 {
		t.Fatalf("movie search: %v %v", mv, err)
	}
	t.Logf("movie %s %q score=%.2f", mv.GetCandidates()[0].GetId(), mv.GetCandidates()[0].GetName(), mv.GetCandidates()[0].GetScore())
}
