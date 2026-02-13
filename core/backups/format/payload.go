package format

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"time"
)

type Checksums struct {
	ManifestSHA256 string `json:"manifest_sha256"`
	DumpSHA256     string `json:"dump_sha256"`
}

type PayloadInfo struct {
	Manifest      Manifest  `json:"manifest"`
	Checksums     Checksums `json:"checksums"`
	ManifestBytes []byte
}

func BuildPayload(dumpPath, outPath string, manifest Manifest) (*PayloadInfo, error) {
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	dumpSHA, dumpSize, err := fileSHA256(dumpPath)
	if err != nil {
		return nil, err
	}
	checksums := Checksums{
		ManifestSHA256: sha256Hex(manifestBytes),
		DumpSHA256:     dumpSHA,
	}
	checksumBytes, err := json.Marshal(checksums)
	if err != nil {
		return nil, err
	}

	out, err := os.Create(outPath)
	if err != nil {
		return nil, err
	}
	defer out.Close()
	tw := tar.NewWriter(out)
	defer tw.Close()

	if err := writeTarBytes(tw, "manifest.json", manifestBytes); err != nil {
		return nil, err
	}
	if err := writeTarFile(tw, "db.dump", dumpPath, dumpSize); err != nil {
		return nil, err
	}
	if err := writeTarBytes(tw, "checksums.json", checksumBytes); err != nil {
		return nil, err
	}
	return &PayloadInfo{
		Manifest:      manifest,
		Checksums:     checksums,
		ManifestBytes: manifestBytes,
	}, nil
}

func writeTarBytes(tw *tar.Writer, name string, payload []byte) error {
	hdr := &tar.Header{
		Name:    name,
		Mode:    0600,
		Size:    int64(len(payload)),
		ModTime: time.Now().UTC(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := tw.Write(payload)
	return err
}

func writeTarFile(tw *tar.Writer, name, path string, size int64) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	hdr := &tar.Header{
		Name:    name,
		Mode:    0600,
		Size:    size,
		ModTime: time.Now().UTC(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err = io.Copy(tw, f)
	return err
}

func fileSHA256(path string) (sum string, size int64, err error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

func sha256Hex(in []byte) string {
	h := sha256.Sum256(in)
	return hex.EncodeToString(h[:])
}
