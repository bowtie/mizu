package artwork

import (
	"bytes"
	"context"
	"image"
	_ "image/jpeg" // decoders for image.Decode
	_ "image/png"
	"io"
	"math/bits"
	"net/http"
	"sync"
	"time"
)

const (
	// dedupeConcurrency bounds parallel thumbnail downloads per listing.
	dedupeConcurrency = 6
	// dedupeFetchTimeout caps one thumbnail download.
	dedupeFetchTimeout = 10 * time.Second
	// dedupeMaxBytes caps how much of one image is read for hashing.
	dedupeMaxBytes = 8 << 20
	// dedupeMaxPixels caps a thumbnail's decoded size. A small compressed
	// file can declare enormous dimensions, and decoding allocates
	// width×height×4 bytes, so the header is checked before decoding.
	// 40 MP covers any real thumbnail (TMDB originals top out near 24 MP).
	dedupeMaxPixels = 40_000_000
	// dedupeMaxDistance is the largest Hamming distance (of 64 bits) at which
	// two hashes count as the same picture. Re-uploads of one artwork at a
	// different size or compression land within 0–3; distinct artwork is
	// typically 15+ apart.
	dedupeMaxDistance = 4
)

// dedupeOptions removes repeated artwork from the posters and banners of a
// candidate listing. Providers often carry the same picture several times as
// separate files (re-uploads, different resolutions), which URL comparison
// can't see, so images are compared by a perceptual hash of their content.
//
// Logos are left alone: they are transparent, and a white and a black
// version of one logo are both worth offering.
//
// Only images in the same language are merged, so a localized poster is never
// hidden behind its English twin. Of a duplicate group the highest-resolution
// copy (then the most-liked) is kept in the position of the group's first
// member; the dropped URLs are recorded in AltURLs so a caller can still
// recognize "the image I have is this one".
//
// Best effort: an image that can't be fetched or decoded is kept as is.
func (m *Manager) dedupeOptions(ctx context.Context, opts *ArtworkOptions) {
	if opts == nil {
		return
	}
	opts.Posters = m.dedupeSlot(ctx, opts.Posters)
	opts.Banners = m.dedupeSlot(ctx, opts.Banners)
}

func (m *Manager) dedupeSlot(ctx context.Context, in []ArtworkOption) []ArtworkOption {
	if len(in) < 2 {
		return in
	}

	hashes := make([]uint64, len(in))
	ok := make([]bool, len(in))
	sem := make(chan struct{}, dedupeConcurrency)
	var wg sync.WaitGroup
	for i := range in {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			url := in[i].ThumbURL
			if url == "" {
				url = in[i].URL
			}
			hashes[i], ok[i] = m.imageHash(ctx, url)
		}(i)
	}
	wg.Wait()
	if ctx.Err() != nil {
		return in
	}

	type kept struct {
		opt  ArtworkOption
		hash uint64
		ok   bool
	}
	var out []kept
outer:
	for i, opt := range in {
		if ok[i] {
			for j := range out {
				k := &out[j]
				if !k.ok || k.opt.Lang != opt.Lang || bits.OnesCount64(k.hash^hashes[i]) > dedupeMaxDistance {
					continue
				}
				// Same picture: keep the better copy in the group's slot.
				if betterOption(opt, k.opt) {
					opt.AltURLs = append(append(opt.AltURLs, k.opt.URL), k.opt.AltURLs...)
					k.opt = opt
					k.hash = hashes[i]
				} else {
					k.opt.AltURLs = append(k.opt.AltURLs, opt.URL)
				}
				continue outer
			}
		}
		out = append(out, kept{opt: opt, hash: hashes[i], ok: ok[i]})
	}

	res := make([]ArtworkOption, len(out))
	for i, k := range out {
		res[i] = k.opt
	}
	return res
}

// betterOption reports whether a is a preferable copy of the same picture:
// more pixels first, then more likes. Equal or unknown sizes keep the
// earlier entry.
func betterOption(a, b ArtworkOption) bool {
	if pa, pb := a.Width*a.Height, b.Width*b.Height; pa != pb {
		return pa > pb
	}
	return a.Likes > b.Likes
}

// imageHash returns the perceptual hash of the image at url, caching
// successes for the life of the process.
func (m *Manager) imageHash(ctx context.Context, url string) (uint64, bool) {
	if v, hit := m.hashCache.Load(url); hit {
		return v.(uint64), true
	}
	ctx, cancel := context.WithTimeout(ctx, dedupeFetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, false
	}
	req.Header.Set("User-Agent", "Mizu/1.0")
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return 0, false
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, false
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, dedupeMaxBytes))
	if err != nil {
		return 0, false
	}
	if !decodableSize(data) {
		return 0, false
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return 0, false
	}
	h := differenceHash(img)
	m.hashCache.Store(url, h)
	return h, true
}

// decodableSize reports whether data's image header declares dimensions
// small enough to decode safely.
func decodableSize(data []byte) bool {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return false
	}
	return int64(cfg.Width)*int64(cfg.Height) <= dedupeMaxPixels
}

// differenceHash is a 64-bit dHash: the picture is reduced to a 9x8 grid of
// average brightness and each bit records whether a cell is brighter than
// its right-hand neighbour. It survives rescaling and recompression.
func differenceHash(img image.Image) uint64 {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return 0
	}

	var grid [8][9]float64
	for gy := 0; gy < 8; gy++ {
		for gx := 0; gx < 9; gx++ {
			x0, x1 := b.Min.X+gx*w/9, b.Min.X+(gx+1)*w/9
			y0, y1 := b.Min.Y+gy*h/8, b.Min.Y+(gy+1)*h/8
			if x1 <= x0 {
				x1 = x0 + 1
			}
			if y1 <= y0 {
				y1 = y0 + 1
			}
			// Sample at most ~8 points per axis; the cell average doesn't
			// need every pixel of a 1920px image.
			sx, sy := max(1, (x1-x0)/8), max(1, (y1-y0)/8)
			var sum float64
			var n int
			for y := y0; y < y1; y += sy {
				for x := x0; x < x1; x += sx {
					r, g, bl, _ := img.At(x, y).RGBA()
					sum += 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(bl)
					n++
				}
			}
			grid[gy][gx] = sum / float64(n)
		}
	}

	var hash uint64
	for gy := 0; gy < 8; gy++ {
		for gx := 0; gx < 8; gx++ {
			hash <<= 1
			if grid[gy][gx] > grid[gy][gx+1] {
				hash |= 1
			}
		}
	}
	return hash
}
