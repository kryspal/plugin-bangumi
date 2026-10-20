# plugin-bangumi

Anime and anime film metadata from [Bangumi](https://bgm.tv) (Bangumi Keikaku), the Chinese-language anime database, for [Kryspal Media](https://github.com/kryspal). An official Kryspal Media plugin, built on [plugin-sdk](https://github.com/kryspal/plugin-sdk). No account or API key is needed.

Metadata from Bangumi (bgm.tv), licensed [CC BY-SA](https://creativecommons.org/licenses/by-sa/4.0/). This plugin is **unofficial**: it is not made, endorsed or certified by Bangumi. Bangumi asks API clients to send a User-Agent that names the application, and the plugin does. See [docs/design.md](docs/design.md#bangumi-terms) for what is and is not known about Bangumi's terms before you use it commercially.

## What it provides

- Chinese and original titles, synopsis, studios, genres, scores with vote counts and posters.
- Japanese voice actors with the characters they play, staff (director, writer, composer, producer) and people pages.
- Seasons, episodes and specials: for every episode its title, air date, runtime and synopsis; absolute episode numbers.

Bangumi lists each season as a separate subject. The plugin treats the subject a title is matched to as season 1 and follows its sequel relations to find seasons 2, 3 and so on (a series released in two cours is one season), so a library with `Show/Season 2` folders works. Special episodes are season 0.

## Installation

Install it from the Kryspal plugin catalog in the server's administration (Plugins). Official builds are signed and published through the catalog; builds from source are treated as third-party plugins. Enable it on an anime library, or on any library whose items it should look up; it needs no key and is ready as soon as it is enabled.

## Settings

| Setting | Default | Description |
| --- | --- | --- |
| Title language | Automatic | `Automatic` follows the language of the library: Chinese libraries get the Chinese title, Japanese libraries the original title, others an English alias when Bangumi lists one and the original title otherwise. `Chinese` and `Original` force one. |
| Voice actors per title (advanced) | 25 | 0 to 200. 0 lists no voice actors or characters; the staff are still listed. |
| API base URL (advanced) | `https://api.bgm.tv` | Only for a trusted HTTPS proxy of the Bangumi API. Must be `https`. |

## Known limitations

- Anime only. Bangumi's live-action, game, book and music entries are not used; live-action titles in a library go to another metadata plugin.
- Season structure follows Bangumi, one subject per broadcast run. A library that splits a long run into TMDB-style season folders, or that has more episodes than Bangumi lists for a run, cannot be laid onto it; use absolute episode numbers, or identify such a series by TMDB. A cour that Bangumi names after its story arc becomes a separate season.
- Only posters. Bangumi has no backdrops, logos or trailers.
- Studios come from the "animation production" entry, which many subjects lack.
- A title Bangumi translates differently from a library's folder name is found only through the other titles the server passes along (the original title, an alias); otherwise the user picks it.

## Building and testing

```sh
go build -o bangumi .
go vet ./... && go test ./...
BANGUMI_E2E=1 go test -run TestRealBangumi -v   # optional: a few read-only queries against the real API
```

Implementation notes: [docs/design.md](docs/design.md). Contributions: [CONTRIBUTING.md](CONTRIBUTING.md). Security: [SECURITY.md](SECURITY.md).

Licensed under the Apache License 2.0 (see `LICENSE` and `NOTICE`).
