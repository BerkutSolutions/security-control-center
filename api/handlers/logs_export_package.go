package handlers

import (
	"archive/zip"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"berkut-scc/core/store"
)

func (h *LogsHandler) ExportPackage(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.audits == nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	filter := parseLogFilter(r)
	if filter.Limit <= 0 || filter.Limit > 10000 {
		filter.Limit = 10000
	}
	items, err := h.audits.ListIntegrityFiltered(r.Context(), filter.Since, filter.To, filter.Limit*3)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}

	filtered := make([]store.AuditIntegrityRecord, 0, min(filter.Limit, len(items)))
	for i := range items {
		if !matchLogFilter(items[i].AuditRecord, filter) {
			continue
		}
		filtered = append(filtered, items[i])
		if len(filtered) >= filter.Limit {
			break
		}
	}

	chainOK, signatureOK := verifyAuditChain(filtered)
	eventsJSONL := renderAuditJSONL(filtered)
	eventsHash := sumHex(eventsJSONL)

	manifest := map[string]any{
		"generated_at":         time.Now().UTC(),
		"filter":               filter,
		"items":                len(filtered),
		"chain_ok":             chainOK,
		"signature_ok":         signatureOK,
		"events_jsonl_sha256":  eventsHash,
		"format_version":       "1",
		"append_only_expected": true,
	}
	manifestBytes, _ := json.MarshalIndent(manifest, "", "  ")
	manifestHash := sumHex(manifestBytes)

	signature := ""
	signingKey := strings.TrimSpace(os.Getenv("BERKUT_AUDIT_SIGNING_KEY"))
	if signingKey != "" {
		signature = signWithHMACSHA256(manifestHash, []byte(signingKey))
	}

	checksums := []byte(strings.Join([]string{
		eventsHash + "  audit_events.jsonl",
		manifestHash + "  manifest.json",
		sumHex([]byte(signature)) + "  manifest.signature",
	}, "\n") + "\n")

	filename := "audit_package_" + time.Now().UTC().Format("20060102_150405") + ".zip"
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	w.WriteHeader(http.StatusOK)

	zw := zip.NewWriter(w)
	if err := writeZipEntry(zw, "audit_events.jsonl", eventsJSONL); err != nil {
		_ = zw.Close()
		return
	}
	if err := writeZipEntry(zw, "manifest.json", manifestBytes); err != nil {
		_ = zw.Close()
		return
	}
	if err := writeZipEntry(zw, "manifest.signature", []byte(signature+"\n")); err != nil {
		_ = zw.Close()
		return
	}
	if err := writeZipEntry(zw, "checksums.sha256", checksums); err != nil {
		_ = zw.Close()
		return
	}
	_ = zw.Close()
}

func writeZipEntry(zw *zip.Writer, name string, data []byte) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func renderAuditJSONL(items []store.AuditIntegrityRecord) []byte {
	var b strings.Builder
	for i := range items {
		line, _ := json.Marshal(map[string]any{
			"id":         items[i].ID,
			"created_at": items[i].CreatedAt.UTC().Format(time.RFC3339Nano),
			"username":   strings.TrimSpace(items[i].Username),
			"action":     strings.TrimSpace(items[i].Action),
			"details":    strings.TrimSpace(items[i].Details),
			"prev_hash":  strings.TrimSpace(items[i].PrevHash),
			"event_hash": strings.TrimSpace(items[i].EventHash),
			"event_sig":  strings.TrimSpace(items[i].EventSig),
		})
		b.Write(line)
		b.WriteByte('\n')
	}
	return []byte(b.String())
}

func verifyAuditChain(items []store.AuditIntegrityRecord) (chainOK bool, signatureOK bool) {
	chainOK = true
	signatureOK = true
	key := strings.TrimSpace(os.Getenv("BERKUT_AUDIT_SIGNING_KEY"))
	prev := ""
	for i := range items {
		item := items[i]
		exp := hashAuditEvent(prev, item.Username, item.Action, item.Details, item.CreatedAt)
		cur := strings.TrimSpace(item.EventHash)
		if cur == "" {
			cur = exp
		}
		if strings.TrimSpace(item.PrevHash) != "" && strings.TrimSpace(item.PrevHash) != prev {
			chainOK = false
		}
		if cur != exp {
			chainOK = false
		}
		if key != "" {
			sig := strings.TrimSpace(item.EventSig)
			if sig == "" || sig != signWithHMACSHA256(cur, []byte(key)) {
				signatureOK = false
			}
		}
		prev = cur
	}
	if key == "" {
		signatureOK = false
	}
	return chainOK, signatureOK
}

func sumHex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func signWithHMACSHA256(payload string, key []byte) string {
	if strings.TrimSpace(payload) == "" || len(key) == 0 {
		return ""
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func hashAuditEvent(prevHash, username, action, details string, createdAt time.Time) string {
	payload := strings.Join([]string{
		strings.TrimSpace(prevHash),
		createdAt.UTC().Format(time.RFC3339Nano),
		strings.TrimSpace(username),
		strings.TrimSpace(action),
		strings.TrimSpace(details),
	}, "|")
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func (f logFilter) String() string {
	return fmt.Sprintf("since=%s limit=%d", f.Since.UTC().Format(time.RFC3339), f.Limit)
}
