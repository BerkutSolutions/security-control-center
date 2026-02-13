package backups

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	backupcrypto "berkut-scc/core/backups/crypto"
	"berkut-scc/core/backups/format"
	"berkut-scc/core/backups/restore"
)

type importVerifyResult struct {
	Manifest  format.Manifest
	Checksums format.Checksums
}

func (s *Service) ImportBackup(ctx context.Context, req ImportBackupRequest) (*BackupArtifact, error) {
	if s == nil || s.repo == nil || s.cfg == nil || req.File == nil {
		return nil, NewDomainError(ErrorCodeStorageMissing, ErrorKeyStorageMissing)
	}
	defer req.File.Close()
	if err := s.beginPipeline("backup"); err != nil {
		return nil, err
	}
	defer s.endPipeline("backup")
	if s.IsMaintenanceMode(ctx) {
		return nil, NewDomainError(ErrorCodeConcurrent, ErrorKeyConcurrent)
	}
	backupRunning, err := s.repo.HasRunningBackupRun(ctx)
	if err != nil {
		return nil, err
	}
	if backupRunning {
		return nil, NewDomainError(ErrorCodeConcurrent, ErrorKeyConcurrent)
	}
	restoreRunning, err := s.repo.HasRunningRestoreRun(ctx)
	if err != nil {
		return nil, err
	}
	if restoreRunning {
		return nil, NewDomainError(ErrorCodeConcurrent, ErrorKeyConcurrent)
	}

	baseDir := strings.TrimSpace(s.cfg.Backups.Path)
	if baseDir == "" {
		return nil, NewDomainError(ErrorCodeStorageMissing, ErrorKeyStorageMissing)
	}
	importDir := filepath.Join(baseDir, "imports")
	if err := os.MkdirAll(importDir, 0o700); err != nil {
		return nil, NewDomainError(ErrorCodeStorageMissing, ErrorKeyStorageMissing)
	}
	tmpDir := filepath.Join(baseDir, "tmp")
	if err := os.MkdirAll(tmpDir, 0o700); err != nil {
		return nil, NewDomainError(ErrorCodeStorageMissing, ErrorKeyStorageMissing)
	}

	maxSize := s.cfg.Backups.UploadMaxBytes
	if maxSize <= 0 {
		maxSize = 512 * 1024 * 1024
	}
	filename := "import_" + time.Now().UTC().Format("2006-01-02_15-04-05") + "_" + randomSuffix(10) + ".bscc"
	tmpPath := filepath.Join(tmpDir, filename+".part")
	size, writeErr := writeLimitedFile(tmpPath, req.File, maxSize)
	if writeErr != nil {
		_ = os.Remove(tmpPath)
		return nil, writeErr
	}

	verify, verr := s.verifyImportedFile(tmpPath)
	if verr != nil {
		_ = os.Remove(tmpPath)
		return nil, verr
	}
	path := filepath.Join(importDir, filename)
	if moveErr := os.Rename(tmpPath, path); moveErr != nil {
		_ = os.Remove(tmpPath)
		return nil, NewDomainError(ErrorCodeStorageMissing, ErrorKeyStorageMissing)
	}

	checksum, _, cerr := fileSHA256(path)
	if cerr != nil {
		_ = os.Remove(path)
		return nil, NewDomainError(ErrorCodeStorageMissing, ErrorKeyStorageMissing)
	}

	origin := sanitizeOriginFilename(req.OriginalName)
	meta, _ := json.Marshal(map[string]any{
		"format_version":   verify.Manifest.FormatVersion,
		"created_at":       verify.Manifest.CreatedAt,
		"app_version":      verify.Manifest.AppVersion,
		"db_engine":        verify.Manifest.DBEngine,
		"goose_db_version": verify.Manifest.GooseDBVersion,
		"includes_files":   verify.Manifest.IncludesFiles,
		"checksums":        verify.Checksums,
		"source":           ArtifactSourceImported,
		"origin_filename":  origin,
	})
	artifact := &BackupArtifact{
		Source:      ArtifactSourceImported,
		CreatedByID: nullableID(req.RequestedByID),
		OriginFile:  nullableStringValue(origin),
		Status:      StatusSuccess,
		SizeBytes:   &size,
		Checksum:    &checksum,
		Filename:    &filename,
		StoragePath: &path,
		MetaJSON:    meta,
	}
	return s.repo.CreateArtifact(ctx, artifact)
}

