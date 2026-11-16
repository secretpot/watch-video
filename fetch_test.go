package main

import (
	"encoding/json"
	"testing"
)

func TestFetchSubtitleLanguage(t *testing.T) {
	for _, tc := range []struct {
		name     string
		metadata string
		options  fetchOptions
		language string
		optional bool
	}{
		{
			name:     "exact original authored track",
			metadata: `{"language":"pt-BR","subtitles":{"pt-BR":[{"ext":"vtt"}],"en":[{"ext":"vtt"}]}}`,
			options:  fetchOptions{autoSubtitle: true},
			language: "pt-BR",
			optional: true,
		},
		{
			name:     "explicit override wins",
			metadata: `{"language":"en","subtitles":{"en":[{"ext":"srt"}]}}`,
			options:  fetchOptions{subtitleLanguage: "de", autoSubtitle: true},
			language: "de",
		},
		{
			name:     "no automatic selection when disabled",
			metadata: `{"language":"en","subtitles":{"en":[{"ext":"srt"}]}}`,
		},
		{
			name:     "no inferred original language",
			metadata: `{"subtitles":{"en":[{"ext":"srt"}]}}`,
			options:  fetchOptions{autoSubtitle: true},
		},
		{
			name:     "no short language tag fallback",
			metadata: `{"language":"en-GB","subtitles":{"en":[{"ext":"srt"}]}}`,
			options:  fetchOptions{autoSubtitle: true},
		},
		{
			name:     "no automatic captions",
			metadata: `{"language":"en","subtitles":{"fr":[{"ext":"srt"}]},"automatic_captions":{"en":[{"ext":"vtt"}]}}`,
			options:  fetchOptions{autoSubtitle: true},
		},
		{
			name:     "no empty track",
			metadata: `{"language":"en","subtitles":{"en":[]}}`,
			options:  fetchOptions{autoSubtitle: true},
		},
		{
			name:     "invalid language tag",
			metadata: `{"language":"../en","subtitles":{"../en":[{"ext":"srt"}]}}`,
			options:  fetchOptions{autoSubtitle: true},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var metadata fetchMetadata
			if err := json.Unmarshal([]byte(tc.metadata), &metadata); err != nil {
				t.Fatal(err)
			}
			language, optional := fetchSubtitleLanguage(tc.options, metadata)
			if language != tc.language || optional != tc.optional {
				t.Fatalf("got (%q, %t), want (%q, %t)", language, optional, tc.language, tc.optional)
			}
		})
	}
}
