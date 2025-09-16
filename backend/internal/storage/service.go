package storage

import (
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "io"
    "os"
    "path/filepath"
)

type Service struct {
    RootDir string
}

func New(root string) *Service { return &Service{RootDir: root} }

// WriteAndHash stores the content to a deterministic path by SHA-256. If the file already
// exists, it is left intact. Returns hex hash string and absolute path.
func (s *Service) WriteAndHash(r io.Reader) (string, string, int64, error) {
    hasher := sha256.New()
    tmpFile, err := os.CreateTemp(s.RootDir, "upload-*")
    if err != nil { return "", "", 0, err }
    defer func() { _ = tmpFile.Close(); _ = os.Remove(tmpFile.Name()) }()

    written, err := io.Copy(io.MultiWriter(hasher, tmpFile), r)
    if err != nil { return "", "", 0, err }
    sum := hex.EncodeToString(hasher.Sum(nil))
    // path by hash prefix for sharding
    shard := filepath.Join(s.RootDir, sum[:2], sum[2:4])
    if err := os.MkdirAll(shard, 0o755); err != nil { return "", "", 0, err }
    finalPath := filepath.Join(shard, sum)
    if _, err := os.Stat(finalPath); err == nil {
        // already exists; dedup
        return sum, finalPath, written, nil
    }
    // move temp to final
    if err := os.Rename(tmpFile.Name(), finalPath); err != nil {
        return "", "", 0, fmt.Errorf("rename: %w", err)
    }
    return sum, finalPath, written, nil
}



