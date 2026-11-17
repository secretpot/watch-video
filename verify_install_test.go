package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestVerifyInstallFailsWithoutMediaTools(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	result, err := verifyInstallResult(t)
	if err == nil || result.Ready || result.Video.Status != "error" || result.Speech.Status != "not_run" {
		t.Fatalf("verification = %+v, %v; want a failed video check and no speech check", result, err)
	}
}

func TestVerifyInstallFailsWithoutModel(t *testing.T) {
	videoTools(t)
	missing := filepath.Join(t.TempDir(), "missing-model.bin")
	result, err := verifyInstallResult(t, "--whisper-model", missing)
	if err == nil || result.Ready || result.Video.Status != "verified" || result.Speech.Status != "model_missing" || result.Speech.Model != missing {
		t.Fatalf("verification = %+v, %v; want the missing model reported after video decode", result, err)
	}
}

func TestVerifyInstallWithLocalWhisper(t *testing.T) {
	model := os.Getenv("WATCH_VIDEO_TEST_WHISPER_MODEL")
	if model == "" {
		t.Skip("set WATCH_VIDEO_TEST_WHISPER_MODEL to run local speech verification")
	}
	if runtime.GOOS == "windows" {
		// Reproduce model and sample paths under the same non-ASCII profile.
		profile := filepath.Join(t.TempDir(), "中文 用户")
		if err := os.Mkdir(profile, 0700); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(model)
		if err != nil {
			t.Fatal(err)
		}
		model = filepath.Join(profile, "whisper.bin")
		if err := os.WriteFile(model, data, 0600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("TEMP", profile)
		t.Setenv("TMP", profile)
	}
	result, err := verifyInstallResult(t, "--whisper-model", model)
	if err != nil || !result.Ready || result.Video.Status != "verified" || result.Speech.Status != "verified" || !strings.Contains(strings.ToLower(result.Speech.Transcript), "brown fox") {
		t.Fatalf("verification = %+v, %v; want decoded video and recognizable local speech", result, err)
	}
	broken := filepath.Join(t.TempDir(), "invalid-model.bin")
	if err := os.WriteFile(broken, []byte("this file does not contain Whisper weights"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err = verifyInstallResult(t, "--whisper-model", broken)
	if err == nil || result.Ready || result.Video.Status != "verified" || result.Speech.Status != "error" {
		t.Fatalf("verification = %+v, %v; want corrupt model failure", result, err)
	}
}

func verifyInstallResult(t *testing.T, options ...string) (installVerification, error) {
	t.Helper()
	var stdout bytes.Buffer
	err := run(t.Context(), append([]string{"verify-install"}, options...), &stdout)
	var result installVerification
	if decodeErr := json.Unmarshal(stdout.Bytes(), &result); decodeErr != nil {
		t.Fatalf("read verification report %q: %v (command error: %v)", stdout.String(), decodeErr, err)
	}
	return result, err
}
