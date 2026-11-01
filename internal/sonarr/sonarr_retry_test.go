package sonarr

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mizu/internal/models"
)

// recordingTransport serves canned responses/errors and counts attempts,
// giving precise control over what each attempt sees.
type recordingTransport struct {
	mu       chan struct{}
	attempts int
	respond  func(attempt int) (*http.Response, error)
}

func newRecordingTransport(respond func(attempt int) (*http.Response, error)) *recordingTransport {
	return &recordingTransport{mu: make(chan struct{}, 1), respond: respond}
}

func (t *recordingTransport) count() int {
	t.mu <- struct{}{}
	n := t.attempts
	<-t.mu
	return n
}

func (t *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.mu <- struct{}{}
	t.attempts++
	<-t.mu
	resp, err := t.respond(t.attempts)
	if resp != nil {
		resp.Request = req
	}
	return resp, err
}

func jsonOK(body string) *http.Response {
	return &http.Response{
		StatusCode:    http.StatusOK,
		Status:        "200 OK",
		Header:        http.Header{"Content-Type": []string{"application/json"}},
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
	}
}

func statusOnly(status int, retryAfter string) *http.Response {
	h := http.Header{}
	if retryAfter != "" {
		h.Set("Retry-After", retryAfter)
	}
	return &http.Response{
		StatusCode: status,
		Status:     "error",
		Header:     h,
		Body:       io.NopCloser(strings.NewReader("")),
	}
}

func fastRetryClient(transport http.RoundTripper) *Client {
	c := New("http://sonarr.test", "key", 2*time.Second)
	c.httpClient = &http.Client{Transport: transport, Timeout: 2 * time.Second}
	c.retryBaseBackoff = time.Millisecond
	c.retryMaxBackoff = 2 * time.Millisecond
	return c
}

func TestGetRetriesTransient500AndSucceeds(t *testing.T) {
	tr := newRecordingTransport(func(attempt int) (*http.Response, error) {
		if attempt < 2 {
			return statusOnly(http.StatusInternalServerError, ""), nil
		}
		return jsonOK(`{"version":"3.0.6"}`), nil
	})
	client := fastRetryClient(tr)

	status, err := client.TestConnection(context.Background())
	require.NoError(t, err)
	require.NotNil(t, status)
	assert.Equal(t, "3.0.6", status.Version)
	assert.Equal(t, 2, tr.count())
}

func TestGetExhaustsRetriesOnPersistentFailure(t *testing.T) {
	tr := newRecordingTransport(func(int) (*http.Response, error) {
		return statusOnly(http.StatusBadGateway, ""), nil
	})
	client := fastRetryClient(tr)

	_, err := client.GetAllSeries(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "502")
	assert.Equal(t, client.retryMaxAttempts, tr.count())
}

func TestPostIsNotRetriedOnHTTPError(t *testing.T) {
	// A POST that got a real HTTP response may have landed server-side;
	// retrying it could double-add a series.
	tr := newRecordingTransport(func(int) (*http.Response, error) {
		return statusOnly(http.StatusInternalServerError, ""), nil
	})
	client := fastRetryClient(tr)

	_, err := client.AddSeries(context.Background(),
		models.SonarrSeries{Title: "Test", TVDBID: 1},
		AddSeriesOptions{RootFolderPath: "/anime"},
	)
	require.Error(t, err)
	assert.Equal(t, 1, tr.count(), "POST must not be retried after an ambiguous server response")
}

func TestPostIsRetriedOnDialFailure(t *testing.T) {
	// Dial failures provably never reached the server — safe to resend.
	dialErr := &net.OpError{Op: "dial", Err: errors.New("connection refused")}
	tr := newRecordingTransport(func(int) (*http.Response, error) {
		return nil, dialErr
	})
	client := fastRetryClient(tr)

	_, err := client.AddSeries(context.Background(),
		models.SonarrSeries{Title: "Test", TVDBID: 1},
		AddSeriesOptions{RootFolderPath: "/anime"},
	)
	require.Error(t, err)
	assert.True(t, errors.Is(err, dialErr))
	assert.Equal(t, client.retryMaxAttempts, tr.count())
}

func TestPostIsNotRetriedOnMidFlightAbort(t *testing.T) {
	// A read abort after send is ambiguous for POSTs.
	abortErr := &net.OpError{Op: "read", Err: errors.New("connection reset")}
	tr := newRecordingTransport(func(int) (*http.Response, error) {
		return nil, abortErr
	})
	client := fastRetryClient(tr)

	_, err := client.AddSeries(context.Background(),
		models.SonarrSeries{Title: "Test", TVDBID: 1},
		AddSeriesOptions{RootFolderPath: "/anime"},
	)
	require.Error(t, err)
	assert.Equal(t, 1, tr.count())
}

func TestGetIsRetriedOnMidFlightAbort(t *testing.T) {
	abortErr := &net.OpError{Op: "read", Err: errors.New("connection reset")}
	tr := newRecordingTransport(func(int) (*http.Response, error) {
		return nil, abortErr
	})
	client := fastRetryClient(tr)

	_, err := client.GetEpisodes(context.Background(), 42)
	require.Error(t, err)
	assert.Equal(t, client.retryMaxAttempts, tr.count())
}

func TestGetIsRetriedOn429(t *testing.T) {
	tr := newRecordingTransport(func(attempt int) (*http.Response, error) {
		if attempt == 1 {
			return statusOnly(http.StatusTooManyRequests, ""), nil
		}
		return jsonOK(`[{"id":1,"title":"One Piece"}]`), nil
	})
	client := fastRetryClient(tr)

	series, err := client.GetAllSeries(context.Background())
	require.NoError(t, err)
	require.Len(t, series, 1)
	assert.Equal(t, 2, tr.count())
}

func TestPutIsRetriedOnTransientFailure(t *testing.T) {
	tr := newRecordingTransport(func(attempt int) (*http.Response, error) {
		if attempt < 3 {
			return statusOnly(http.StatusServiceUnavailable, ""), nil
		}
		return statusOnly(http.StatusOK, ""), nil
	})
	client := fastRetryClient(tr)

	err := client.BulkSetEpisodesMonitored(context.Background(), []int{1, 2}, false)
	require.NoError(t, err)
	assert.Equal(t, 3, tr.count())
}

func TestErrorBodyIsCapped(t *testing.T) {
	huge := strings.Repeat("x", maxErrorBodyBytes*4)
	tr := newRecordingTransport(func(int) (*http.Response, error) {
		resp := statusOnly(http.StatusInternalServerError, "")
		resp.Body = io.NopCloser(strings.NewReader(huge))
		return resp, nil
	})
	client := fastRetryClient(tr)
	client.retryMaxAttempts = 1

	_, err := client.GetAllSeries(context.Background())
	require.Error(t, err)
	assert.LessOrEqual(t, len(err.Error()), maxErrorBodyBytes+64,
		"error message must not embed an unbounded body")
}

func TestContextCancelAbortsRetrySleep(t *testing.T) {
	tr := newRecordingTransport(func(int) (*http.Response, error) {
		return statusOnly(http.StatusInternalServerError, ""), nil
	})
	client := fastRetryClient(tr)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	_, err := client.GetEpisodes(ctx, 7)
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled), "cancellation should surface, got: %v", err)
	assert.Less(t, time.Since(start), time.Second)
}
