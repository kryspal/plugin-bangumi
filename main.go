// Command bangumi is an unofficial metadata provider plugin for Bangumi (bgm.tv), the Chinese-language anime database (capabilities
// metadata.movie, metadata.series, metadata.series.absolute, metadata.search.hints, metadata.person).
//
// Bangumi lists every season as its own subject. The plugin treats the subject a title is matched to as season 1 and follows its
// sequel relations to build the later seasons; special episodes become season 0. No account or key is needed; Bangumi asks API
// clients for a User-Agent that names the application, and the plugin sends one.
package main

import "github.com/kryspal/plugin-sdk/sdk"

func main() {
	p := newPlugin()
	sdk.Main(sdk.Config{Plugin: p, MetadataProvider: p})
}
