package media

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func writeTestJPEG(path string, w, h int, quality int) error {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	// Deterministic gradient-ish pattern (no rand allocations).
	for y := 0; y < h; y++ {
		row := y * img.Stride
		for x := 0; x < w; x++ {
			i := row + x*4
			img.Pix[i+0] = uint8((x + y) & 0xff)
			img.Pix[i+1] = uint8((x*3 + y*7) & 0xff)
			img.Pix[i+2] = uint8((x*11 + y*13) & 0xff)
			img.Pix[i+3] = 0xff
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0644)
}

func writeTestPNG(path string, w, h int) error {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: uint8(x ^ y), A: 0xff})
		}
	}

	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&buf, img); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0644)
}

func benchGenerateThumbnail(b *testing.B, srcPath string, tw, th, q int, convertToJPEG bool, disableVips bool) {
	b.Helper()
	b.ReportAllocs()

	old := os.Getenv("PLAINNAS_DISABLE_VIPS")
	if disableVips {
		_ = os.Setenv("PLAINNAS_DISABLE_VIPS", "1")
	} else {
		_ = os.Unsetenv("PLAINNAS_DISABLE_VIPS")
	}
	b.Cleanup(func() {
		if old == "" {
			_ = os.Unsetenv("PLAINNAS_DISABLE_VIPS")
			return
		}
		_ = os.Setenv("PLAINNAS_DISABLE_VIPS", old)
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data, _, err := GenerateThumbnail(srcPath, tw, th, q, convertToJPEG)
		if err != nil {
			b.Fatal(err)
		}
		if len(data) == 0 {
			b.Fatal("empty thumbnail")
		}
	}
}

func BenchmarkGenerateThumbnail_PureGo_JPEG(b *testing.B) {
	tmp := b.TempDir()
	src := filepath.Join(tmp, "src.jpg")
	if err := writeTestJPEG(src, 4000, 3000, 90); err != nil {
		b.Fatal(err)
	}
	benchGenerateThumbnail(b, src, 320, 0, 80, false, true)
}

func BenchmarkGenerateThumbnail_PureGo_PNG(b *testing.B) {
	tmp := b.TempDir()
	src := filepath.Join(tmp, "src.png")
	if err := writeTestPNG(src, 2500, 1800); err != nil {
		b.Fatal(err)
	}
	benchGenerateThumbnail(b, src, 320, 0, 80, false, true)
}

func BenchmarkGenerateThumbnail_Vips_JPEG(b *testing.B) {
	if _, err := exec.LookPath("vips"); err != nil {
		b.Skip("vips not installed")
	}
	tmp := b.TempDir()
	src := filepath.Join(tmp, "src.jpg")
	if err := writeTestJPEG(src, 4000, 3000, 90); err != nil {
		b.Fatal(err)
	}
	benchGenerateThumbnail(b, src, 320, 0, 80, false, false)
}

func BenchmarkGenerateThumbnail_Vips_PNG(b *testing.B) {
	if _, err := exec.LookPath("vips"); err != nil {
		b.Skip("vips not installed")
	}
	tmp := b.TempDir()
	src := filepath.Join(tmp, "src.png")
	if err := writeTestPNG(src, 2500, 1800); err != nil {
		b.Fatal(err)
	}
	benchGenerateThumbnail(b, src, 320, 0, 80, false, false)
}

func BenchmarkGenerateThumbnail_Vips_CommandOverhead(b *testing.B) {
	// Measures only vips process spawn + minimal work. Useful to estimate floor.
	if _, err := exec.LookPath("vips"); err != nil {
		b.Skip("vips not installed")
	}
	if envTruthy("PLAINNAS_DISABLE_VIPS") {
		b.Skip("vips disabled via env")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := exec.Command("vips", "--version")
		if out, err := cmd.CombinedOutput(); err != nil || len(out) == 0 {
			b.Fatal(err)
		}
	}
}
