package storage

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

type Storage struct {
	basePath   string
	inputsDir  string
	outputsDir string
}

func New(basePath string) (*Storage, error) {
	s := &Storage{
		basePath:   basePath,
		inputsDir:  filepath.Join(basePath, "inputs"),
		outputsDir: filepath.Join(basePath, "outputs"),
	}

	for _, dir := range []string{s.inputsDir, s.outputsDir} {
		if err := os.MkdirAll(dir, 0750); err != nil {
			return nil, fmt.Errorf("create storage dir %s: %w", dir, err)
		}
	}

	return s, nil
}

func (s *Storage) InputPath(jobID string) string {
	return filepath.Join(s.inputsDir, jobID)
}

func (s *Storage) OutputPath(jobID string) string {
	return filepath.Join(s.outputsDir, jobID)
}

func (s *Storage) InputExists(jobID string) bool {
	_, err := os.Stat(s.InputPath(jobID))
	return err == nil
}

func (s *Storage) OutputExists(jobID string) bool {
	_, err := os.Stat(s.OutputPath(jobID))
	return err == nil
}

func (s *Storage) DeleteJobFiles(jobID string) {
	os.Remove(s.InputPath(jobID))
	os.Remove(s.OutputPath(jobID))
}

func (s *Storage) CreateInput(jobID string) (*os.File, error) {
	p := s.InputPath(jobID)
	f, err := os.Create(p)
	if err != nil {
		return nil, fmt.Errorf("create input %s: %w", p, err)
	}
	return f, nil
}

func (s *Storage) OpenInput(jobID string) (*os.File, error) {
	p := s.InputPath(jobID)
	f, err := os.Open(p)
	if err != nil {
		return nil, fmt.Errorf("open input %s: %w", p, err)
	}
	return f, nil
}

func (s *Storage) OpenOutput(jobID string) (*os.File, error) {
	p := s.OutputPath(jobID)
	f, err := os.Open(p)
	if err != nil {
		return nil, fmt.Errorf("open output %s: %w", p, err)
	}
	return f, nil
}

func (s *Storage) FileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func (s *Storage) UsedBytes() (int64, error) {
	var total int64

	err := filepath.WalkDir(s.basePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return nil
			}
			total += info.Size()
		}
		return nil
	})

	return total, err
}

func (s *Storage) UsedMB() int64 {
	bytes, _ := s.UsedBytes()
	return bytes / (1024 * 1024)
}