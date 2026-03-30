package audio

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/hajimehoshi/go-mp3"
)

type WAVHeader struct {
	ChunkID       [4]byte
	ChunkSize     uint32
	Format        [4]byte
	Subchunk1ID   [4]byte
	Subchunk1Size uint32
	AudioFormat   uint16
	NumChannels   uint16
	SampleRate    uint32
	ByteRate      uint32
	BlockAlign    uint16
	BitsPerSample uint16
	Subchunk2ID   [4]byte
	Subchunk2Size uint32
}

func WAVDurationFromBinary(data []byte) (float64, error) {
	const wavHeaderSize = 44
	if len(data) < wavHeaderSize {
		return 0, fmt.Errorf("wav data too short: %d", len(data))
	}

	buf := bytes.NewReader(data)
	var header WAVHeader
	if err := binary.Read(buf, binary.LittleEndian, &header); err != nil {
		return 0, fmt.Errorf("read wav header: %w", err)
	}
	if string(header.ChunkID[:]) != "RIFF" || string(header.Format[:]) != "WAVE" {
		return 0, fmt.Errorf("invalid wav header")
	}
	if header.AudioFormat != 1 {
		return 0, fmt.Errorf("unsupported wav audio format: %d", header.AudioFormat)
	}

	dataLen := len(data) - wavHeaderSize
	if dataLen == 0 {
		return 0, fmt.Errorf("empty wav payload")
	}
	sampleBytes := int64(header.NumChannels) * int64(header.BitsPerSample/8)
	if sampleBytes == 0 {
		return 0, fmt.Errorf("invalid wav params channels=%d bits=%d", header.NumChannels, header.BitsPerSample)
	}
	if header.SampleRate == 0 {
		return 0, fmt.Errorf("wav sample rate is zero")
	}
	totalSamples := int64(dataLen) / sampleBytes
	return float64(totalSamples) / float64(header.SampleRate), nil
}

func MP3DurationFromBinary(data []byte) (float64, error) {
	if len(data) == 0 {
		return 0, fmt.Errorf("empty mp3 payload")
	}
	decoder, err := mp3.NewDecoder(bytes.NewReader(data))
	if err != nil {
		return 0, fmt.Errorf("decode mp3: %w", err)
	}
	sampleRate := decoder.SampleRate()
	totalBytes := decoder.Length()
	if sampleRate == 0 {
		return 0, fmt.Errorf("mp3 sample rate is zero")
	}
	if totalBytes <= 0 {
		return 0, fmt.Errorf("invalid mp3 byte length: %d", totalBytes)
	}
	const bytesPerSample = 4
	totalSamples := totalBytes / int64(bytesPerSample)
	return float64(totalSamples) / float64(sampleRate), nil
}

type Format string

const (
	FormatWAV     Format = "wav"
	FormatMP3     Format = "mp3"
	FormatUnknown Format = "unknown"
)

func DetectFormat(data []byte) Format {
	if len(data) < 4 {
		return FormatUnknown
	}
	if string(data[:4]) == "RIFF" && len(data) >= 12 && string(data[8:12]) == "WAVE" {
		return FormatWAV
	}
	if len(data) >= 3 && string(data[:3]) == "ID3" {
		return FormatMP3
	}
	if len(data) >= 2 {
		firstTwo := binary.BigEndian.Uint16(data[:2])
		if (firstTwo & 0xFFE0) == 0xFFE0 {
			return FormatMP3
		}
	}
	return FormatUnknown
}

func DurationFromBytes(data []byte) (float64, error) {
	switch DetectFormat(data) {
	case FormatWAV:
		return WAVDurationFromBinary(data)
	case FormatMP3:
		return MP3DurationFromBinary(data)
	default:
		return 0, fmt.Errorf("unknown audio format")
	}
}

func EstimatedTokensFromDuration(durationSeconds float64) int {
	if durationSeconds <= 0 {
		return 0
	}
	const unitTokenNum = 1250
	const unitSecondNum = 60
	tokensPerSecond := float64(unitTokenNum) / float64(unitSecondNum)
	return int(math.Ceil(tokensPerSecond * durationSeconds))
}

func EstimatedTokensFromBytes(data []byte) (int, error) {
	duration, err := DurationFromBytes(data)
	if err != nil {
		return 0, err
	}
	return EstimatedTokensFromDuration(duration), nil
}

func DurationFromBytesOrDefault(data []byte, defaultSeconds float64) float64 {
	duration, err := DurationFromBytes(data)
	if err != nil {
		return defaultSeconds
	}
	return duration
}

func EstimatedTokensFromBytesOrDefault(data []byte, defaultSeconds float64) int {
	return EstimatedTokensFromDuration(DurationFromBytesOrDefault(data, defaultSeconds))
}

func SumDurationsFromPayloads(payloads [][]byte) (float64, error) {
	total := 0.0
	for _, payload := range payloads {
		duration, err := DurationFromBytes(payload)
		if err != nil {
			return 0, err
		}
		total += duration
	}
	return total, nil
}

func SumEstimatedTokensFromPayloads(payloads [][]byte) (int, error) {
	total := 0
	for _, payload := range payloads {
		tokens, err := EstimatedTokensFromBytes(payload)
		if err != nil {
			return 0, err
		}
		total += tokens
	}
	return total, nil
}

func SumDurationsFromPayloadsOrDefault(payloads [][]byte, defaultSeconds float64) float64 {
	total := 0.0
	for _, payload := range payloads {
		total += DurationFromBytesOrDefault(payload, defaultSeconds)
	}
	return total
}

func SumEstimatedTokensFromPayloadsOrDefault(payloads [][]byte, defaultSeconds float64) int {
	total := 0
	for _, payload := range payloads {
		total += EstimatedTokensFromBytesOrDefault(payload, defaultSeconds)
	}
	return total
}
