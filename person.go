package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	pluginv1 "github.com/kryspal/plugin-sdk/pluginv1"
	"github.com/kryspal/plugin-sdk/sdk"
)

// GetPerson implements pluginv1.MetadataProviderServer. Voice actors and staff are Bangumi persons.
func (p *plugin) GetPerson(ctx context.Context, req *pluginv1.GetPersonRequest) (*pluginv1.PersonMetadata, error) {
	id, err := strconv.Atoi(req.GetId())
	if err != nil || id <= 0 {
		return nil, sdk.InvalidArgument("id must be a Bangumi person ID")
	}
	var per bgmPerson
	if err := p.api.get(ctx, p.settings().baseURL, "/v0/persons/"+strconv.Itoa(id), nil, &per); err != nil {
		return nil, mapErr(err, "person")
	}
	sid := strconv.Itoa(per.ID)
	out := &pluginv1.PersonMetadata{
		Id: sid, Name: per.Name, Overview: cleanText(per.Summary), Gender: gender(per.Gender), ProviderIds: map[string]string{pluginID: sid},
		Links: []*pluginv1.Link{{Kind: pluginID, Url: "https://bgm.tv/person/" + sid}},
	}
	// A Chinese reader gets the Simplified Chinese name when Bangumi has one; the original name becomes an alias.
	cn := infoFirst(per.Infobox, "简体中文名")
	if cn != "" && languageKind(req.GetLanguages()) == "zh" {
		out.Name = cn
	}
	seen := map[string]bool{out.Name: true}
	for _, n := range append(append([]string{per.Name, cn}, infoValues(per.Infobox, "别名")...), infoValues(per.Infobox, "昵称")...) {
		if n = strings.TrimSpace(n); n != "" && !seen[n] {
			seen[n] = true
			out.Aliases = append(out.Aliases, n)
		}
	}
	if per.BirthY > 0 && per.BirthM > 0 && per.BirthD > 0 {
		out.BirthDate = fmt.Sprintf("%04d-%02d-%02d", per.BirthY, per.BirthM, per.BirthD)
	}
	out.BirthPlace = infoFirst(per.Infobox, "出生地", "出身地")
	if u := per.Images.best(); u != "" {
		out.Images = []*pluginv1.Image{{Type: pluginv1.ImageType_IMAGE_TYPE_PRIMARY, Url: u, Source: sourceName}}
	}
	attrs := map[string]string{}
	if h := atoiFirst(infoFirst(per.Infobox, "身高")); h > 0 {
		attrs["height_cm"] = strconv.Itoa(h)
	}
	if b := strings.TrimSpace(strings.TrimSuffix(infoFirst(per.Infobox, "血型"), "型")); b != "" {
		attrs["blood_type"] = b
	}
	if a := infoFirst(per.Infobox, "事务所", "所属事务所"); a != "" {
		attrs["agency"] = a
	}
	if len(attrs) > 0 {
		out.Attributes = attrs
	}
	return out, nil
}
