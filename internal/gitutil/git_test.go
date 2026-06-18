package gitutil

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
)

func TestRunWithRetry_RetriesTransientErrors(t *testing.T) {
	attempts := 0
	err := RunWithRetry(Options{Attempts: 3}, func() error {
		attempts++
		if attempts < 3 {
			return fmt.Errorf("failed to clone repository: read tcp 127.0.0.1: connection reset by peer")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RunWithRetry() error = %v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}

func TestRunWithRetry_DoesNotRetryPermanentErrors(t *testing.T) {
	attempts := 0
	wantErr := errors.New("ref release-1 not found as branch or tag")
	err := RunWithRetry(Options{Attempts: 3}, func() error {
		attempts++
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("RunWithRetry() error = %v, want %v", err, wantErr)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}

func TestNewGitHTTPClient_TransportLevelTimeoutNotTotal(t *testing.T) {
	c := newGitHTTPClient(30 * time.Second)
	if c.Timeout != 0 {
		t.Errorf("Client.Timeout = %v, want 0 (transport-level timeouts, not total request timeout)", c.Timeout)
	}
	tr, ok := c.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport type = %T, want *http.Transport", c.Transport)
	}
	if tr.ResponseHeaderTimeout != 30*time.Second {
		t.Errorf("ResponseHeaderTimeout = %v, want 30s", tr.ResponseHeaderTimeout)
	}
	if tr.TLSHandshakeTimeout != 30*time.Second {
		t.Errorf("TLSHandshakeTimeout = %v, want 30s", tr.TLSHandshakeTimeout)
	}
}

func TestRunWithRetry_RetriesRealHTTPTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	t.Cleanup(server.Close)

	attempts := 0
	err := RunWithRetry(Options{Attempts: 2, Timeout: 20 * time.Millisecond, RetryDelay: time.Millisecond}, func() error {
		attempts++
		_, err := git.PlainClone(t.TempDir(), false, &git.CloneOptions{URL: server.URL + "/repo.git"})
		return err
	})
	if err == nil {
		t.Fatal("expected timeout error after retries")
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2 (real HTTP timeout should be classified transient and retried)", attempts)
	}
}

func TestRunWithRetry_AppliesHTTPTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = w.Write([]byte("not a git server"))
	}))
	t.Cleanup(server.Close)

	started := time.Now()
	err := RunWithRetry(Options{Attempts: 1, Timeout: 20 * time.Millisecond}, func() error {
		_, err := git.PlainClone(t.TempDir(), false, &git.CloneOptions{URL: server.URL + "/repo.git"})
		return err
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed := time.Since(started); elapsed >= 150*time.Millisecond {
		t.Fatalf("operation took %s, want timeout before server responds", elapsed)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "timeout") && !strings.Contains(strings.ToLower(err.Error()), "deadline") {
		t.Fatalf("error = %v, want timeout/deadline", err)
	}
}

func TestCloneWithCleanup_RemovesPartialBeforeRetry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clone")
	attempt := 0
	client := NewClient(Options{Attempts: 2, RetryDelay: time.Millisecond})
	err := client.CloneWithCleanup(path, func() error {
		attempt++
		if attempt == 1 {
			if err := os.MkdirAll(path, 0755); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(path, "partial.txt"), []byte("partial"), 0644); err != nil {
				return err
			}
			return fmt.Errorf("connection reset by peer")
		}
		if _, err := os.Stat(filepath.Join(path, "partial.txt")); err == nil {
			return fmt.Errorf("partial file should have been removed before retry")
		}
		return os.MkdirAll(path, 0755)
	})
	if err != nil {
		t.Fatalf("CloneWithCleanup: %v", err)
	}
	if attempt != 2 {
		t.Fatalf("attempts = %d, want 2", attempt)
	}
}

func TestCloneWithCleanup_CleanupFailureIncludesPriorCloneError(t *testing.T) {
	oldRemoveAll := removeAll
	removeAll = func(string) error {
		return fmt.Errorf("permission denied")
	}
	t.Cleanup(func() { removeAll = oldRemoveAll })

	client := NewClient(Options{Attempts: 2, RetryDelay: time.Millisecond})
	err := client.CloneWithCleanup(filepath.Join(t.TempDir(), "clone"), func() error {
		return fmt.Errorf("connection reset by peer")
	})
	if err == nil {
		t.Fatal("expected cleanup error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "permission denied") {
		t.Fatalf("error = %v, want cleanup failure", err)
	}
	if !strings.Contains(msg, "prior clone error") || !strings.Contains(msg, "connection reset by peer") {
		t.Fatalf("error = %v, want prior clone error context", err)
	}
}
