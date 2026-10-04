package uassettool

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
)

func TestDecryptIoStoreDirectCallsWorker(t *testing.T) {
	for _, key := range []string{"", "fixture-key"} {
		// Arrange
		fake := &fakeCaller{respond: respondWithJSON(`{"encrypted":false,"changed":true}`)}

		// Act
		err := DecryptIoStoreDirect(fake, "fixture.utoc", key)

		// Assert
		if err != nil {
			t.Fatal(err)
		}
		if fake.action != "decrypt_iostore" || fake.params["file_path"] != "fixture.utoc" {
			t.Fatal("the decrypt request has the wrong action or path")
		}
		if key == "" {
			if _, exists := fake.params["aes_key"]; exists {
				t.Fatal("an empty key overrides the worker default")
			}
		} else if fake.params["aes_key"] != key {
			t.Fatal("the supplied key did not reach the worker")
		}
	}
}

func TestDecryptIoStoreDirectRejectsMissingPath(t *testing.T) {
	// Arrange
	fake := &fakeCaller{}

	// Act
	err := DecryptIoStoreDirect(fake, "", "")

	// Assert
	if err == nil || fake.action != "" {
		t.Fatal("an empty path reached the worker")
	}
}

func TestDecryptIoStoreDirectPropagatesWorkerFailure(t *testing.T) {
	// Arrange
	want := &ToolError{Action: "decrypt_iostore", Message: "invalid TOC"}
	fake := &fakeCaller{err: want}

	// Act
	err := DecryptIoStoreDirect(fake, "fixture.utoc", "")

	// Assert
	if !errors.Is(err, want) {
		t.Fatalf("worker error = %v, want %v", err, want)
	}
}

func TestDecryptIoStoreDirectRejectsUnverifiedState(t *testing.T) {
	for _, body := range []string{`{}`, `{"encrypted":true}`} {
		// Arrange
		fake := &fakeCaller{respond: respondWithJSON(body)}

		// Act
		err := DecryptIoStoreDirect(fake, "fixture.utoc", "")

		// Assert
		if err == nil {
			t.Fatalf("accepted an unverified decrypt response: %s", body)
		}
		if body == `{}` && !errors.Is(err, ErrMalformedResponse) {
			t.Fatalf("missing state error = %v, want ErrMalformedResponse", err)
		}
	}
}

func TestDecryptIoStoreDirectWithWorker(t *testing.T) {
	// Arrange
	executable := pinnedWorkerExecutablePath(t)
	if _, err := os.Stat(executable); err != nil {
		t.Skip("fetch the worker to run this archive test")
	}
	worker, err := NewWorker(WorkerConfig{ExecutablePath: executable, ExpectedSourceRevision: PinnedSourceRevision})
	if err != nil {
		t.Fatal(err)
	}
	defer worker.Close()
	path := buildIoStoreFixture(t, worker, t.TempDir())
	plainTOC, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	base := strings.TrimSuffix(path, ".utoc")
	plainUCAS, err := os.ReadFile(base + ".ucas")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := EncryptIoStoreDirect(worker, base+".pak", MarvelRivalsAESKey); err != nil {
		t.Fatal(err)
	}

	// Act
	err = DecryptIoStoreDirect(worker, path, MarvelRivalsAESKey)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := IsIoStoreEncrypted(worker, path)
	if err != nil || encrypted {
		t.Fatalf("decrypted state = %v, %v", encrypted, err)
	}
	gotTOC, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotTOC[56:64], plainTOC[56:64]) {
		t.Fatal("container identity changed")
	}
	gotUCAS, err := os.ReadFile(base + ".ucas")
	if err != nil {
		t.Fatal(err)
	}
	if len(gotUCAS) < len(plainUCAS) || !bytes.Equal(gotUCAS[:len(plainUCAS)], plainUCAS) || len(bytes.Trim(gotUCAS[len(plainUCAS):], "\x00")) != 0 {
		t.Fatal("the original container header payload changed")
	}
}
