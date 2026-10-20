package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	pluginv1 "github.com/kryspal/plugin-sdk/pluginv1"
)

var (
	cnDateRE  = regexp.MustCompile(`(\d{4})年(\d{1,2})月(\d{1,2})日`)
	isoDateRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	minutesRE = regexp.MustCompile(`(\d+)\s*(?:分|min)`)
	digitsRE  = regexp.MustCompile(`\d+`)
	blankRE   = regexp.MustCompile(`\n{3,}`)
)

// cleanText turns Bangumi text into plain text: CRLF to LF, the full-width indent of each paragraph removed, blank lines collapsed.
func cleanText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimSpace(strings.TrimLeft(l, "　 \t"))
	}
	return strings.TrimSpace(blankRE.ReplaceAllString(strings.Join(lines, "\n"), "\n\n"))
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return strings.TrimSpace(string(r[:n-1])) + "…"
}

func firstNonEmpty(s ...string) string {
	for _, v := range s {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// infoValues lists the values of the first infobox entry with one of the keys: one string, or the "v" of each item of a list.
func infoValues(box []bgmInfo, keys ...string) []string {
	for _, k := range keys {
		for _, it := range box {
			if it.Key != k {
				continue
			}
			var out []string
			switch v := it.Value.(type) {
			case string:
				if s := strings.TrimSpace(v); s != "" {
					out = append(out, s)
				}
			case []any:
				for _, e := range v {
					if m, ok := e.(map[string]any); ok {
						if s, _ := m["v"].(string); strings.TrimSpace(s) != "" {
							out = append(out, strings.TrimSpace(s))
						}
					}
				}
			}
			if len(out) > 0 {
				return out
			}
		}
	}
	return nil
}

func infoFirst(box []bgmInfo, keys ...string) string {
	if v := infoValues(box, keys...); len(v) > 0 {
		return v[0]
	}
	return ""
}

// cnDate converts a date written with the Chinese year, month and day characters (2016, 9, 26) to 2016-09-26; it is empty when the date is not complete.
func cnDate(s string) string {
	m := cnDateRE.FindStringSubmatch(s)
	if m == nil {
		return ""
	}
	y, _ := strconv.Atoi(m[1])
	mo, _ := strconv.Atoi(m[2])
	d, _ := strconv.Atoi(m[3])
	return fmt.Sprintf("%04d-%02d-%02d", y, mo, d)
}

// isoDate returns s when it is a complete YYYY-MM-DD date.
func isoDate(s string) string {
	if isoDateRE.MatchString(s) && s != "0000-00-00" {
		if _, err := time.Parse("2006-01-02", s); err == nil {
			return s
		}
	}
	return ""
}

func minutes(s string) int {
	if m := minutesRE.FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return 0
}

func atoiFirst(s string) int {
	n, _ := strconv.Atoi(digitsRE.FindString(s))
	return n
}

// isLatin reports whether s has Latin letters and no Han, kana or Hangul: an English or romaji name.
func isLatin(s string) bool {
	latin := false
	for _, r := range s {
		switch {
		case unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Hangul, r):
			return false
		case unicode.Is(unicode.Latin, r):
			latin = true
		}
	}
	return latin
}

func languageKind(l *pluginv1.Languages) string {
	tag := strings.ToLower(l.GetLanguage())
	switch {
	case strings.HasPrefix(tag, "ja"):
		return "ja"
	case strings.HasPrefix(tag, "zh"):
		return "zh"
	}
	return "other"
}

// pickTitle chooses the name to show and its language (BCP 47).
func pickTitle(s *bgmSubject, pref string, l *pluginv1.Languages) (title, lang string) {
	native := strings.TrimSpace(s.Name)
	chinese := func() (string, string) {
		if cn := strings.TrimSpace(s.NameCN); cn != "" {
			return cn, "zh"
		}
		return native, "ja"
	}
	switch pref {
	case titleChinese:
		return chinese()
	case titleNative:
		return firstNonEmpty(native, s.NameCN), "ja"
	}
	switch languageKind(l) {
	case "zh":
		return chinese()
	case "ja":
		return firstNonEmpty(native, s.NameCN), "ja"
	}
	for _, a := range infoValues(s.Infobox, "别名") {
		if isLatin(a) {
			return a, "en"
		}
	}
	return firstNonEmpty(native, s.NameCN), "ja"
}

func (s *bgmSubject) year() int {
	if len(s.Date) >= 4 {
		n, _ := strconv.Atoi(s.Date[:4])
		return n
	}
	return 0
}

// allTitles are every name Bangumi knows for the subject, for matching.
func (s *bgmSubject) allTitles() []string {
	var out []string
	for _, t := range append([]string{s.Name, s.NameCN}, infoValues(s.Infobox, "别名")...) {
		if strings.TrimSpace(t) != "" {
			out = append(out, strings.TrimSpace(t))
		}
	}
	return out
}

func (s *bgmSubject) isMovie() bool { return s.Platform == "剧场版" }

// episodeCount is the number of main episodes Bangumi lists; 0 when unknown.
func (s *bgmSubject) episodeCount() int {
	if s.Eps > 0 {
		return s.Eps
	}
	return s.TotalEpisodes
}
