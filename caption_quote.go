package main

import (
	"errors"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

func quoteCharacter(r rune) rune {
	if unicode.IsSpace(r) || unicode.IsPunct(r) {
		return -1
	}
	return unicode.ToLower(r)
}

func findCaptionQuote(cues []captionCue, quote string) (captionCue, error) {
	query := strings.Map(quoteCharacter, quote)
	if query == "" {
		return captionCue{}, errors.New("provide words from the captions")
	}
	type position struct {
		stream, cue, from, to int
	}
	var positions []position
	var normalized strings.Builder
	for i, cue := range cues {
		for offset, r := range cue.Text {
			if key := quoteCharacter(r); key >= 0 {
				positions = append(positions, position{normalized.Len(), i, offset, offset + utf8.RuneLen(r)})
				normalized.WriteRune(key)
			}
		}
	}
	source := normalized.String()
	firstMatch := strings.Index(source, query)
	if firstMatch < 0 {
		return captionCue{}, errors.New("words do not match the captions in this interval")
	}
	bounds := func(match int) (position, position) {
		first := sort.Search(len(positions), func(i int) bool { return positions[i].stream >= match })
		last := sort.Search(len(positions), func(i int) bool { return positions[i].stream >= match+len(query) }) - 1
		return positions[first], positions[last]
	}
	first, last := bounds(firstMatch)
	if later := strings.LastIndex(source, query); later != firstMatch {
		laterFirst, laterLast := bounds(later)
		if first.cue != laterFirst.cue || last.cue != laterLast.cue {
			return captionCue{}, errors.New("words match multiple cue ranges. Select a narrower interval")
		}
	}
	var matched strings.Builder
	for i := first.cue; i <= last.cue; i++ {
		start, end := 0, len(cues[i].Text)
		if i == first.cue {
			start = first.from
		}
		if i == last.cue {
			end = last.to
		}
		if i > first.cue {
			matched.WriteByte(' ')
		}
		matched.WriteString(strings.TrimSpace(cues[i].Text[start:end]))
	}
	text := strings.Join(strings.Fields(matched.String()), " ")
	if utf8.RuneCountInString(text) > transcriptBudget {
		return captionCue{}, errors.New("select a shorter quote")
	}
	return captionCue{
		Start: cues[first.cue].Start,
		End:   cues[last.cue].End,
		Text:  text,
	}, nil
}
