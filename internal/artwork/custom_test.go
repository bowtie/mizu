package artwork

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"mizu/internal/database"
	"mizu/internal/models"
)

func tinyImage(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 10), uint8(y * 10), 128, 255})
		}
	}
	return img
}

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	require.NoError(t, png.Encode(&b, tinyImage(w, h)))
	return b.Bytes()
}

func jpegData(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	require.NoError(t, jpeg.Encode(&b, tinyImage(8, 8), nil))
	return b.Bytes()
}

// fullProvider hands back a poster and a banner so a refresh has something
// to write to every slot a test cares about.
type fullProvider struct{ poster, banner string }

func (p *fullProvider) Name() string    { return "anilist" }
func (p *fullProvider) Available() bool { return true }
func (p *fullProvider) Fetch(context.Context, models.SonarrSeries) (*Artwork, error) {
	return &Artwork{PosterURL: p.poster, BannerURL: p.banner, Source: "anilist"}, nil
}

func newFullManager(t *testing.T, db *database.DB) (*Manager, string) {
	t.Helper()
	img := testImageServer(t, "provider-image")
	artDir := filepath.Join(t.TempDir(), "artwork")
	mgr := NewManager(artDir, db)
	mgr.Register(&fullProvider{poster: img.URL + "/poster.jpg", banner: img.URL + "/banner.jpg"})
	return mgr, artDir
}

func readSlot(t *testing.T, artDir string, id int64, file string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(artDir, fmt.Sprintf("%d", id), file))
	require.NoError(t, err)
	return string(data)
}

func TestValidateCustomImage(t *testing.T) {
	var g bytes.Buffer
	require.NoError(t, gif.Encode(&g, tinyImage(4, 4), nil))

	assert.NoError(t, validateCustomImage(pngBytes(t, 8, 8)))
	assert.NoError(t, validateCustomImage(jpegData(t)))
	assert.NoError(t, validateCustomImage(g.Bytes()))
	// WebP is accepted on its header alone (no decoder is bundled).
	assert.NoError(t, validateCustomImage([]byte("RIFF\x24\x00\x00\x00WEBPVP8 ")))

	for name, data := range map[string][]byte{
		"empty":       nil,
		"text":        []byte("definitely not an image"),
		"svg":         []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
		"html":        []byte("<html><body>hi</body></html>"),
		"truncated":   pngBytes(t, 8, 8)[:20],
		"huge pixels": pngBytes(t, maxCustomImageSide+1, 1),
		"too big":     append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, MaxCustomImageBytes)...),
	} {
		t.Run(name, func(t *testing.T) {
			err := validateCustomImage(data)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidCustomImage)
		})
	}
}

func TestSetCustomArtworkStoresSlotAndFlagsIt(t *testing.T) {
	db := newTestDB(t)
	mgr, artDir := newFullManager(t, db)
	id := insertMapping(t, db, 100)
	ctx := context.Background()

	// A provider fetch first, so there is something in the other slots.
	_, err := mgr.FetchAndStore(ctx, id, testSeries(100), "anilist")
	require.NoError(t, err)

	upload := pngBytes(t, 16, 24)
	require.NoError(t, mgr.SetCustomArtwork(ctx, id, SlotPoster, upload))

	assert.Equal(t, string(upload), readSlot(t, artDir, id, "poster.jpg"), "stored exactly as uploaded")
	assert.Equal(t, "provider-image", readSlot(t, artDir, id, "banner.jpg"), "other slots are untouched")

	m, err := db.GetMappingByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, []string{"poster"}, m.ArtworkCustomSlots)
	assert.Empty(t, m.ArtworkPosterURL, "an upload has no remote origin")
	assert.NotEmpty(t, m.ArtworkBannerURL, "other slots keep their source URL")
}

func TestSetCustomArtworkRejectsBadInput(t *testing.T) {
	db := newTestDB(t)
	mgr, _ := newFullManager(t, db)
	id := insertMapping(t, db, 100)
	ctx := context.Background()

	assert.ErrorIs(t, mgr.SetCustomArtwork(ctx, id, "cover", pngBytes(t, 4, 4)), ErrInvalidCustomImage)
	assert.ErrorIs(t, mgr.SetCustomArtwork(ctx, id, SlotPoster, []byte("nope")), ErrInvalidCustomImage)
	assert.Error(t, mgr.SetCustomArtwork(ctx, 9999, SlotPoster, pngBytes(t, 4, 4)), "unknown mapping")

	m, err := db.GetMappingByID(ctx, id)
	require.NoError(t, err)
	assert.Empty(t, m.ArtworkCustomSlots)
}

