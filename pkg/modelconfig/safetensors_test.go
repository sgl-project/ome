package modelconfig

import (
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestParseSafetensorsBytesParsesHeaderOnly(t *testing.T) {
	header := readSafetensorsHeaderForTest(t, filepath.Join("testdata", "tiny-random-PhiModel", "model-1-of-2.safetensors"))

	count, err := ParseSafetensorsBytes(header, "model-1-of-2.safetensors")
	if err != nil {
		t.Fatalf("ParseSafetensorsBytes returned error: %v", err)
	}
	if count <= 0 {
		t.Fatalf("expected positive parameter count, got %d", count)
	}
}

func TestFindAndParseSafetensorsFilesParsesHeaderInputs(t *testing.T) {
	modelDir := filepath.Join("testdata", "tiny-random-PhiModel")

	count, err := FindAndParseSafetensorsFiles([]SafetensorsFileInput{
		{
			Path: "model.safetensors.index.json",
			Data: []byte(`{"weight_map":{"a":"model-1-of-2.safetensors","b":"model-2-of-2.safetensors"}}`),
		},
		{
			Path: "model-1-of-2.safetensors",
			Data: readSafetensorsHeaderForTest(t, filepath.Join(modelDir, "model-1-of-2.safetensors")),
		},
		{
			Path: "model-2-of-2.safetensors",
			Data: readSafetensorsHeaderForTest(t, filepath.Join(modelDir, "model-2-of-2.safetensors")),
		},
	})
	if err != nil {
		t.Fatalf("FindAndParseSafetensorsFiles returned error: %v", err)
	}
	if count != 92564 {
		t.Fatalf("expected parameter count 92564, got %d", count)
	}
}

func TestFindAndParseSafetensorsFilesResolvesIndexSubdirectory(t *testing.T) {
	modelDir := filepath.Join("testdata", "tiny-random-PhiModel")

	count, err := FindAndParseSafetensorsFiles([]SafetensorsFileInput{
		{
			Path: "weights/model.safetensors.index.json",
			Data: []byte(`{"weight_map":{"a":"model-1-of-2.safetensors","b":"model-2-of-2.safetensors"}}`),
		},
		{
			Path: "weights/model-1-of-2.safetensors",
			Data: readSafetensorsHeaderForTest(t, filepath.Join(modelDir, "model-1-of-2.safetensors")),
		},
		{
			Path: "weights/model-2-of-2.safetensors",
			Data: readSafetensorsHeaderForTest(t, filepath.Join(modelDir, "model-2-of-2.safetensors")),
		},
	})
	if err != nil {
		t.Fatalf("FindAndParseSafetensorsFiles returned error: %v", err)
	}
	if count != 92564 {
		t.Fatalf("expected parameter count 92564, got %d", count)
	}
}

func readSafetensorsHeaderForTest(t *testing.T, filePath string) []byte {
	t.Helper()

	file, err := os.Open(filePath)
	if err != nil {
		t.Fatalf("open safetensors fixture: %v", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Fatalf("close safetensors fixture: %v", err)
		}
	}()

	headerLenBytes := make([]byte, 8)
	if _, err := io.ReadFull(file, headerLenBytes); err != nil {
		t.Fatalf("read safetensors header length: %v", err)
	}

	headerLen := binary.LittleEndian.Uint64(headerLenBytes)
	header := make([]byte, headerLen)
	if _, err := io.ReadFull(file, header); err != nil {
		t.Fatalf("read safetensors header: %v", err)
	}

	data := make([]byte, 0, 8+len(header))
	data = append(data, headerLenBytes...)
	data = append(data, header...)
	return data
}
