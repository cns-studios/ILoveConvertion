package processor

import (
	"context"
	"fmt"

	"fileforge/internal/models"
)

func VideoCompress(ctx context.Context, inputPath, outputPath string, params models.JobParams) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	args := []string{
		"-hide_banner", "-loglevel", "error",
		"-i", inputPath,
	}

	switch params.OutputFormat {
	case "mp4":
		args = append(args, h264Args(params.Quality)...)
		args = append(args, aacAudioArgs()...)
		args = append(args, "-movflags", "+faststart")

	case "mkv":
		args = append(args, h264Args(params.Quality)...)
		args = append(args, aacAudioArgs()...)

	case "webm":
		args = append(args, vp9Args(params.Quality)...)
		args = append(args, opusAudioArgs()...)

	default:
		args = append(args, h264Args(params.Quality)...)
		args = append(args, aacAudioArgs()...)
	}

	args = append(args,
		"-sn",
		"-dn",
		"-map_metadata", "-1",
	)

	args = append(args, "-y", outputPath)

	_, err := runCommand(ctx, "ffmpeg", args...)
	if err != nil {
		return fmt.Errorf("video compress to %s (q=%d): %w",
			params.OutputFormat, params.Quality, err)
	}
	return nil
}

func h264Args(quality int) []string {
	crf := qualityToH264CRF(quality)
	return []string{
		"-c:v", "libx264",
		"-crf", fmt.Sprintf("%d", crf),
		"-preset", h264Preset(quality),
		"-pix_fmt", "yuv420p", // max player compatibility
		"-threads", "0",       // auto
	}
}

func qualityToH264CRF(quality int) int {
	return mapRange(quality, 1, 100, 45, 17)
}

func h264Preset(quality int) string {
	switch {
	case quality >= 80:
		return "slow"
	case quality >= 40:
		return "medium"
	default:
		return "faster"
	}
}


func vp9Args(quality int) []string {
	crf := qualityToVP9CRF(quality)
	return []string{
		"-c:v", "libvpx-vp9",
		"-crf", fmt.Sprintf("%d", crf),
		"-b:v", "0",     // required for CRF mode in VP9
		"-row-mt", "1",  // row-based multithreading (significant speedup)
		"-threads", "0", // auto
		"-pix_fmt", "yuv420p",
	}
}

func qualityToVP9CRF(quality int) int {
	return mapRange(quality, 1, 100, 50, 15)
}

func aacAudioArgs() []string {
	return []string{"-c:a", "aac", "-b:a", "128k", "-ac", "2"}
}

func opusAudioArgs() []string {
	return []string{"-c:a", "libopus", "-b:a", "128k", "-ac", "2"}
}