package processor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"fileforge/internal/models"
)

func ImageRemoveBG(ctx context.Context, inputPath, outputPath, rembgURL string, params models.JobParams) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	outFmt := params.OutputFormat
	if outFmt == "" {
		outFmt = "png"
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filepath.Base(inputPath))
	if err != nil {
		return fmt.Errorf("create form file: %w", err)
	}

	inputFile, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("open input: %w", err)
	}
	defer inputFile.Close()

	if _, err := io.Copy(part, inputFile); err != nil {
		return fmt.Errorf("copy to multipart: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("close multipart writer: %w", err)
	}

	url := fmt.Sprintf("%s/remove-bg?format=%s", rembgURL, outFmt)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("rembg request cancelled/timed out: %w", ctx.Err())
		}
		return fmt.Errorf("rembg request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("rembg service error (HTTP %d): %s", resp.StatusCode, string(errBody))
	}

	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}
	defer outFile.Close()

	written, err := io.Copy(outFile, resp.Body)
	if err != nil {
		return fmt.Errorf("write output: %w", err)
	}

	if written == 0 {
		return fmt.Errorf("rembg returned empty response")
	}

	return outFile.Sync()
}