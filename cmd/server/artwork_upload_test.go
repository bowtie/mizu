package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mizu/internal/artwork"
	"mizu/internal/database"
	"mizu/internal/models"
)

func uploadRequest(t *testing.T, id int64, slot, field string, data []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	if data != nil {
		part, err := mw.CreateFormFile(field, "art.png")
		require.NoError(t, err)
		_, err = part.Write(data)
		require.NoError(t, err)
	}
	require.NoError(t, mw.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/mappings/x/artwork/custom/"+slot, &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", fmt.Sprintf("%d", id))
	rctx.URLParams.Add("slot", slot)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func deleteSlotRequest(id int64, slot string) *http.Request {
	req := httptest.NewRequest(http.MethodDelete, "/api/mappings/x/artwork/custom/"+slot, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", fmt.Sprintf("%d", id))
	rctx.URLParams.Add("slot", slot)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func smallPNG(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	require.NoError(t, png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 6, 9))))
	return b.Bytes()
}

func newUploadTestEnv(t *testing.T) (*database.DB, *artwork.Manager, string, int64) {
	t.Helper()
	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	artDir := filepath.Join(t.TempDir(), "artwork")
	mgr := artwork.NewManager(artDir, db)
	m := &models.ShowMapping{SonarrSeriesID: 5, SonarrTitle: "Show", AFLSlug: "show", Enabled: true}
	require.NoError(t, db.UpsertMapping(context.Background(), m))
	return db, mgr, artDir, m.ID
}

func TestUploadCustomArtworkStoresAndReportsTheSlot(t *testing.T) {
	db, mgr, artDir, id := newUploadTestEnv(t)
	data := smallPNG(t)

	rec := httptest.NewRecorder()
	handleUploadCustomArtwork(db, mgr)(rec, uploadRequest(t, id, "banner", "file", data))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var got models.ShowMapping
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, []string{"banner"}, got.ArtworkCustomSlots)
	stored, err := os.ReadFile(filepath.Join(artDir, fmt.Sprintf("%d", id), "banner.jpg"))
	require.NoError(t, err)
	assert.Equal(t, data, stored)
}

func TestUploadCustomArtworkRejectsBadRequests(t *testing.T) {
	db, mgr, _, id := newUploadTestEnv(t)
	h := handleUploadCustomArtwork(db, mgr)

	cases := []struct {
		name string
		req  *http.Request
		want int
	}{
		{"unknown slot", uploadRequest(t, id, "cover", "file", smallPNG(t)), http.StatusBadRequest},
		{"missing file field", uploadRequest(t, id, "poster", "other", smallPNG(t)), http.StatusBadRequest},
		{"no file at all", uploadRequest(t, id, "poster", "file", nil), http.StatusBadRequest},
		{"not an image", uploadRequest(t, id, "poster", "file", []byte("<script>alert(1)</script>")), http.StatusBadRequest},
		{"unknown show", uploadRequest(t, 9999, "poster", "file", smallPNG(t)), http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h(rec, tc.req)
			assert.Equal(t, tc.want, rec.Code, rec.Body.String())
		})
	}

	m, err := db.GetMappingByID(context.Background(), id)
	require.NoError(t, err)
	assert.Empty(t, m.ArtworkCustomSlots, "a rejected upload must change nothing")
}

func TestUploadCustomArtworkRejectsOversizeBody(t *testing.T) {
	db, mgr, _, id := newUploadTestEnv(t)
	huge := append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, artwork.MaxCustomImageBytes+(2<<20))...)

	rec := httptest.NewRecorder()
	handleUploadCustomArtwork(db, mgr)(rec, uploadRequest(t, id, "poster", "file", huge))
	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code, rec.Body.String())
}

func TestRemoveCustomArtworkEndpoint(t *testing.T) {
	db, mgr, artDir, id := newUploadTestEnv(t)
	rec := httptest.NewRecorder()
	handleUploadCustomArtwork(db, mgr)(rec, uploadRequest(t, id, "logo", "file", smallPNG(t)))
	require.Equal(t, http.StatusOK, rec.Code)

	rec = httptest.NewRecorder()
	handleRemoveCustomArtwork(db, mgr)(rec, deleteSlotRequest(id, "logo"))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var got models.ShowMapping
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Empty(t, got.ArtworkCustomSlots)
	_, err := os.Stat(filepath.Join(artDir, fmt.Sprintf("%d", id), "logo.png"))
	assert.True(t, os.IsNotExist(err))

	rec = httptest.NewRecorder()
	handleRemoveCustomArtwork(db, mgr)(rec, deleteSlotRequest(id, "cover"))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
