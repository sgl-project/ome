package modelagent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sgl-project/ome/pkg/apis/ome/v1beta1"
	"github.com/sgl-project/ome/pkg/utils/storage"
)

const (
	artifactManifestVersion = 1
	artifactManifestFileExt = ".json"
	artifactHashAlgorithm   = "sha256"
)

type artifactManifest struct {
	Version      int                 `json:"version"`
	StorageURI   string              `json:"storageUri,omitempty"`
	StoragePath  string              `json:"storagePath,omitempty"`
	SourceType   string              `json:"sourceType,omitempty"`
	ArtifactRoot string              `json:"artifactRoot"`
	CreatedAt    string              `json:"createdAt"`
	Files        []artifactFileEntry `json:"files"`
}

type artifactFileEntry struct {
	Path          string `json:"path"`
	Size          int64  `json:"size"`
	Hash          string `json:"hash,omitempty"`
	HashAlgorithm string `json:"hashAlgorithm,omitempty"`
}

func validateArtifactManifest(ctx context.Context, spec *v1beta1.BaseModelSpec, modelRootDir, modelPath string, deep bool) integrityReport {
	manifest, root, err := loadArtifactManifest(spec, modelRootDir, modelPath)
	if err != nil {
		if os.IsNotExist(err) {
			return inconclusiveReport(integrityReasonManifestError, err.Error())
		}
		return failureReport(integrityReasonManifestError, err.Error())
	}
	if manifest.Version != artifactManifestVersion {
		return failureReport(integrityReasonManifestError, fmt.Sprintf("unsupported artifact manifest version %d", manifest.Version))
	}
	if len(manifest.Files) == 0 {
		return failureReport(integrityReasonManifestError, "artifact manifest does not contain any files")
	}

	var bytesScanned int64
	for _, file := range manifest.Files {
		relPath, err := cleanManifestRelativePath(file.Path)
		if err != nil {
			return failureReport(integrityReasonManifestError, err.Error())
		}
		localPath := filepath.Join(root, relPath)
		info, err := os.Stat(localPath)
		if err != nil {
			if os.IsNotExist(err) {
				return failureReport(integrityReasonMissingWeight, fmt.Sprintf("manifest file is missing: %s", file.Path))
			}
			return failureReport(integrityReasonManifestError, err.Error())
		}
		if !info.Mode().IsRegular() {
			return failureReport(integrityReasonManifestError, fmt.Sprintf("manifest path is not a regular file: %s", file.Path))
		}
		if info.Size() != file.Size {
			return failureReport(integrityReasonSizeMismatch,
				fmt.Sprintf("manifest file size mismatch for %s: expected %d got %d", file.Path, file.Size, info.Size()))
		}
		if deep && file.Hash != "" {
			if file.HashAlgorithm != artifactHashAlgorithm {
				return failureReport(integrityReasonManifestError,
					fmt.Sprintf("unsupported manifest hash algorithm %s for %s", file.HashAlgorithm, file.Path))
			}
			hash, scanned, err := hashFile(ctx, localPath)
			bytesScanned += scanned
			if err != nil {
				return failureReport(integrityReasonManifestError, err.Error())
			}
			if hash != file.Hash {
				return failureReport(integrityReasonChecksumMismatch,
					fmt.Sprintf("manifest checksum mismatch for %s", file.Path))
			}
		}
	}
	return successReport(integrityReasonOK, bytesScanned)
}

