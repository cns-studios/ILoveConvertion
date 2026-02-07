package processor

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"fileforge/internal/models"
)

func PDFCompress(ctx context.Context, inputPath, outputPath, tmpDir string, params models.JobParams) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	gsOutput := filepath.Join(tmpDir, "gs_intermediate.pdf")

	gsErr := runGhostscript(ctx, inputPath, gsOutput, params)

	qpdfInput := inputPath
	if gsErr != nil {
		log.Printf("[pdf] Ghostscript failed (will try qpdf on original): %v", gsErr)
	} else {
		qpdfInput = gsOutput
	}

	qpdfErr := runQPDF(ctx, qpdfInput, outputPath)
	if qpdfErr != nil {
		log.Printf("[pdf] qpdf failed: %v", qpdfErr)

		if gsErr == nil {
			return copyFile(gsOutput, outputPath)
		}

		bareErr := runQPDFMinimal(ctx, inputPath, outputPath)
		if bareErr != nil {
			return fmt.Errorf("PDF compression failed: ghostscript=%v, qpdf=%v", gsErr, qpdfErr)
		}
	}

	return nil
}

func runGhostscript(ctx context.Context, inputPath, outputPath string, params models.JobParams) error {
	dpi := params.ImageDPI
	if dpi <= 0 {
		dpi = 150
	}

	quality := params.ImageQuality
	if quality <= 0 {
		quality = 75
	}

	preset := pdfPreset(quality)

	monoDPI := min(dpi*2, 600)

	args := []string{
		"-sDEVICE=pdfwrite",
		"-dCompatibilityLevel=1.4",
		"-dSAFER",
		"-dNOPAUSE",
		"-dBATCH",
		"-dQUIET",

		fmt.Sprintf("-dPDFSETTINGS=%s", preset),

		fmt.Sprintf("-dColorImageResolution=%d", dpi),
		fmt.Sprintf("-dGrayImageResolution=%d", dpi),
		fmt.Sprintf("-dMonoImageResolution=%d", monoDPI),

		"-dDownsampleColorImages=true",
		"-dDownsampleGrayImages=true",
		"-dDownsampleMonoImages=true",

		"-dColorImageDownsampleType=/Bicubic",
		"-dGrayImageDownsampleType=/Bicubic",

		"-dCompressFonts=true",
		"-dEmbedAllFonts=true",
		"-dSubsetFonts=true",

		fmt.Sprintf("-sOutputFile=%s", outputPath),
		inputPath,
	}

	_, err := runCommand(ctx, "gs", args...)
	return err
}

func pdfPreset(quality int) string {
	switch {
	case quality <= 30:
		return "/screen"
	case quality <= 60:
		return "/ebook"
	case quality <= 85:
		return "/printer"
	default:
		return "/prepress"
	}
}

func runQPDF(ctx context.Context, inputPath, outputPath string) error {
	_, err := runCommand(ctx, "qpdf",
		"--linearize",
		"--object-streams=generate",
		"--compress-streams=y",
		"--recompress-flate",
		"--decode-level=generalized",
		inputPath,
		outputPath,
	)
	return err
}

func runQPDFMinimal(ctx context.Context, inputPath, outputPath string) error {
	_, err := runCommand(ctx, "qpdf",
		"--linearize",
		inputPath,
		outputPath,
	)
	return err
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy %s → %s: %w", src, dst, err)
	}

	return out.Sync()
}