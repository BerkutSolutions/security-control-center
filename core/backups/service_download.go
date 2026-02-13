package backups

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

func (s *Service) DownloadArtifact(ctx context.Context, id int64) (*DownloadArtifact, error) {
	if s == nil || s.repo == nil {
		return nil, NewDomainError(ErrorCodeNotFound, ErrorKeyNotFound)
	}
	artifact, err := s.repo.GetArtifact(ctx, id)
	if err != nil {
		if err == ErrNotFound {
			return nil, NewDomainError(ErrorCodeNotFound, ErrorKeyNotFound)
		}
		return nil, err
	}
	if artifact.Status != StatusSuccess {
		return nil, NewDomainError(ErrorCodeNotReady, ErrorKeyNotReady)
	}
	if artifact.StoragePath == nil || strings.TrimSpace(*artifact.StoragePath) == "" {
		return nil, NewDomainError(ErrorCodeFileMissing, ErrorKeyFileMissing)
	}
	cleanStorage, ok := s.cleanStoragePath(*artifact.StoragePath)
	if !ok {
		return nil, NewDomainError(ErrorCodeFileMissing, ErrorKeyFileMissing)
	}
	if err := s.beginDownload(id); err != nil {
		return nil, err
	}
	release := true
	defer func() {
		if release {
			s.endDownload(id)
		}
	}()
	info, err := os.Stat(cleanStorage)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, NewDomainError(ErrorCodeFileMissing, ErrorKeyFileMissing)
		}
		return nil, NewDomainError(ErrorCodeCannotOpenFile, ErrorKeyCannotOpenFile)
	}
	if info.IsDir() {
		return nil, NewDomainError(ErrorCodeCannotOpenFile, ErrorKeyCannotOpenFile)
	}
	f, err := os.Open(cleanStorage)
	if err != nil {
		return nil, NewDomainError(ErrorCodeCannotOpenFile, ErrorKeyCannotOpenFile)
	}
	filename := "backup_" + strings.TrimSpace(strings.ReplaceAll(strings.TrimPrefix(filepath.Base(cleanStorage), ".."), string(os.PathSeparator), "_"))
	if artifact.Filename != nil && strings.TrimSpace(*artifact.Filename) != "" {
		filename = filepath.Base(strings.TrimSpace(*artifact.Filename))
	}
	if !strings.HasSuffix(strings.ToLower(filename), ".bscc") {
		filename = filename + ".bscc"
	}
	release = false
	return &DownloadArtifact{
		ID:       artifact.ID,
		Filename: filename,
		Size:     info.Size(),
		ModTime:  info.ModTime().UTC(),
		Reader:   trackedReadCloser{ReadSeekCloser: f, done: func() { s.endDownload(id) }},
	}, nil
}

type trackedReadCloser struct {
	ReadSeekCloser
	done func()
}

func (t trackedReadCloser) Close() error {
	err := t.ReadSeekCloser.Close()
	if t.done != nil {
		t.done()
	}
	return err
}
