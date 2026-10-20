package main

import (
	"sort"
	"strconv"
	"strings"

	pluginv1 "github.com/kryspal/plugin-sdk/pluginv1"
)

const sourceName = "Bangumi"

// notGenres are the meta tags that name the format or the country, not a genre.
var notGenres = map[string]bool{"TV": true, "WEB": true, "OVA": true, "OAD": true, "剧场版": true, "日本": true, "中国": true, "美国": true, "韩国": true, "欧美": true}

func providerIDs(s *bgmSubject) map[string]string {
	return map[string]string{pluginID: strconv.Itoa(s.ID)}
}

func gender(s string) pluginv1.Gender {
	switch strings.ToLower(s) {
	case "female":
		return pluginv1.Gender_GENDER_FEMALE
	case "male":
		return pluginv1.Gender_GENDER_MALE
	case "":
		return pluginv1.Gender_GENDER_UNSPECIFIED
	}
	return pluginv1.Gender_GENDER_OTHER
}

func genres(s *bgmSubject) []string {
	var out []string
	for _, t := range s.MetaTags {
		if !notGenres[t] {
			out = append(out, t)
		}
	}
	return out
}

func images(s *bgmSubject) []*pluginv1.Image {
	if u := s.Images.best(); u != "" {
		return []*pluginv1.Image{{Type: pluginv1.ImageType_IMAGE_TYPE_PRIMARY, Url: u, Source: sourceName}}
	}
	return nil
}

// staffKind maps a Bangumi staff relation to a credit kind; relations with no matching kind are dropped.
func staffKind(rel string) (pluginv1.PersonKind, bool) {
	switch rel {
	case "导演", "总导演", "监督":
		return pluginv1.PersonKind_PERSON_KIND_DIRECTOR, true
	case "原作", "脚本", "系列构成", "编剧", "剧本", "原案":
		return pluginv1.PersonKind_PERSON_KIND_WRITER, true
	case "音乐":
		return pluginv1.PersonKind_PERSON_KIND_COMPOSER, true
	case "制片人", "制作人", "监制":
		return pluginv1.PersonKind_PERSON_KIND_PRODUCER, true
	}
	return 0, false
}

// characterRank puts main characters before supporting ones, keeping the source order inside each group.
func characterRank(rel string) int {
	switch rel {
	case "主角":
		return 0
	case "配角":
		return 1
	}
	return 2
}

func credit(id int, name, imageURL string, kind pluginv1.PersonKind, role string, order int) *pluginv1.Credit {
	sid := strconv.Itoa(id)
	return &pluginv1.Credit{PersonId: sid, Name: name, Kind: kind, Role: role, Order: int32(order), ImageUrl: imageURL,
		ProviderIds: map[string]string{pluginID: sid}}
}

// credits lists the voice actors with the character they play (up to maxCast) and the staff that map to a credit kind.
func credits(chars []bgmCharacter, staff []bgmRelatedPerson, maxCast int) []*pluginv1.Credit {
	var out []*pluginv1.Credit
	seen := map[string]bool{}
	add := func(c *pluginv1.Credit) bool {
		if c.GetPersonId() == "0" || c.GetName() == "" {
			return false
		}
		key := c.GetPersonId() + "/" + c.GetKind().String()
		if seen[key] {
			return false
		}
		seen[key] = true
		out = append(out, c)
		return true
	}
	sorted := append([]bgmCharacter(nil), chars...)
	sort.SliceStable(sorted, func(i, j int) bool { return characterRank(sorted[i].Relation) < characterRank(sorted[j].Relation) })
	n := 0
loop:
	for _, c := range sorted {
		for _, a := range c.Actors {
			if n >= maxCast {
				break loop
			}
			if add(credit(a.ID, a.Name, a.Images.best(), pluginv1.PersonKind_PERSON_KIND_ACTOR, c.Name, n)) {
				n++
			}
		}
	}
	order := map[pluginv1.PersonKind]int{}
	for _, p := range staff {
		kind, ok := staffKind(p.Relation)
		if !ok {
			continue
		}
		if add(credit(p.ID, p.Name, p.Images.best(), kind, p.Relation, order[kind])) {
			order[kind]++
		}
	}
	return out
}

func baseMetadata(s *bgmSubject, t pluginv1.ItemType, set settings, l *pluginv1.Languages) *pluginv1.ItemMetadata {
	title, lang := pickTitle(s, set.titleLang, l)
	out := &pluginv1.ItemMetadata{
		Id: strconv.Itoa(s.ID), Type: t, Name: title, OriginalTitle: strings.TrimSpace(s.Name), Overview: cleanText(s.Summary),
		ProductionYear: int32(s.year()), PremiereDate: isoDate(s.Date), ProviderIds: providerIDs(s), Genres: genres(s),
		Images: images(s), Language: lang, OriginalLanguage: "ja",
	}
	for _, st := range infoValues(s.Infobox, "动画制作") {
		out.Studios = append(out.Studios, strings.Split(st, " / ")...)
	}
	if s.Rating.Score > 0 {
		out.CommunityRating = s.Rating.Score
		out.Ratings = []*pluginv1.Rating{{Source: pluginID, Value: s.Rating.Score, Max: 10, Votes: s.Rating.Total}}
	}
	if m := minutes(infoFirst(s.Infobox, "片长", "单集长度")); m > 0 {
		out.RuntimeSeconds = int64(m) * 60
	}
	if t == pluginv1.ItemType_ITEM_TYPE_SERIES {
		if end := cnDate(infoFirst(s.Infobox, "播放结束")); end != "" {
			out.EndDate = end
			out.Status = pluginv1.SeriesStatus_SERIES_STATUS_ENDED
		}
	}
	return out
}
