package audio

import "regexp"

// whisper.cpp represents silence with this marker. Only edge annotations are
// removed; a quoted marker in spoken prose is not a command to rewrite text.
var whisperSilenceEdges = regexp.MustCompile(`^(?:\s*\[BLANK_AUDIO\])+|(?:\[BLANK_AUDIO\]\s*)+$`)

func whisperSpeechText(text string) string {
	return whisperSilenceEdges.ReplaceAllString(text, "")
}
