package restore

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"berkut-scc/core/backups/format"
)

type PayloadData struct {
	Manifest     format.Manifest
	Checksums    format.Checksums
	DumpPath     string
	ManifestSHA  string
	DumpSHA      string
	MissingParts []string
}

func ExtractPayload(tarPath, workDir string) (*PayloadData, error) {
	f, err := os.Open(tarPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	tr := tar.NewReader(f)
	out := &PayloadData{}
	var manifestBytes []byte
	var checksBytes []byte

	dumpPath := filepath.Join(workDir, "restore-db.dump")
	var dumpFile *os.File
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		name := strings.TrimSpace(hdr.Name)
		switch name {
		case "manifest.json":
			payload, readErr := io.ReadAll(tr)
			if readErr != nil {
				return nil, readErr
			}
			manifestBytes = payload
		case "checksums.json":
			payload, readErr := io.ReadAll(tr)
			if readErr != nil {
				return nil, readErr
			}
			checksBytes = payload
		case "db.dump":
			if dumpFile != nil {
				_ = dumpFile.Close()
			}
			file, createErr := os.Create(dumpPath)
			if createErr != nil {
				return nil, createErr
			}
			dumpFile = file
			hash := sha256.New()
			w := io.MultiWriter(dumpFile, hash)
			if _, copyErr := io.Copy(w, tr); copyErr != nil {
				_ = dumpFile.Close()
				return nil, copyErr
			}
			_ = dumpFile.Close()
			out.DumpPath = dumpPath
			out.DumpSHA = hex.EncodeToString(hash.Sum(nil))
		}
	}

	if len(manifestBytes) == 0 {
		out.MissingParts = append(out.MissingParts, "manifest.json")
	}
	if len(checksBytes) == 0 {
		out.MissingParts = append(out.MissingParts, "checksums.json")
	}
	if strings.TrimSpace(out.DumpPath) == "" {
		out.MissingParts = append(out.MissingParts, "db.dump")
	}
	if len(out.MissingParts) > 0 {
		return out, fmt.Errorf("payload missing required parts")
	}

	if err := json.Unmarshal(manifestBytes, &out.Manifest); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(checksBytes, &out.Checksums); err != nil {
		return nil, err
	}
	manifestHash := sha256.Sum256(manifestBytes)
	out.ManifestSHA = hex.EncodeToString(manifestHash[:])
	return out, nil
}
