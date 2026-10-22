# Changelog

All notable changes to this plugin are listed here. The version in `VERSION` is the single source of the version number; a published version is never changed.

## 1.1.1

- Built with plugin SDK v0.5.0. No functional change.

## 1.1.0 (2026-10-20)

First public version. Earlier builds (0.1.0) ran only on test servers.

- Movie and series search and details, seasons built from sequel relations, special episodes as season 0, episode titles, air dates, runtimes and synopses, absolute episode numbers, voice actors, staff and people pages.
- A series released in two cours is one season: a sequel named like a later part of the same season (part 2, also written "Part.2", the second cour, the second half, the latter part) joins the season before it, with its episodes numbered on from the first cour, so the next season is the real next season. Two sequels that carry the same season wording ("season 3, attack arc" and "season 3, counterattack arc") are one season too, and a one-episode sequel aired on TV is a special, not a season.
- A film or OVA among the sequels is not a season, but the walk goes on through it to the TV season that follows it, so a series whose third season is the sequel of a film still gets its third season.
- Episodes of a later part are numbered by their position after the earlier parts, whatever numbers Bangumi gives them (restarting at 1, continuing the season, or continuing the whole series). Series of more than 1000 episodes are read in full (up to 3000).
- Search, from a run against the real Bangumi with the names libraries use: a film is matched without Bangumi's leading "theatrical version" word; a remake that Bangumi names "Title 2009" is the identical match when that year is asked for; the later part of a season never outranks its first part; when nothing matched exactly and exactly one candidate extends the searched name and aired in the requested year, it is lifted just over the server's bar (two such candidates stay below it, so the user is asked); a Roman numeral character and full-width letters compare as plain letters.
- Search, from a read-only run over 100 series and films of a real anime library (titles and years only): a candidate without a date is not applied on a partial match when the request has a year; in a series search the later seasons of a series rank behind its first season, so a folder named after the series finds the series.
- A request that carries a Bangumi ID returns that subject as a confirmed candidate, which a server that knows the field applies on its own (older servers see a score of 1).
- Built with plugin SDK v0.4.0. Needs server 0.9.2 or newer.