func (s *Service) verifyImportedFile(path string) (*importVerifyResult, error) {
	header, err := readBSCCHeader(path)
	if err != nil {
		return nil, NewDomainError(ErrorCodeInvalidFormat, ErrorKeyInvalidFormat)
	}
	if header.FormatVersion != format.FileFormatVersion || header.CryptoVersion != format.CryptoFormatVersion {
		return nil, NewDomainError(ErrorCodeInvalidFormat, ErrorKeyInvalidFormat)
	}

	key := strings.TrimSpace(s.cfg.Backups.EncryptionKey)
	cipher, err := backupcrypto.NewFileCipher(key, 1024*1024)
	if err != nil {
		return nil, NewDomainError(ErrorCodeInvalidEncKey, ErrorKeyInvalidEncKey)
	}

	tmpDir, err := os.MkdirTemp("", "backup-import-verify-*")
	if err != nil {
		return nil, NewDomainError(ErrorCodeStorageMissing, ErrorKeyStorageMissing)
	}
	defer os.RemoveAll(tmpDir)

	decrypted := filepath.Join(tmpDir, "payload.tar")
	if err := cipher.DecryptFile(path, decrypted); err != nil {
		return nil, NewDomainError(ErrorCodeDecryptImport, ErrorKeyDecryptImport)
	}
	payload, err := restore.ExtractPayload(decrypted, tmpDir)
	if err != nil {
		return nil, NewDomainError(ErrorCodeInvalidFormat, ErrorKeyInvalidFormat)
	}
	if payload.ManifestSHA != payload.Checksums.ManifestSHA256 || payload.DumpSHA != payload.Checksums.DumpSHA256 {
		return nil, NewDomainError(ErrorCodeChecksumImport, ErrorKeyChecksumImport)
	}
	if payload.Manifest.FormatVersion != "1" || payload.Manifest.DBEngine != "postgres" {
		return nil, NewDomainError(ErrorCodeInvalidFormat, ErrorKeyInvalidFormat)
	}
	return &importVerifyResult{Manifest: payload.Manifest, Checksums: payload.Checksums}, nil
}

func readBSCCHeader(path string) (format.Header, error) {
	f, err := os.Open(path)
	if err != nil {
		return format.Header{}, err
	}
	defer f.Close()
	return format.DecodeHeader(f)
}

func writeLimitedFile(path string, src io.Reader, max int64) (int64, error) {
	out, err := os.Create(path)
	if err != nil {
		return 0, NewDomainError(ErrorCodeStorageMissing, ErrorKeyStorageMissing)
	}
	defer out.Close()

	limited := io.LimitReader(src, max+1)
	written, err := io.CopyBuffer(out, limited, make([]byte, 32*1024))
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return 0, NewDomainError(ErrorCodeUploadTooLarge, ErrorKeyUploadTooLarge)
		}
		return 0, NewDomainError(ErrorCodeStorageMissing, ErrorKeyStorageMissing)
	}
	if written > max {
		return 0, NewDomainError(ErrorCodeUploadTooLarge, ErrorKeyUploadTooLarge)
	}
	if written == 0 {
		return 0, NewDomainError(ErrorCodeInvalidFormat, ErrorKeyInvalidFormat)
	}
	return written, nil
}

func sanitizeOriginFilename(name string) string {
	raw := strings.TrimSpace(name)
	if raw == "" {
		return "uploaded.bscc"
	}
	raw = filepath.Base(raw)
	raw = strings.ReplaceAll(raw, "\\", "_")
	raw = strings.ReplaceAll(raw, "/", "_")
	raw = strings.ReplaceAll(raw, "\"", "_")
	if !strings.HasSuffix(strings.ToLower(raw), ".bscc") {
		raw += ".bscc"
	}
	return raw
}

func nullableID(v int64) *int64 {
	if v <= 0 {
		return nil
	}
	out := v
	return &out
}

func nullableStringValue(v string) *string {
	s := strings.TrimSpace(v)
	if s == "" {
		return nil
	}
	return &s
}

func randomSuffix(n int) string {
	if n <= 0 {
		n = 8
	}
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "rnd"
	}
	out := hex.EncodeToString(buf)
	if len(out) > n {
		return out[:n]
	}
	return out
}
