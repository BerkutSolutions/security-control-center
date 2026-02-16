package backups

import "errors"

var (
	ErrNotFound       = errors.New("not found")
	ErrNotImplemented = errors.New("not implemented")
)

type DomainError struct {
	Code    string
	I18NKey string
}

func (e *DomainError) Error() string {
	if e == nil {
		return ""
	}
	if e.Code != "" {
		return e.Code
	}
	return "backups.error"
}

func AsDomainError(err error) (*DomainError, bool) {
	if err == nil {
		return nil, false
	}
	var de *DomainError
	if errors.As(err, &de) {
		return de, true
	}
	return nil, false
}

const (
	ErrorKeyNotImplemented = "backups.error.notImplemented"
	ErrorKeyForbidden      = "backups.error.forbidden"
	ErrorKeyNotFound       = "backups.error.notFound"
	ErrorKeyNotReady       = "backups.error.notReady"
	ErrorKeyFileMissing    = "backups.error.fileMissing"
	ErrorKeyCannotOpenFile = "backups.error.cannotOpenFile"
	ErrorKeyStreamFailed   = "backups.error.streamFailed"
	ErrorKeyPGDumpFailed   = "backups.error.pgDumpFailed"
	ErrorKeyEncryptFailed  = "backups.error.encryptFailed"
	ErrorKeyStorageMissing = "backups.error.storageUnavailable"
	ErrorKeyInvalidEncKey  = "backups.error.invalidEncryptionKey"
	ErrorKeyInvalidFormat  = "backups.error.invalidFormat"
	ErrorKeyUploadTooLarge = "backups.error.uploadTooLarge"
	ErrorKeyDecryptImport  = "backups.error.decryptFailed"
	ErrorKeyChecksumImport = "backups.error.checksumMismatch"
	ErrorKeyPermission     = "backups.error.permissionDenied"
	ErrorKeyInvalidRequest = "backups.error.invalidRequest"
	ErrorKeyConcurrent     = "backups.error.concurrentOperation"
	ErrorKeyFileBusy       = "backups.error.fileBusy"
	ErrorKeyInternal       = "backups.error.internal"
	ErrorKeyDecryptFailed  = "backups.restore.error.decryptFailed"
	ErrorKeyChecksumFailed = "backups.restore.error.checksumMismatch"
	ErrorKeyIncompatible   = "backups.restore.error.incompatibleVersion"
	ErrorKeyPgRestore      = "backups.restore.error.pgRestoreFailed"
	ErrorKeyMaintenance    = "backups.restore.error.maintenanceFailed"
	ErrorKeyInvalidPlan    = "backups.plan.error.invalid"
)

const (
	ErrorCodeNotImplemented = "backups.not_implemented"
	ErrorCodeForbidden      = "backups.forbidden"
	ErrorCodeNotFound       = "backups.not_found"
	ErrorCodeNotReady       = "backups.not_ready"
	ErrorCodeFileMissing    = "backups.file_missing"
	ErrorCodeCannotOpenFile = "backups.cannot_open_file"
	ErrorCodeStreamFailed   = "backups.stream_failed"
	ErrorCodePGDumpFailed   = "backups.pg_dump_failed"
	ErrorCodeEncryptFailed  = "backups.encrypt_failed"
	ErrorCodeStorageMissing = "backups.storage_unavailable"
	ErrorCodeInvalidEncKey  = "backups.invalid_encryption_key"
	ErrorCodeInvalidFormat  = "backups.invalid_format"
	ErrorCodeUploadTooLarge = "backups.upload_too_large"
	ErrorCodeDecryptImport  = "backups.decrypt_failed"
	ErrorCodeChecksumImport = "backups.checksum_mismatch"
	ErrorCodeInvalidRequest = "backups.invalid_request"
	ErrorCodeConcurrent     = "backups.concurrent_operation"
	ErrorCodeFileBusy       = "backups.file_busy"
	ErrorCodeDecryptFailed  = "backups.restore.decrypt_failed"
	ErrorCodeChecksumFailed = "backups.restore.checksum_mismatch"
	ErrorCodeIncompatible   = "backups.restore.incompatible_version"
	ErrorCodePgRestore      = "backups.restore.pg_restore_failed"
	ErrorCodeMaintenance    = "backups.restore.maintenance_failed"
	ErrorCodeInvalidPlan    = "backups.plan.invalid"
	ErrorCodeInternal       = "backups.internal"
)

func NewDomainError(code, i18nKey string) *DomainError {
	return &DomainError{Code: code, I18NKey: i18nKey}
}
