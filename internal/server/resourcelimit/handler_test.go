package resourcelimit

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/server/testutils"
	"github.com/Fallen-Breath/pavonis/internal/utils"
	"github.com/stretchr/testify/require"
)

func createPoliciesAtCapacity(t *testing.T) (*Policy, *Policy) {
	t.Helper()
	m := newTestManager(t, 1, config.ResourceQuotaConfig{
		AllClients: config.ClientQuotaConfig{RequestConcurrency: utils.ToPtr(1)},
	})
	a, b := m.CreateSitePolicy("a", ""), m.CreateSitePolicy("b", "")
	t.Cleanup(acquireLease(t, a, "192.0.2.1").Release)
	return a, b
}

type rejectionDeadlineWriter struct {
	*httptest.ResponseRecorder
	read, write time.Time
}

func (w *rejectionDeadlineWriter) SetReadDeadline(d time.Time) error {
	w.read = d
	return nil
}

func (w *rejectionDeadlineWriter) SetWriteDeadline(d time.Time) error {
	w.write = d
	return nil
}

func TestRejectionBoundsReadAndWriteByRemainingRequestTime(t *testing.T) {
	for _, tc := range []struct {
		name            string
		remaining, want time.Duration
	}{
		{"early rejection", time.Hour, time.Second},
		{"request nearly expired", 200 * time.Millisecond, 200 * time.Millisecond},
	} {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				policy, _ := createPoliciesAtCapacity(t)
				now := time.Now()
				ctx, cancel := context.WithTimeout(t.Context(), tc.remaining)
				defer cancel()
				r := httptest.NewRequestWithContext(ctx, http.MethodPost, "/", strings.NewReader("data"))
				w := &rejectionDeadlineWriter{ResponseRecorder: httptest.NewRecorder()}
				policy.ServeRequest(testutils.NewRequestContext(), w, r, func() { t.Fatal("request was admitted") })
				require.Equal(t, now.Add(tc.want), w.read)
				require.Equal(t, now.Add(tc.want), w.write)
			})
		})
	}
}

func TestRejectedHeadWritesNoBody(t *testing.T) {
	policy, _ := createPoliciesAtCapacity(t)
	// ResponseRecorder keeps every byte the handler writes, even for HEAD.
	w := httptest.NewRecorder()
	policy.ServeRequest(testutils.NewRequestContext(), w, httptest.NewRequest(http.MethodHead, "/", nil), func() {
		t.Fatal("request was admitted")
	})
	require.Equal(t, http.StatusTooManyRequests, w.Code)
	require.Zero(t, w.Body.Len())
}

type blockedRejectionWriter struct {
	http.ResponseWriter
	flushing chan struct{}
	release  <-chan struct{}
}

func (w *blockedRejectionWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *blockedRejectionWriter) FlushError() error {
	close(w.flushing)
	<-w.release
	return os.ErrDeadlineExceeded
}

func TestRejectionConcurrencySharedAcrossSitesUntilFlushReturns(t *testing.T) {
	first, second := createPoliciesAtCapacity(t)
	flushing, release := make(chan struct{}), make(chan struct{})
	unblock := sync.OnceFunc(func() { close(release) })
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		policy := second
		if r.URL.Path == "/first" {
			policy = first
			w = &blockedRejectionWriter{w, flushing, release}
		}
		policy.ServeRequest(testutils.NewRequestContext(), w, r, func() { w.WriteHeader(http.StatusOK) })
	}))
	defer srv.Close()
	defer unblock()
	client := srv.Client()
	client.Timeout = 5 * time.Second
	getError := func(path string) error {
		resp, err := client.Get(srv.URL + path)
		if resp != nil {
			resp.Body.Close()
		}
		return err
	}

	finished := make(chan error, 1)
	go func() { finished <- getError("/first") }()
	select {
	case <-flushing:
	case err := <-finished:
		t.Fatalf("first request finished before flushing its 429: %v", err)
	}

	err := getError("/second")
	require.Error(t, err)
	require.NotErrorIs(t, err, context.DeadlineExceeded)
	unblock()
	require.Error(t, <-finished)

	resp, err := client.Get(srv.URL + "/second")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	_, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.True(t, resp.Close, "client should not reuse the connection after a local 429")
}
