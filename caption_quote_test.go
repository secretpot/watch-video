package main

import "testing"

func TestCaptionQuoteFollowsItsFirstAndLastCues(t *testing.T) {
	cues := []captionCue{
		{Start: 869.32, End: 870.32, Text: "不能"},
		{Start: 870.32, End: 873.48, Text: "现实里的恶通常是高度组织化的"},
		{Start: 873.48, End: 874.64, Text: "他必须有同谋"},
		{Start: 874.64, End: 876, Text: "有上下级执行链，最后"},
		{Start: 876, End: 876.96, Text: "有资金划拨"},
	}
	match, err := findCaptionQuote(cues, "恶通常是高度组织化的：他必须有同谋，有上下级执行链。")
	want := captionCue{Start: 870.32, End: 876, Text: "恶通常是高度组织化的 他必须有同谋 有上下级执行链"}
	if err != nil || match != want {
		t.Fatalf("matched %+v, %v. Want %+v", match, err, want)
	}
}

func TestCaptionQuoteRejectsAmbiguousOrEditedWords(t *testing.T) {
	cues := []captionCue{{Start: 5, End: 6, Text: "One result"}, {Start: 7, End: 8, Text: "one result"}}
	for _, query := range []string{"ONE result!", "one improved result", "？"} {
		if match, err := findCaptionQuote(cues, query); err == nil {
			t.Fatalf("unexpected match of %q: %+v", query, match)
		}
	}
}
