package audio

import "testing"

func TestWhisperSilenceAnnotations(t *testing.T) {
	for _, tt := range []struct{ input, want string }{
		{"[BLANK_AUDIO][BLANK_AUDIO] Tell me a story.", " Tell me a story."},
		{" [BLANK_AUDIO] \n [BLANK_AUDIO]", ""},
		{" café 日本語 [BLANK_AUDIO]", " café 日本語 "},
		{"Say the literal [BLANK_AUDIO] token.", "Say the literal [BLANK_AUDIO] token."},
		{" ordinary words ", " ordinary words "},
	} {
		if got := whisperSpeechText(tt.input); got != tt.want {
			t.Errorf("%q: got %q, want %q", tt.input, got, tt.want)
		}
	}
}
