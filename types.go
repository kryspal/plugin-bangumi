package main

// The structs below mirror the parts of Bangumi's v0 API the plugin reads.

type bgmImages struct {
	Large  string `json:"large"`
	Common string `json:"common"`
	Medium string `json:"medium"`
}

// best is the largest image URL that is set.
func (i bgmImages) best() string { return firstNonEmpty(i.Large, i.Common, i.Medium) }

type bgmInfo struct {
	Key   string `json:"key"`
	Value any    `json:"value"` // a string, or a list of {k, v} objects
}

type bgmRating struct {
	Score float64 `json:"score"`
	Total int64   `json:"total"`
}

type bgmSubject struct {
	full          bool      // fetched by ID, so it can be used as the series; search results are not complete
	ID            int       `json:"id"`
	Type          int       `json:"type"`
	Name          string    `json:"name"`
	NameCN        string    `json:"name_cn"`
	Summary       string    `json:"summary"`
	Date          string    `json:"date"`
	Platform      string    `json:"platform"`
	Images        bgmImages `json:"images"`
	Infobox       []bgmInfo `json:"infobox"`
	Rating        bgmRating `json:"rating"`
	Eps           int       `json:"eps"`
	TotalEpisodes int       `json:"total_episodes"`
	MetaTags      []string  `json:"meta_tags"`
	NSFW          bool      `json:"nsfw"`
	Collection    struct {
		Collect int `json:"collect"`
	} `json:"collection"`
}

type bgmEpisode struct {
	ID              int     `json:"id"`
	Name            string  `json:"name"`
	NameCN          string  `json:"name_cn"`
	Desc            string  `json:"desc"`
	Airdate         string  `json:"airdate"`
	Ep              float64 `json:"ep"`
	Sort            float64 `json:"sort"`
	DurationSeconds int     `json:"duration_seconds"`
	Type            int     `json:"type"`
}

type bgmEpisodes struct {
	Total int          `json:"total"`
	Data  []bgmEpisode `json:"data"`
}

type bgmRelation struct {
	ID       int    `json:"id"`
	Type     int    `json:"type"`
	Relation string `json:"relation"`
}

type bgmRelatedPerson struct {
	ID       int       `json:"id"`
	Name     string    `json:"name"`
	Relation string    `json:"relation"`
	Images   bgmImages `json:"images"`
}

type bgmCharacter struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Relation string `json:"relation"`
	Actors   []struct {
		ID     int       `json:"id"`
		Name   string    `json:"name"`
		Images bgmImages `json:"images"`
	} `json:"actors"`
}

type bgmPerson struct {
	ID      int       `json:"id"`
	Name    string    `json:"name"`
	Summary string    `json:"summary"`
	Gender  string    `json:"gender"`
	Images  bgmImages `json:"images"`
	Infobox []bgmInfo `json:"infobox"`
	BirthY  int       `json:"birth_year"`
	BirthM  int       `json:"birth_mon"`
	BirthD  int       `json:"birth_day"`
}