func TestRefreshLeavesCustomSlotsAlone(t *testing.T) {
	db := newTestDB(t)
	mgr, artDir := newFullManager(t, db)
	id := insertMapping(t, db, 100)
	ctx := context.Background()

	upload := pngBytes(t, 16, 24)
	require.NoError(t, mgr.SetCustomArtwork(ctx, id, SlotPoster, upload))

	res, err := mgr.FetchAndStore(ctx, id, testSeries(100), "anilist")
	require.NoError(t, err)
	assert.True(t, res.Found)

	assert.Equal(t, string(upload), readSlot(t, artDir, id, "poster.jpg"), "the upload survives a refresh")
	assert.Equal(t, "provider-image", readSlot(t, artDir, id, "banner.jpg"), "uncustomised slots still refresh")

	m, err := db.GetMappingByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, []string{"poster"}, m.ArtworkCustomSlots)
	assert.Equal(t, filepath.Join(fmt.Sprintf("%d", id), "poster.jpg"), m.ArtworkPosterPath)
	assert.Empty(t, m.ArtworkPosterURL)
	assert.NotEmpty(t, m.ArtworkBannerPath)
}

func TestSwitchingToSonarrKeepsCustomSlots(t *testing.T) {
	db := newTestDB(t)
	mgr, artDir := newFullManager(t, db)
	id := insertMapping(t, db, 100)
	ctx := context.Background()

	_, err := mgr.FetchAndStore(ctx, id, testSeries(100), "anilist")
	require.NoError(t, err)
	upload := pngBytes(t, 16, 24)
	require.NoError(t, mgr.SetCustomArtwork(ctx, id, SlotBanner, upload))

	require.NoError(t, mgr.ClearArtworkForSonarr(ctx, id))

	assert.Equal(t, string(upload), readSlot(t, artDir, id, "banner.jpg"))
	_, err = os.Stat(filepath.Join(artDir, fmt.Sprintf("%d", id), "poster.jpg"))
	assert.True(t, os.IsNotExist(err), "provider artwork is cleared for Sonarr")

	m, err := db.GetMappingByID(ctx, id)
	require.NoError(t, err)
	assert.Empty(t, m.ArtworkPosterPath)
	assert.NotEmpty(t, m.ArtworkBannerPath)
	assert.Equal(t, []string{"banner"}, m.ArtworkCustomSlots)
}

func TestRemoveCustomArtworkFreesTheSlot(t *testing.T) {
	db := newTestDB(t)
	mgr, artDir := newFullManager(t, db)
	id := insertMapping(t, db, 100)
	ctx := context.Background()

	require.NoError(t, mgr.SetCustomArtwork(ctx, id, SlotPoster, pngBytes(t, 8, 8)))
	require.NoError(t, mgr.SetCustomArtwork(ctx, id, SlotLogo, pngBytes(t, 8, 8)))
	require.NoError(t, mgr.RemoveCustomArtwork(ctx, id, SlotPoster))

	_, err := os.Stat(filepath.Join(artDir, fmt.Sprintf("%d", id), "poster.jpg"))
	assert.True(t, os.IsNotExist(err))
	m, err := db.GetMappingByID(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, []string{"logo"}, m.ArtworkCustomSlots)
	assert.Empty(t, m.ArtworkPosterPath)

	// With the flag gone, a refresh fills the slot again.
	_, err = mgr.FetchAndStore(ctx, id, testSeries(100), "anilist")
	require.NoError(t, err)
	assert.Equal(t, "provider-image", readSlot(t, artDir, id, "poster.jpg"))

	// Removing a slot that is not custom is a no-op, not an error.
	require.NoError(t, mgr.RemoveCustomArtwork(ctx, id, SlotBanner))
}

func TestPickingAProviderImageEndsCustomStatus(t *testing.T) {
	db := newTestDB(t)
	mgr, artDir := newFullManager(t, db)
	id := insertMapping(t, db, 100)
	ctx := context.Background()

	require.NoError(t, mgr.SetCustomArtwork(ctx, id, SlotPoster, pngBytes(t, 8, 8)))
	img := testImageServer(t, "picked-image")

	res, err := mgr.SelectArtwork(ctx, id, "anilist", img.URL+"/p.jpg", "", "")
	require.NoError(t, err)
	require.True(t, res.Found)

	assert.Equal(t, "picked-image", readSlot(t, artDir, id, "poster.jpg"))
	m, err := db.GetMappingByID(ctx, id)
	require.NoError(t, err)
	assert.Empty(t, m.ArtworkCustomSlots, "a picked provider image replaces the upload")
}

func TestRedactURLErrorDropsAPIKey(t *testing.T) {
	err := redactURLError(&url.Error{Op: "Get", URL: "https://webservice.fanart.tv/v3.2/tv/1?api_key=SECRET", Err: errors.New("dial tcp: i/o timeout")})
	assert.NotContains(t, err.Error(), "SECRET")
	assert.Contains(t, err.Error(), "i/o timeout")
}
