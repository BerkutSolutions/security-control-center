package backups

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	corebackups "berkut-scc/core/backups"
)

func (h *Handler) ImportBackup(w http.ResponseWriter, r *http.Request) {
	session := currentSession(r)
	limit := h.svc.UploadMaxBytes()
	if limit <= 0 {
		limit = 512 * 1024 * 1024
	}
	details := "resource_type=backup user_id=" + strconv.FormatInt(session.UserID, 10)
	corebackups.Log(h.audits, r.Context(), session.Username, corebackups.AuditImportRequested, "requested", details)

	r.Body = http.MaxBytesReader(w, r.Body, limit+(1<<20))
	reader, err := r.MultipartReader()
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			corebackups.Log(h.audits, r.Context(), session.Username, corebackups.AuditImportFailed, "failed", details+" reason_code="+corebackups.ErrorCodeUploadTooLarge)
			writeError(w, http.StatusRequestEntityTooLarge, corebackups.ErrorCodeUploadTooLarge, corebackups.ErrorKeyUploadTooLarge)
			return
		}
		corebackups.Log(h.audits, r.Context(), session.Username, corebackups.AuditImportFailed, "failed", details+" reason_code="+corebackups.ErrorCodeInvalidFormat)
		writeError(w, http.StatusBadRequest, corebackups.ErrorCodeInvalidFormat, corebackups.ErrorKeyInvalidFormat)
		return
	}

	var uploaded bool
	for {
		part, nextErr := reader.NextPart()
		if errors.Is(nextErr, http.ErrMissingFile) {
			break
		}
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			if strings.Contains(strings.ToLower(nextErr.Error()), "too large") {
				corebackups.Log(h.audits, r.Context(), session.Username, corebackups.AuditImportFailed, "failed", details+" reason_code="+corebackups.ErrorCodeUploadTooLarge)
				writeError(w, http.StatusRequestEntityTooLarge, corebackups.ErrorCodeUploadTooLarge, corebackups.ErrorKeyUploadTooLarge)
				return
			}
			break
		}
		if part.FormName() != "file" {
			_ = part.Close()
			continue
		}
		uploaded = true
		item, importErr := h.svc.ImportBackup(r.Context(), corebackups.ImportBackupRequest{
			File:          part,
			OriginalName:  part.FileName(),
			RequestedByID: session.UserID,
		})
		_ = part.Close()
		if importErr != nil {
			if de, ok := corebackups.AsDomainError(importErr); ok {
				corebackups.Log(h.audits, r.Context(), session.Username, corebackups.AuditImportFailed, "failed", details+" reason_code="+de.Code)
				writeError(w, importErrorHTTPStatus(de.Code), de.Code, de.I18NKey)
				return
			}
			corebackups.Log(h.audits, r.Context(), session.Username, corebackups.AuditImportFailed, "failed", details+" reason_code="+corebackups.ErrorCodeInternal)
			writeError(w, http.StatusInternalServerError, corebackups.ErrorCodeInternal, "common.serverError")
			return
		}
		size := int64(0)
		name := ""
		if item.SizeBytes != nil {
			size = *item.SizeBytes
		}
		if item.Filename != nil {
			name = *item.Filename
		}
		corebackups.Log(h.audits, r.Context(), session.Username, corebackups.AuditImportSuccess, "success", details+" size_bytes="+strconv.FormatInt(size, 10)+" filename="+name)
		writeJSON(w, http.StatusCreated, map[string]any{"item": item})
		return
	}

	if !uploaded {
		corebackups.Log(h.audits, r.Context(), session.Username, corebackups.AuditImportFailed, "failed", details+" reason_code="+corebackups.ErrorCodeInvalidFormat)
		writeError(w, http.StatusBadRequest, corebackups.ErrorCodeInvalidFormat, corebackups.ErrorKeyInvalidFormat)
		return
	}
	corebackups.Log(h.audits, r.Context(), session.Username, corebackups.AuditImportFailed, "failed", details+" reason_code="+corebackups.ErrorCodeInvalidFormat)
	writeError(w, http.StatusBadRequest, corebackups.ErrorCodeInvalidFormat, corebackups.ErrorKeyInvalidFormat)
}

func importErrorHTTPStatus(code string) int {
	switch code {
	case corebackups.ErrorCodeUploadTooLarge:
		return http.StatusRequestEntityTooLarge
	case corebackups.ErrorCodeInvalidRequest:
		return http.StatusBadRequest
	case corebackups.ErrorCodeConcurrent, corebackups.ErrorCodeFileBusy:
		return http.StatusConflict
	case corebackups.ErrorCodeInvalidFormat, corebackups.ErrorCodeInvalidEncKey, corebackups.ErrorCodeDecryptImport, corebackups.ErrorCodeChecksumImport:
		return http.StatusBadRequest
	case corebackups.ErrorCodeStorageMissing:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}
