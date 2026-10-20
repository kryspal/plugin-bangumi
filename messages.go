package main

import "github.com/kryspal/plugin-sdk/i18n"

// messages is the declared string table: en, zh-TW and zh-CN; other languages follow the i18n fallback chain, ending in English.
// The attribution (attribution.text) is the English original, the same in every language, and is not here.
var messages = i18n.New(messageTables)

var messageTables = map[string]map[string]string{
	"en": {
		"desc":          "Fetches anime and anime film metadata from Bangumi (bgm.tv), the Chinese-language anime database: Chinese and original titles, synopsis, studios, scores, posters, voice actors, staff, people pages and episode titles with air dates and synopses. Bangumi lists every season as its own entry, so the plugin treats the entry a title is matched to as season 1 and follows its sequels for the later seasons; special episodes become season 0. No account or key is needed. This plugin is unofficial and is not made or endorsed by Bangumi. Metadata from Bangumi (bgm.tv), CC BY-SA.",
		"groupAdvanced": "Advanced",
		"titleLabel":    "Title language",
		"titleHelp":     "Which Bangumi title to use as the name. Automatic follows the language of the library: Chinese libraries get the Chinese title, Japanese libraries get the original title, and the others get an English alias when Bangumi lists one and the original title otherwise.",
		"titleAuto":     "Automatic",
		"titleChinese":  "Chinese",
		"titleNative":   "Original",
		"maxCastLabel":  "Maximum cast",
		"maxCastHelp":   "Upper limit on the number of voice actors per title, 0 to 200. 0 turns the cast off; the staff are still listed.",
		"baseURLLabel":  "API address",
		"baseURLHelp":   "Leave empty to use the official API. Only for a trusted HTTPS proxy.",
		"specials":      "Specials",
		"errHTTPS":      "Must be an https:// address.",
		"errTitleLang":  "Choose one of the offered title languages.",
		"errMaxCast":    "Enter a whole number from 0 to 200.",
	},
	"zh-TW": {
		"desc":          "從 Bangumi 番組計畫（bgm.tv）取得動畫與動畫電影的資料：中文與原文標題、簡介、製作公司、評分、海報、聲優、工作人員、人物頁面，以及每集的標題、播出日期與簡介。Bangumi 把每一季各自當成一個條目，所以外掛把配對到的條目當作第 1 季，再順著續集關係找出後面的季；特別篇則歸入第 0 季。不需要帳號或金鑰。這個外掛是非官方的，不是 Bangumi 製作或背書。Metadata from Bangumi (bgm.tv), CC BY-SA.",
		"groupAdvanced": "進階",
		"titleLabel":    "標題語言",
		"titleHelp":     "用 Bangumi 的哪一種標題當名稱。自動會依媒體庫語言決定：中文媒體庫用中文標題，日文媒體庫用原文標題，其他語言在 Bangumi 有英文別名時用英文別名，否則用原文標題。",
		"titleAuto":     "自動",
		"titleChinese":  "中文",
		"titleNative":   "原文",
		"maxCastLabel":  "演員人數上限",
		"maxCastHelp":   "每部作品的聲優人數上限，0 到 200。0 表示不取得演員，工作人員仍會列出。",
		"baseURLLabel":  "API 網址",
		"baseURLHelp":   "留空表示使用官方 API。只用於可信任的 HTTPS 代理。",
		"specials":      "特別篇",
		"errHTTPS":      "必須是 https:// 網址。",
		"errTitleLang":  "請從選項中選一種標題語言。",
		"errMaxCast":    "請輸入 0 到 200 的整數。",
	},
	"zh-CN": {
		"desc":          "从 Bangumi 番组计划（bgm.tv）获取动画和动画电影的资料：中文和原文标题、简介、制作公司、评分、海报、声优、工作人员、人物页面，以及每集的标题、播出日期和简介。Bangumi 把每一季各自当成一个条目，所以插件把匹配到的条目当作第 1 季，再顺着续集关系找出后面的季；特别篇则归入第 0 季。不需要账号或密钥。这个插件是非官方的，不是 Bangumi 制作或背书。Metadata from Bangumi (bgm.tv), CC BY-SA.",
		"groupAdvanced": "高级",
		"titleLabel":    "标题语言",
		"titleHelp":     "用 Bangumi 的哪一种标题当名称。自动会按媒体库语言决定：中文媒体库用中文标题，日文媒体库用原文标题，其他语言在 Bangumi 有英文别名时用英文别名，否则用原文标题。",
		"titleAuto":     "自动",
		"titleChinese":  "中文",
		"titleNative":   "原文",
		"maxCastLabel":  "演员人数上限",
		"maxCastHelp":   "每部作品的声优人数上限，0 到 200。0 表示不获取演员，工作人员仍会列出。",
		"baseURLLabel":  "API 网址",
		"baseURLHelp":   "留空表示使用官方 API。只用于可信任的 HTTPS 代理。",
		"specials":      "特别篇",
		"errHTTPS":      "必须是 https:// 网址。",
		"errTitleLang":  "请从选项中选一种标题语言。",
		"errMaxCast":    "请输入 0 到 200 的整数。",
	},
}