func createArtifactManifest(ctx context.Context, spec *v1beta1.BaseModelSpec, modelRootDir, modelPath string, storageType storage.StorageType) (integrityReport, error) {
	root, err := resolveManifestRoot(modelPath)
	if err != nil {
		return integrityReport{}, err
	}

	manifestPath, storageURI, storagePath, err := artifactManifestPath(spec, modelRootDir, modelPath)
	if err != nil {
		return integrityReport{}, err
	}
	manifest := artifactManifest{
		Version:      artifactManifestVersion,
		StorageURI:   storageURI,
		StoragePath:  storagePath,
		SourceType:   string(storageType),
		ArtifactRoot: root,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}

	var bytesScanned int64
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == artifactManifestDir {
				return filepath.SkipDir
			}
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}

		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		hash, scanned, err := hashFile(ctx, path)
		if err != nil {
			return err
		}
		bytesScanned += scanned
		manifest.Files = append(manifest.Files, artifactFileEntry{
			Path:          filepath.ToSlash(relPath),
			Size:          info.Size(),
			Hash:          hash,
			HashAlgorithm: artifactHashAlgorithm,
		})
		return nil
	})
	if err != nil {
		return integrityReport{}, err
	}
	if len(manifest.Files) == 0 {
		return integrityReport{}, fmt.Errorf("no regular files found under model artifact path %s", modelPath)
	}
	sort.Slice(manifest.Files, func(i, j int) bool {
		return manifest.Files[i].Path < manifest.Files[j].Path
	})

	if err := writeArtifactManifest(manifestPath, manifest); err != nil {
		return integrityReport{}, err
	}
	return successReport(integrityReasonBaselineCreated, bytesScanned), nil
}

func loadArtifactManifest(spec *v1beta1.BaseModelSpec, modelRootDir, modelPath string) (*artifactManifest, string, error) {
	root, err := resolveManifestRoot(modelPath)
	if err != nil {
		return nil, "", err
	}
	manifestPath, _, _, err := artifactManifestPath(spec, modelRootDir, modelPath)
	if err != nil {
		return nil, "", err
	}
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, root, err
	}
	var manifest artifactManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, root, err
	}
	return &manifest, root, nil
}

func artifactManifestPath(spec *v1beta1.BaseModelSpec, modelRootDir, modelPath string) (manifestPath, storageURI, storagePath string, err error) {
	if modelRootDir == "" {
		return "", "", "", fmt.Errorf("model root directory is empty")
	}
	storageURI, storagePath, ok := StorageIdentityForSpec(spec)
	if !ok {
		return "", "", "", fmt.Errorf("model storage identity is missing")
	}
	if storagePath == "" {
		storagePath = modelPath
	}
	sum := sha256.Sum256([]byte(storageURI + "\n" + storagePath))
	filename := hex.EncodeToString(sum[:]) + artifactManifestFileExt
	return filepath.Join(modelRootDir, artifactManifestDir, "integrity", filename), storageURI, storagePath, nil
}

func writeArtifactManifest(manifestPath string, manifest artifactManifest) error {
	manifestDir := filepath.Dir(manifestPath)
	if err := os.MkdirAll(manifestDir, 0755); err != nil {
		return err
	}

	tmpFile, err := os.CreateTemp(manifestDir, "artifact-manifest-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmpFile.Name()
	encoder := json.NewEncoder(tmpFile)
	encoder.SetIndent("", "  ")
	encodeErr := encoder.Encode(manifest)
	closeErr := tmpFile.Close()
	if encodeErr != nil {
		_ = os.Remove(tmpName)
		return encodeErr
	}
	if closeErr != nil {
		_ = os.Remove(tmpName)
		return closeErr
	}
	return os.Rename(tmpName, manifestPath)
}

func resolveManifestRoot(modelPath string) (string, error) {
	if modelPath == "" {
		return "", fmt.Errorf("model artifact path is empty")
	}
	resolved, err := filepath.EvalSymlinks(modelPath)
	if err != nil {
		return "", err
	}
	return resolved, nil
}

func cleanManifestRelativePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("manifest contains an empty file path")
	}
	cleaned := filepath.Clean(filepath.FromSlash(path))
	if filepath.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("manifest contains an invalid relative path: %s", path)
	}
	return cleaned, nil
}

func hashFile(ctx context.Context, path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()

	hasher := sha256.New()
	buffer := make([]byte, 1024*1024)
	var total int64
	for {
		select {
		case <-ctx.Done():
			return "", total, ctx.Err()
		default:
		}

		n, readErr := file.Read(buffer)
		if n > 0 {
			total += int64(n)
			if _, err := hasher.Write(buffer[:n]); err != nil {
				return "", total, err
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", total, readErr
		}
	}
	return hex.EncodeToString(hasher.Sum(nil)), total, nil
}
