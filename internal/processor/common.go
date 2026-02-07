package processor

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"

	"github.com/h2non/bimg"
)

func runCommand(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("operation timed out")
		}
		if ctx.Err() == context.Canceled {
			return "", ctx.Err()
		}

		errOutput := stderr.String()
		if errOutput == "" {
			errOutput = stdout.String()
		}
		if len(errOutput) > 500 {
			errOutput = errOutput[:500] + "…"
		}
		return "", fmt.Errorf("%s failed: %v — %s", name, err, errOutput)
	}

	return stdout.String(), nil
}

func runCommandPipeInput(ctx context.Context, stdinData []byte, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = bytes.NewReader(stdinData)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		errMsg := stderr.String()
		if len(errMsg) > 500 {
			errMsg = errMsg[:500] + "…"
		}
		return nil, fmt.Errorf("%s failed: %v — %s", name, err, errMsg)
	}

	return stdout.Bytes(), nil
}

func bimgType(format string) (bimg.ImageType, bool) {
	switch format {
	case "jpeg", "jpg":
		return bimg.JPEG, true
	case "png":
		return bimg.PNG, true
	case "webp":
		return bimg.WEBP, true
	case "tiff", "tif":
		return bimg.TIFF, true
	case "gif":
		return bimg.GIF, true
	case "avif":
		return bimg.AVIF, true
	case "heif", "heic":
		return bimg.HEIF, true
	default:
		return 0, false
	}
}