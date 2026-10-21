# bangumi: a metadata provider for Bangumi

A Kryspal Media plugin on protocol v1 (plugin SDK v0.4.0). It declares `metadata.movie`, `metadata.series`, `metadata.series.absolute`, `metadata.search.hints` and `metadata.person`. It needs no key, so it is ready as soon as it is enabled; `Configure` makes no network call. It uses Bangumi's public v0 API (`https://api.bgm.tv`) and only reads.

## Bangumi terms

Checked on 2026-10-20:

- **User-Agent** (required): Bangumi's API guideline asks non-browser clients for a User-Agent with the developer, the application, the version for distributed clients, and the project page for open source ones, and says the default agents of HTTP libraries may be banned. The plugin sends `kryspal/plugin-bangumi/<version> (https://github.com/kryspal/plugin-bangumi)`; a test fails if any request lacks that shape.
- **Rate limit**: none is published in the API repository and the responses carry no limit headers. The plugin sends one request at a time, at most about three a second, and honors `Retry-After` on a 429 (a minute when it is missing).
- **Licence and use of the data** (the copyright statement at bgm.tv/about/copyright, read on 2026-10-20): subject and character information is CC BY-SA, so the source must be named; developers may build plugins on the API; Bangumi's data must not be handed to third parties on the sly; users' own content (comments, the short reactions called tucao, images they posted) must not be reused; data may be taken only through the API, never by scraping pages. "No commercial use without permission" refers to the logo, the page design and the graphics. This plugin runs on the user's own server and calls the API directly; nothing passes through Kryspal's servers.
- **What the plugin does about it**: the source is named in the manifest attribution ("Metadata from Bangumi (bgm.tv), licensed CC BY-SA", shown on the plugin's page), in the plugin description (all languages), in `CATALOG.json` and in the README. It reads only the API's subject, character, person, relation and episode records; it never reads comments, tucao reactions or user collections, never fetches web pages, and does not use Bangumi's logo (the manifest has no logo). Images are the cover and people photos that the API returns.
- **Adult titles**: the public API hides them from anonymous clients, so the plugin has no setting for them.
- **Storage**: the plugin keeps subjects, season chains and episode lists in memory for 30 minutes only. The server stores the metadata it scrapes, as it does for any provider.

## Model: every season is its own subject

- The subject a title is matched to is the series, and season 1.
- Season N is found by following the sequel relation (named `xuji` in Chinese in the API) of type anime. The relation list has no format, so the first three sequel candidates are fetched and the first one whose platform is `TV` or `WEB` wins; films (platform `juchangban`), OVAs and the like are not seasons, but when no TV or web sequel is found among them the walk looks at their own sequels, one level down (`maxSequelDepth`), because a TV season is sometimes the sequel of the film before it. A sequel with a single episode is a special that Bangumi files as a TV subject (a recap, an anniversary episode) and is passed over. The walk stops at the end of the chain, at a repeated subject or after 30 subjects, and is cached for 30 minutes.
- **Split cours are one season.** Bangumi makes each cour of a split series its own subject, with a name such as "part 2" or "the second cour" (the second half, the latter part). A sequel whose Chinese or original name matches that wording (`continuationRE` in `item.go`: part or cour 2 to 9, "part N" or "Part.N", the second half, the latter part) joins the season before it instead of starting a new one, so the series is one season with all its episodes, as on TMDB, and the next season is the next real season. So does a sequel whose name carries the same season wording as the season's first subject (`seasonMark`: "season 3" with an arc name on both). A sequel with its own subtitle and no season wording is a new season: by its name alone a cour named after its arc cannot be told from a sequel ("After Story"), and Bangumi's arc-named cours (the train arc and the district arc of one series) therefore come out as separate seasons where TMDB has one; identify such a library by TMDB, or use absolute numbers. The episodes of a later part are numbered by their position after the earlier parts, because Bangumi's own numbers of a later cour vary (restarting at 1, continuing the season, or continuing the whole series). A season's name, synopsis and poster come from its first subject.
- Season 0 is the special episodes (episode type 1) of the series subject. A special is numbered by its position, not by its number, because Bangumi's special numbers are unreliable (they often all read 0). Season 0 itself is named "Specials" in the request language.
- Main episodes are matched by `ep`, the number inside the subject (so a split-cour subject whose episodes are 13 to 24 across the series still matches 1 to 12 locally), or by `sort` when `ep` is 0.
- If a title is matched to a later subject (for example "second season" in the Chinese folder name), that subject becomes season 1. Fix it by identifying the title as the first subject.
- Absolute episode numbers add up the episode counts of each season (all its parts); a season with a part of unknown count (still airing) takes every number after it.
- Episode lists are read 200 at a time (Bangumi's maximum), at most 1000.

## Search

`POST /v0/search/subjects` with the keyword, `sort: match` and `type: [2]` (anime). Bangumi cannot filter by format, so the plugin asks for up to twice the limit and keeps films for a movie search and everything else for a series search.

- At most 3 queries: the name, then the other non-variant hinted titles. It stops once any result scores 0.9 or more.
- Score (per query against every title of the candidate: original name, Chinese name and the infobox aliases): identical 1; the query extends the candidate 0.95 (candidate of 3 or more characters); the candidate extends the query 0.85; contained 0.8; otherwise bigram similarity up to 0.5. Titles are compared as lowercase letters and digits.
- Year: 0 difference none, 1 year -0.03, 2 years -0.06 when the title is identical and -0.15 otherwise, more -0.3. A request without a year costs nothing (`year.unknown`); a candidate without a date when the request has a year costs 0.1 (`year.missing`), so an undated subject that merely extends the name cannot be applied on its own (a library's "Winnie the Pooh: Springtime (2004)" against an undated Soviet "Winnie the Pooh").
- Series search: a candidate whose name carries a later-season mark (the Chinese "season 2", "part 2", `Season 2`, `2nd Season`, "final season") is capped at 0.8 unless it matched exactly, so a library folder named after the series finds the first season and the later seasons stay behind it (`later_season`). A movie search does not do this.
- Ties go to the subject more people have collected. `matched_title` is the matching title; `reasons` are `title.exact|prefix|partial|weak`, `year.exact|diff1|diff2|far|unknown`, `id.exact`.
- A request that carries `provider_ids.bangumi` is looked up directly; a hit of the right kind is returned alone with score 1 and `confirmed` set, so a host that knows the field (host feature `search_confirmed`) applies it without looking at the score.

## Metadata

| Field | Source |
| --- | --- |
| name | `name_cn` or `name` (below) |
| original title | `name` |
| overview | `summary`, paragraphs un-indented |
| date, year | `date` (complete dates only) |
| end date, status | infobox "end of broadcast" (`bofang jieshu` in Chinese); a series with one is ended, otherwise the status is left unknown |
| rating | `rating.score` with `rating.total` votes, as a `ratings` entry with source `bangumi` |
| genres | `meta_tags` without the format and country tags |
| studios | infobox "animation production" (`donghua zhizuo` in Chinese) |
| runtime | infobox "running time" or "episode length"; for an episode, `duration_seconds` |
| images | the cover as the poster (Bangumi has no other artwork) |
| credits | voice actors from the characters list, main characters first, each with the character as the role (up to `max_cast`); staff whose relation is a director (the Chinese relations for director, chief director and supervisor), writer (original work, script, series composition, screenwriter, screenplay, original plan), composer (music) or producer (producer, production lead, executive producer); the exact words are in `staffKind` in `convert.go` |
| IDs | `bangumi` (the plugin ID) |

Titles: with the setting on Automatic, Chinese libraries get `name_cn` (the original name when there is none), Japanese libraries get the original name, and all others get the first Latin-letter alias in the infobox or else the original name. The setting can force the Chinese or the original title. Episode titles follow the same preference between `name_cn` and `name`.

People: `GetPerson` returns the name, the Simplified Chinese name (used as the name for Chinese libraries) and aliases, biography, birth date (when complete), birthplace, gender, photo, a link to the page, and the attributes `height_cm`, `blood_type` and `agency` when the infobox has them. Characters are not people and have no pages.

## Settings

| Key | Type | Description |
| --- | --- | --- |
| `title_language` | select | `auto` (default), `chinese` or `native` |
| `max_cast` | int (advanced) | Voice actors per title, 0 to 200, default 25; 0 skips the characters request, the staff are still listed |
| `base_url` | string (advanced) | Default `https://api.bgm.tv`; only for a trusted HTTPS proxy, and it must be https |

## Tests

`plugin_test.go` runs against a fake API server: the manifest in four languages, settings validation, title choice, text helpers, search (film/series filtering, scoring, aliases, hint titles, ID lookup), the full series mapping with credits, the season chain (skipping a film among the sequels) and its cache, season 0, episodes (both languages, in-subject numbering, paging past 200, specials by position), absolute numbers, errors (404, 429 with `Retry-After`, 502) and people. Every fake request is checked for the User-Agent shape. `conformance_test.go` runs the SDK suite on the built binary. `e2e_test.go` has an optional read-only run against the real API (`BANGUMI_E2E=1`), done on 2026-10-20 for a series with sequels, its first episode and specials, a voice actor and a film.

## Not done

- Live-action titles (Bangumi's type 6) and games, books and music are not covered; only anime.
- Only the cover image: Bangumi has no backdrops or logos.
- No trailers.
- Studios are read from the "animation production" entry, which many subjects lack.
- Season structure follows Bangumi, one subject per broadcast run. A library that splits a long run into TMDB-style season folders (four folders of a show Bangumi has as one subject of 65 episodes) or that has more episodes than Bangumi lists cannot be laid onto it by the plugin; absolute numbers, or identification by TMDB, are the way out. Seen in a read-only run over 100 titles of a real library (2026-10-20): 77 applied on their own, all with the right title and year; 1 asked the user (two seasons of one show, the same year); 21 below the bar, nearly all live-action or children's content Bangumi does not have, or titles whose Chinese translation differs from Bangumi's; 1 with no result.
