package processor

import (
	"context"
	"fmt"

	"fileforge/internal/models"

	"github.com/h2non/bimg"
)

func ImageConvert(ctx context.Context, inputPath, outputPath string, params models.JobParams) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	imgType, supported := bimgType(params.OutputFormat)

	if !supported {
		_, err := runCommand(ctx, "ffmpeg",
			"-hide_banner", "-loglevel", "error",
			"-i", inputPath,
			"-y", outputPath,
		)
		if err != nil {
			return fmt.Errorf("image convert (ffmpeg fallback): %w", err)
		}
		return nil
	}

	buf, err := bimg.Read(inputPath)
	if err != nil {
		return fmt.Errorf("read image: %w", err)
	}

	opts := bimg.Options{
		Type: imgType,
	}

	if imgType == bimg.JPEG {
		opts.Quality = 95
	}

	out, err := bimg.NewImage(buf).Process(opts)
	if err != nil {
		return fmt.Errorf("convert image to %s: %w", params.OutputFormat, err)
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if err := bimg.Write(outputPath, out); err != nil {
		return fmt.Errorf("write output: %w", err)
	}

	return nil
}

func ImageCompress(ctx context.Context, inputPath, outputPath string, params models.JobParams) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if params.OutputFormat == "png" {
		return compressPNG(ctx, inputPath, outputPath, params.Quality, params.Lossless)
	}

	imgType, supported := bimgType(params.OutputFormat)
	if !supported {
		return fmt.Errorf("unsupported format for compression: %s", params.OutputFormat)
	}

	buf, err := bimg.Read(inputPath)
	if err != nil {
		return fmt.Errorf("read image: %w", err)
	}

	opts := bimg.Options{
		Type:    imgType,
		Quality: params.Quality,
	}

	switch params.OutputFormat {
	case "webp":
		if params.Lossless {
			opts.Lossless = true
			opts.Quality = 0
		}

	case "avif":
		if params.Lossless {
			opts.Lossless = true
		}

	case "jpeg":
		opts.StripMetadata = true

	case "tiff":
		if params.Lossless {
			opts.Quality = 100
		}
	}

	out, err := bimg.NewImage(buf).Process(opts)
	if err != nil {
		return fmt.Errorf("compress %s: %w", params.OutputFormat, err)
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if err := bimg.Write(outputPath, out); err != nil {
		return fmt.Errorf("write output: %w", err)
	}

	return nil
}

func compressPNG(ctx context.Context, inputPath, outputPath string, quality int, lossless bool) error {
	if lossless {
		return compressPNGLossless(ctx, inputPath, outputPath)
	}
	return compressPNGLossy(ctx, inputPath, outputPath, quality)
}

func compressPNGLossless(ctx context.Context, inputPath, outputPath string) error {
	buf, err := bimg.Read(inputPath)
	if err != nil {
		return fmt.Errorf("read PNG: %w", err)
	}

	out, err := bimg.NewImage(buf).Process(bimg.Options{
		Type:          bimg.PNG,
		StripMetadata: true,
	})
	if err != nil {
		return fmt.Errorf("lossless PNG optimize: %w", err)
	}

	return bimg.Write(outputPath, out)
}

func compressPNGLossy(ctx context.Context, inputPath, outputPath string, quality int) error {
	minQ := quality - 20
	if minQ < 0 {
		minQ = 0
	}

	_, err := runCommand(ctx, "pngquant",
		"--quality", fmt.Sprintf("%d-%d", minQ, quality),
		"--speed", "3",
		"--force",
		"--output", outputPath,
		"--", inputPath,
	)

	if err != nil {
		return compressPNGLossless(ctx, inputPath, outputPath)
	}

	return nil
}