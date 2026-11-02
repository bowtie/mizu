package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"math/rand/v2"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mizu/internal/artwork"
	"mizu/internal/models"
)

// noisyPNG encodes a w×h image of random pixels, which PNG can't compress,
// so the encoded file is roughly w*h*4 bytes.
func noisyPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	_, _ = rand.NewChaCha8([32]byte{1}).Read(img.Pix)
	var b bytes.Buffer
	require.NoError(t, png.Encode(&b, img))
	return b.Bytes()
}

// postArtworkThroughRouter uploads data to the custom artwork route through
// the full router, so global middleware (body caps, auth) is exercised.
func postArtworkThroughRouter(t *testing.T, handler http.Handler, cookie *http.Cookie, id int64, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	part, err := mw.CreateFormFile("file", "art.png")
	require.NoError(t, err)
	_, err = part.Write(data)
	require.NoError(t, err)
	require.NoError(t, mw.Close())

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/mappings/%d/artwork/custom/poster", id), &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func newSignedInArtworkRouter(t *testing.T) (http.Handler, *http.Cookie, int64) {
	t.Helper()
	handler, db := newAuthTestRouter(t)
	rec := doJSON(t, handler, http.MethodPost, "/api/auth/setup", `{"password":"hunter2hunter2"}`, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	cookie := sessionCookie(t, rec)

	m := &models.ShowMapping{SonarrSeriesID: 5, SonarrTitle: "Show", AFLSlug: "show", Enabled: true}
	require.NoError(t, db.UpsertMapping(context.Background(), m))
	return handler, cookie, m.ID
}

// The global 1 MiB body cap must not undercut the upload route's own,
// larger limit.
func TestRouterAcceptsCustomArtworkAboveDefaultBodyCap(t *testing.T) {
	handler, cookie, id := newSignedInArtworkRouter(t)
	data := noisyPNG(t, 800, 800)
	require.Greater(t, len(data), 2<<20, "test image must be well above the 1 MiB default cap")
	require.Less(t, len(data), artwork.MaxCustomImageBytes)

	rec := postArtworkThroughRouter(t, handler, cookie, id, data)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var got models.ShowMapping
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, []string{"poster"}, got.ArtworkCustomSlots)
}

func TestRouterRejectsCustomArtworkAboveImageLimit(t *testing.T) {
	handler, cookie, id := newSignedInArtworkRouter(t)
	header := []byte("\x89PNG\r\n\x1a\n")

	cases := map[string]int{
		// Fits under the request cap (which allows for multipart framing)
		// but the image itself is over the limit.
		"just over the image limit": artwork.MaxCustomImageBytes + 1,
		// Over the request cap: cut off while reading the body.
		"far over the image limit": artwork.MaxCustomImageBytes + (2 << 20),
	}
	for name, size := range cases {
		t.Run(name, func(t *testing.T) {
			data := append(append([]byte{}, header...), make([]byte, size-len(header))...)
			rec := postArtworkThroughRouter(t, handler, cookie, id, data)
			assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code, rec.Body.String())
		})
	}
}

func TestMaxRequestBodyOnlyRaisesCapForArtworkUpload(t *testing.T) {
	cases := []struct {
		method, path string
		want         int64
	}{
		{http.MethodPost, "/api/mappings/7/artwork/custom/poster", maxCustomArtworkUploadBody},
		{http.MethodPost, "/api/mappings/7/artwork/custom/logo", maxCustomArtworkUploadBody},
		{http.MethodDelete, "/api/mappings/7/artwork/custom/poster", defaultMaxRequestBody},
		{http.MethodPost, "/api/mappings/7/artwork/custom/poster/extra", defaultMaxRequestBody},
		{http.MethodPost, "/api/mappings/7/artwork/refresh", defaultMaxRequestBody},
		{http.MethodPut, "/api/settings", defaultMaxRequestBody},
		{http.MethodPost, "/api/backup/import", defaultMaxRequestBody},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		assert.Equal(t, tc.want, maxRequestBody(req), "%s %s", tc.method, tc.path)
	}
}
