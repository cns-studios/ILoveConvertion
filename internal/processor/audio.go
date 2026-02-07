package processor

import (
	"context"
	"fmt"

	"fileforge/internal/models"
)

func AudioConvert(ctx context.Context, inputPath, outputPath string, params models.JobParams) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	args := []string{
		"-hide_banner", "-loglevel", "error",
		"-i", inputPath,
	}

	switch params.OutputFormat {
	case "mp3":
		args = append(args, "-c:a", "libmp3lame", "-q:a", "2") // VBR ~190kbps
	case "wav":
		args = append(args, "-c:a", "pcm_s16le")
	case "flac":
		args = append(args, "-c:a", "flac", "-compression_level", "8")
	case "ogg":
		args = append(args, "-c:a", "libvorbis", "-q:a", "5") // ~160kbps
	case "opus":
		args = append(args, "-c:a", "libopus", "-b:a", "128k")
	case "aac":
		args = append(args, "-c:a", "aac", "-b:a", "192k")
	case "m4a":
		args = append(args, "-c:a", "aac", "-b:a", "192k")
	case "aiff":
		args = append(args, "-c:a", "pcm_s16be")
	default:
		args = append(args, "-c:a", "copy")
	}

	args = append(args, "-vn")
	args = append(args, "-y", outputPath)

	_, err := runCommand(ctx, "ffmpeg", args...)
	if err != nil {
		return fmt.Errorf("audio convert to %s: %w", params.OutputFormat, err)
	}
	return nil
}

func AudioCompress(ctx context.Context, inputPath, outputPath string, params models.JobParams) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	args := []string{
		"-hide_banner", "-loglevel", "error",
		"-i", inputPath,
	}

	if params.Lossless {
		args = append(args, losslessAudioArgs(params.OutputFormat)...)
	} else {
		args = append(args, lossyAudioArgs(params.OutputFormat, params.Quality)...)
	}

	args = append(args, "-vn")
	args = append(args, "-y", outputPath)

	_, err := runCommand(ctx, "ffmpeg", args...)
	if err != nil {
		return fmt.Errorf("audio compress (%s, q=%d, lossless=%v): %w",
			params.OutputFormat, params.Quality, params.Lossless, err)
	}
	return nil
}

func losslessAudioArgs(format string) []string {
	switch format {
	case "flac":
		return []string{"-c:a", "flac", "-compression_level", "12"}
	case "wav":
		return []string{"-c:a", "pcm_s16le"}
	case "aiff":
		return []string{"-c:a", "pcm_s16be"}
	default:
		return lossyAudioArgs(format, 100)
	}
}

func lossyAudioArgs(format string, quality int) []string {
	switch format {
	case "mp3":
		br := mapRange(quality, 1, 100, 32, 320)
		return []string{"-c:a", "libmp3lame", "-b:a", fmt.Sprintf("%dk", br)}

	case "ogg":
		q := mapRange(quality, 1, 100, 0, 10)
		return []string{"-c:a", "libvorbis", "-q:a", fmt.Sprintf("%d", q)}

	case "opus":
		br := mapRange(quality, 1, 100, 16, 256)
		return []string{"-c:a", "libopus", "-b:a", fmt.Sprintf("%dk", br)}

	case "aac", "m4a":
		br := mapRange(quality, 1, 100, 32, 256)
		return []string{"-c:a", "aac", "-b:a", fmt.Sprintf("%dk", br)}

	case "flac":
		level := mapRange(quality, 1, 100, 12, 0)
		return []string{"-c:a", "flac", "-compression_level", fmt.Sprintf("%d", level)}

	case "wav":
		return []string{"-c:a", "pcm_s16le"}

	case "aiff":
		return []string{"-c:a", "pcm_s16be"}

	case "wma":
		br := mapRange(quality, 1, 100, 32, 192)
		return []string{"-c:a", "wmav2", "-b:a", fmt.Sprintf("%dk", br)}

	default:
		return []string{"-c:a", "copy"}
	}
}

func mapRange(value, inMin, inMax, outMin, outMax int) int {
	if value <= inMin {
		return outMin
	}
	if value >= inMax {
		return outMax
	}
	return outMin + (value-inMin)*(outMax-outMin)/(inMax-inMin)
}