package artwork

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// scene draws a deterministic picture whose layout depends on seed, at any
// size: the same seed at two sizes is "the same artwork", a different seed
// is different artwork.
func scene(w, h, seed int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			fx, fy := float64(x)/float64(w), float64(y)/float64(h)
			var v uint8
			switch seed {
			case 1: // bright left, dark right
				v = uint8(255 * (1 - fx))
			case 2: // bright top, dark bottom
				v = uint8(255 * (1 - fy))
			default: // bright centre blob
				dx, dy := fx-0.5, fy-0.5
				if dx*dx+dy*dy < 0.08 {
					v = 255
				}
			}
			img.Set(x, y, color.RGBA{v, v, v, 255})
		}
	}
	return img
}

func jpegBytes(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}))
	return buf.Bytes()
}

func dedupeTestManager(t *testing.T) (*Manager, string) {
	t.Helper()
	files := map[string][]byte{
		"/a-big.jpg":   jpegBytes(t, scene(400, 600, 1)),
		"/a-small.jpg": jpegBytes(t, scene(200, 300, 1)),
		"/b.jpg":       jpegBytes(t, scene(400, 600, 2)),
		"/c.jpg":       jpegBytes(t, scene(400, 600, 3)),
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(data)
	}))
	t.Cleanup(srv.Close)
	return &Manager{httpClient: srv.Client()}, srv.URL
}

func TestDedupeMergesSameArtworkAndKeepsHighestResolution(t *testing.T) {
	m, base := dedupeTestManager(t)
	opts := &ArtworkOptions{Posters: []ArtworkOption{
		{URL: base + "/a-small.jpg", Lang: "en", Width: 200, Height: 300},
		{URL: base + "/b.jpg", Lang: "en", Width: 400, Height: 600},
		{URL: base + "/a-big.jpg", Lang: "en", Width: 400, Height: 600},
	}}

	m.dedupeOptions(context.Background(), opts)

	require.Len(t, opts.Posters, 2)
	// The group keeps its first member's position but the better copy.
	assert.Equal(t, base+"/a-big.jpg", opts.Posters[0].URL)
	assert.Equal(t, []string{base + "/a-small.jpg"}, opts.Posters[0].AltURLs)
	assert.Equal(t, base+"/b.jpg", opts.Posters[1].URL)
	assert.Empty(t, opts.Posters[1].AltURLs)
}

func TestDedupeKeepsDifferentArtwork(t *testing.T) {
	m, base := dedupeTestManager(t)
	opts := &ArtworkOptions{Banners: []ArtworkOption{
		{URL: base + "/a-big.jpg"}, {URL: base + "/b.jpg"}, {URL: base + "/c.jpg"},
	}}
	m.dedupeOptions(context.Background(), opts)
	assert.Len(t, opts.Banners, 3)
}

func TestDedupeNeverMergesAcrossLanguages(t *testing.T) {
	m, base := dedupeTestManager(t)
	opts := &ArtworkOptions{Posters: []ArtworkOption{
		{URL: base + "/a-big.jpg", Lang: "en"},
		{URL: base + "/a-small.jpg", Lang: "ja"},
	}}
	m.dedupeOptions(context.Background(), opts)
	assert.Len(t, opts.Posters, 2, "a localized copy must stay selectable")
}

func TestDedupeLeavesLogosAlone(t *testing.T) {
	m, base := dedupeTestManager(t)
	opts := &ArtworkOptions{Logos: []ArtworkOption{
		{URL: base + "/a-big.jpg"}, {URL: base + "/a-small.jpg"},
	}}
	m.dedupeOptions(context.Background(), opts)
	assert.Len(t, opts.Logos, 2)
}

func TestDedupeKeepsImagesItCannotFetch(t *testing.T) {
	m, base := dedupeTestManager(t)
	opts := &ArtworkOptions{Posters: []ArtworkOption{
		{URL: base + "/missing-1.jpg"}, {URL: base + "/missing-2.jpg"}, {URL: base + "/a-big.jpg"},
	}}
	m.dedupeOptions(context.Background(), opts)
	assert.Len(t, opts.Posters, 3, "an unfetchable image is never silently dropped")
}

func TestDedupeFetchesEachThumbnailOnce(t *testing.T) {
	m, base := dedupeTestManager(t)
	opts := func() *ArtworkOptions {
		return &ArtworkOptions{Posters: []ArtworkOption{{URL: base + "/a-big.jpg"}, {URL: base + "/b.jpg"}}}
	}
	m.dedupeOptions(context.Background(), opts())

	// A second listing must come from the hash cache even with the server gone.
	m.httpClient = &http.Client{Transport: failingTransport{}}
	second := opts()
	m.dedupeOptions(context.Background(), second)
	assert.Len(t, second.Posters, 2)
}

type failingTransport struct{}

func (failingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, assert.AnError
}
