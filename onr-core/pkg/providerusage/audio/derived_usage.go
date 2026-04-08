package audio

const speechDurationDerivedPath = "audio_duration_seconds"

// BuildSpeechDerivedUsage derives common audio.speech usage fields from raw
// audio bytes. When decoding fails and fallbackSeconds > 0, the fallback value
// is emitted instead so runtimes can preserve legacy behavior where needed.
func BuildSpeechDerivedUsage(data []byte, fallbackSeconds float64) map[string]any {
	if len(data) == 0 {
		return nil
	}
	duration, err := DurationFromBytes(data)
	if err != nil && fallbackSeconds > 0 {
		duration = fallbackSeconds
	}
	if err != nil && fallbackSeconds <= 0 {
		return nil
	}
	if duration <= 0 {
		return nil
	}
	return map[string]any{
		speechDurationDerivedPath: duration,
	}
}
