package incidents

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"berkut-scc/config"
	"berkut-scc/core/store"
	"berkut-scc/core/utils"
)

type Service struct {
	audits store.AuditStore
	encryptor *utils.Encryptor
	storageDir string
}

func NewService(cfg *config.AppConfig, audits store.AuditStore) (*Service, error) {
	enc, err := utils.NewEncryptorFromString(cfg.Docs.EncryptionKey)
	if err != nil {
		return nil, err
	}
	storageDir := strings.TrimSpace(cfg.Incidents.StorageDir)
	if storageDir == "" {
		storageDir = "data/incidents"
	}
	if err := os.MkdirAll(storageDir, 0o700); err != nil {
		return nil, err
	}
	return &Service{audits: audits, encryptor: enc, storageDir: storageDir}, nil
}

func (s *Service) Log(ctx context.Context, username, action, details string) {
	if s.audits != nil {
		_ = s.audits.Log(ctx, username, action, details)
	}
}

func (s *Service) StorageDir() string {
	return s.storageDir
}

func (s *Service) Encryptor() *utils.Encryptor {
	return s.encryptor
}

func (s *Service) AttachmentPath(incidentID, attachmentID int64) string {
	return filepath.Join(s.storageDir, fmt.Sprintf("%d", incidentID), "attachments", fmt.Sprintf("%d.enc", attachmentID))
}

func sanitizeArtifactID(raw string) string {
	clean := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, strings.TrimSpace(raw))
	clean = strings.Trim(clean, "-_")
	if clean == "" {
		return "artifact"
	}
	if len(clean) > 64 {
		return clean[:64]
	}
	return clean
}

func (s *Service) ArtifactFilePath(incidentID int64, artifactID string, fileID int64) string {
	return filepath.Join(s.storageDir, fmt.Sprintf("%d", incidentID), "artifacts", sanitizeArtifactID(artifactID), fmt.Sprintf("%d.enc", fileID))
}

func (s *Service) CheckACL(user *store.User, roles []string, acl []store.ACLRule, required string) bool {
	if user == nil {
		return false
	}
	req := strings.ToLower(strings.TrimSpace(required))
	if req == "" {
		return false
	}
	allowed := map[string]struct{}{req: {}}
	switch req {
	case "view":
		allowed["edit"] = struct{}{}
		allowed["manage"] = struct{}{}
	case "edit":
		allowed["manage"] = struct{}{}
	}
	for _, rule := range acl {
		if _, ok := allowed[strings.ToLower(rule.Permission)]; !ok {
			continue
		}
		switch strings.ToLower(rule.SubjectType) {
		case "user":
			if rule.SubjectID == user.Username || rule.SubjectID == fmt.Sprintf("%d", user.ID) {
				return true
			}
		case "role":
			for _, r := range roles {
				if r == rule.SubjectID {
					return true
				}
			}
		}
	}
	return false
}
