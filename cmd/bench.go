package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"ismartcoding/plainnas/internal/consts"
	"ismartcoding/plainnas/internal/db"
	"ismartcoding/plainnas/internal/media"
	"ismartcoding/plainnas/internal/search"

	"github.com/spf13/cobra"
)

var benchCmd = &cobra.Command{
	Use:   "bench",
	Short: "Run local performance benchmarks (index, search, thumbnails, memory)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := parseBenchConfig(cmd)
		if err != nil {
			return err
		}

		res, err := runBench(cfg)
		if err != nil {
			return err
		}

		if cfg.jsonOut {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(res)
		}

		printBenchHuman(res)
		return nil
	},
}

func init() {
	benchCmd.Flags().String("dataset", "", "Existing dataset root to benchmark (if empty, generate a temporary dataset)")
	benchCmd.Flags().Int("files", 20000, "Number of small files to generate when creating a dataset")
	benchCmd.Flags().Int("subdirs", 50, "Number of sub-directories to spread generated files across")
	benchCmd.Flags().Int("file-bytes", 1024, "Size of each generated small file in bytes")
	benchCmd.Flags().Int("images", 200, "Number of JPEG images to generate when creating a dataset")
	benchCmd.Flags().Int("image-w", 1920, "Generated image width")
	benchCmd.Flags().Int("image-h", 1080, "Generated image height")

	benchCmd.Flags().String("data-dir", "", "Override PlainNAS data dir for the bench run (default: temp dir)")
	benchCmd.Flags().Bool("keep", false, "Keep generated dataset and data dir (do not delete temp dirs)")

	benchCmd.Flags().Int("thumb-size", 320, "Thumbnail target size (width), height is auto")
	benchCmd.Flags().Int("thumb-quality", 80, "Thumbnail quality (1-100)")
	benchCmd.Flags().Int("thumb-concurrency", 0, "Thumbnail benchmark concurrency (0 = 2*GOMAXPROCS)")

	benchCmd.Flags().Int("queries", 1000, "Number of search queries to execute")
	benchCmd.Flags().Int("limit", 50, "Search result limit per query")
	benchCmd.Flags().Bool("show-hidden", false, "Include dotfiles in indexing")

	benchCmd.Flags().Bool("json", false, "Output machine-readable JSON")
}

type benchConfig struct {
	datasetRoot string
	genFiles    int
	genSubdirs  int
	fileBytes   int
	genImages   int
	imgW        int
	imgH        int
	dataDir     string
	keep        bool
	thumbSize   int
	thumbQ      int
	thumbConc   int
	queries     int
	limit       int
	showHidden  bool
	jsonOut     bool
}

type benchMem struct {
	RSSBytes   uint64 `json:"rssBytes"`
	HeapAlloc  uint64 `json:"heapAlloc"`
	HeapSys    uint64 `json:"heapSys"`
	Sys        uint64 `json:"sys"`
	NumGC      uint32 `json:"numGC"`
	Goroutines int    `json:"goroutines"`
}

type benchPhase struct {
	Name         string        `json:"name"`
	Duration     time.Duration `json:"duration"`
	Ops          int           `json:"ops"`
	OpsPerSecond float64       `json:"opsPerSecond"`
	MemBefore    benchMem      `json:"memBefore"`
	MemAfter     benchMem      `json:"memAfter"`
	Extra        any           `json:"extra,omitempty"`
	Err          string        `json:"err,omitempty"`
}

type benchResult struct {
	WhenUTC  string       `json:"whenUtc"`
	Go       string       `json:"go"`
	GOOS     string       `json:"goos"`
	GOARCH   string       `json:"goarch"`
	CPU      string       `json:"cpu"`
	Procs    int          `json:"gomaxprocs"`
	Dataset  string       `json:"dataset"`
	DataDir  string       `json:"dataDir"`
	Phases   []benchPhase `json:"phases"`
	Warnings []string     `json:"warnings,omitempty"`
}

func parseBenchConfig(cmd *cobra.Command) (benchConfig, error) {
	getInt := func(name string) int {
		v, _ := cmd.Flags().GetInt(name)
		return v
	}
	getBool := func(name string) bool {
		v, _ := cmd.Flags().GetBool(name)
		return v
	}
	getString := func(name string) string {
		v, _ := cmd.Flags().GetString(name)
		return v
	}

	cfg := benchConfig{
		datasetRoot: getString("dataset"),
		genFiles:    getInt("files"),
		genSubdirs:  getInt("subdirs"),
		fileBytes:   getInt("file-bytes"),
		genImages:   getInt("images"),
		imgW:        getInt("image-w"),
		imgH:        getInt("image-h"),
		dataDir:     getString("data-dir"),
		keep:        getBool("keep"),
		thumbSize:   getInt("thumb-size"),
		thumbQ:      getInt("thumb-quality"),
		thumbConc:   getInt("thumb-concurrency"),
		queries:     getInt("queries"),
		limit:       getInt("limit"),
		showHidden:  getBool("show-hidden"),
		jsonOut:     getBool("json"),
	}

	if cfg.genFiles < 0 || cfg.genImages < 0 {
		return benchConfig{}, fmt.Errorf("invalid --files/--images")
	}
	if cfg.genSubdirs <= 0 {
		cfg.genSubdirs = 1
	}
	if cfg.fileBytes < 0 {
		return benchConfig{}, fmt.Errorf("invalid --file-bytes")
	}
	if cfg.thumbSize <= 0 {
		return benchConfig{}, fmt.Errorf("invalid --thumb-size")
	}
	if cfg.thumbQ < 1 || cfg.thumbQ > 100 {
		return benchConfig{}, fmt.Errorf("invalid --thumb-quality")
	}
	if cfg.queries < 0 {
		return benchConfig{}, fmt.Errorf("invalid --queries")
	}
	if cfg.limit <= 0 {
		cfg.limit = 50
	}
	if cfg.thumbConc <= 0 {
		cfg.thumbConc = 2 * runtime.GOMAXPROCS(0)
		if cfg.thumbConc < 1 {
			cfg.thumbConc = 1
		}
	}

	return cfg, nil
}

func runBench(cfg benchConfig) (*benchResult, error) {
	res := &benchResult{
		WhenUTC: time.Now().UTC().Format(time.RFC3339),
		Go:      runtime.Version(),
		GOOS:    runtime.GOOS,
		GOARCH:  runtime.GOARCH,
		CPU:     detectCPUModel(),
		Procs:   runtime.GOMAXPROCS(0),
		Phases:  []benchPhase{},
	}

	datasetRoot, datasetCleanup, err := ensureDataset(cfg)
	if err != nil {
		return nil, err
	}
	defer func() {
		if datasetCleanup != nil {
			_ = datasetCleanup()
		}
	}()
	res.Dataset = datasetRoot

	dataDir, dataCleanup, err := ensureDataDir(cfg)
	if err != nil {
		return nil, err
	}
	defer func() {
		if dataCleanup != nil {
			_ = dataCleanup()
		}
	}()
	res.DataDir = dataDir

	// Important: set DATA_DIR before opening Pebble.
	consts.DATA_DIR = dataDir

	// Phase 0: warmup mem snapshot
	res.Phases = append(res.Phases, benchPhase{
		Name:      "mem.baseline",
		Duration:  0,
		Ops:       0,
		MemBefore: readBenchMem(),
		MemAfter:  readBenchMem(),
	})

	// Phase 1: build search index
	phase := runPhase("index.build", func() (int, any, error) {
		start := time.Now()
		_ = start
		err := search.IndexPaths([]string{datasetRoot}, cfg.showHidden)
		if err != nil {
			return 0, nil, err
		}
		idxSize := dirSizeBytes(filepath.Join(dataDir, "searchidx"))
		pebSize := dirSizeBytes(filepath.Join(dataDir, "pebble"))
		return 1, map[string]any{
			"searchIdxBytes": idxSize,
			"pebbleBytes":    pebSize,
		}, nil
	})
	res.Phases = append(res.Phases, phase)

	// Phase 2: search queries
	queriesPhase := runPhase("index.search", func() (int, any, error) {
		if cfg.queries == 0 {
			return 0, nil, nil
		}
		queries := buildSearchQueries(datasetRoot, cfg.queries)
		if len(queries) == 0 {
			return 0, nil, errors.New("no queries built from dataset")
		}
		lat := make([]time.Duration, 0, len(queries))
		for _, q := range queries {
			t0 := time.Now()
			_, err := search.SearchIndex(q, filepath.ToSlash(datasetRoot), 0, cfg.limit, "", 0)
			lat = append(lat, time.Since(t0))
			if err != nil {
				return len(lat), nil, err
			}
		}
		p50 := percentile(lat, 50)
		p95 := percentile(lat, 95)
		avg := avgDuration(lat)
		return len(lat), map[string]any{
			"avgMs": float64(avg.Microseconds()) / 1000.0,
			"p50Ms": float64(p50.Microseconds()) / 1000.0,
			"p95Ms": float64(p95.Microseconds()) / 1000.0,
		}, nil
	})
	res.Phases = append(res.Phases, queriesPhase)

	// Phase 3: thumbnails
	thumbPhase := runPhase("thumb.generate", func() (int, any, error) {
		imgs := listImages(datasetRoot, 20000)
		if len(imgs) == 0 {
			return 0, map[string]any{"skipped": true, "reason": "no images found"}, nil
		}
		if _, err := exec.LookPath("vipsthumbnail"); err != nil {
			res.Warnings = append(res.Warnings, "vipsthumbnail not found; skipping thumbnail benchmark (install libvips-tools)")
			return 0, map[string]any{"skipped": true, "reason": "vipsthumbnail not found"}, nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()

		lat := make([]time.Duration, len(imgs))
		outBytes := make([]int, len(imgs))
		workCh := make(chan int)
		errCh := make(chan error, 1)
		wg := sync.WaitGroup{}
		workers := cfg.thumbConc
		if workers < 1 {
			workers = 1
		}

		wg.Add(workers)
		for w := 0; w < workers; w++ {
			go func() {
				defer wg.Done()
				for i := range workCh {
					select {
					case <-ctx.Done():
						return
					default:
					}
					t0 := time.Now()
					b, _, err := media.GenerateThumbnail(imgs[i], cfg.thumbSize, 0, cfg.thumbQ, false)
					lat[i] = time.Since(t0)
					if err != nil {
						select {
						case errCh <- err:
						default:
						}
						return
					}
					outBytes[i] = len(b)
					if len(b) == 0 {
						select {
						case errCh <- errors.New("empty thumbnail"):
						default:
						}
						return
					}
				}
			}()
		}

		for i := range imgs {
			workCh <- i
		}
		close(workCh)
		wg.Wait()

		select {
		case err := <-errCh:
			return 0, nil, err
		default:
		}

		p50 := percentile(lat, 50)
		p95 := percentile(lat, 95)
		avg := avgDuration(lat)
		avgBytes := avgInt(outBytes)

		return len(imgs), map[string]any{
			"thumbSize":   cfg.thumbSize,
			"thumbQ":      cfg.thumbQ,
			"concurrency": workers,
			"avgMs":       float64(avg.Microseconds()) / 1000.0,
			"p50Ms":       float64(p50.Microseconds()) / 1000.0,
			"p95Ms":       float64(p95.Microseconds()) / 1000.0,
			"avgBytes":    avgBytes,
		}, nil
	})
	res.Phases = append(res.Phases, thumbPhase)

	// Best-effort close DB to flush.
	_ = db.GetDefault().Close()

	return res, nil
}

func runPhase(name string, fn func() (ops int, extra any, err error)) benchPhase {
	runtime.GC()
	before := readBenchMem()
	start := time.Now()
	ops, extra, err := fn()
	dur := time.Since(start)
	after := readBenchMem()

	p := benchPhase{Name: name, Duration: dur, Ops: ops, MemBefore: before, MemAfter: after, Extra: extra}
	if dur > 0 && ops > 0 {
		p.OpsPerSecond = float64(ops) / dur.Seconds()
	}
	if err != nil {
		p.Err = err.Error()
	}
	return p
}

func ensureDataset(cfg benchConfig) (root string, cleanup func() error, err error) {
	if cfg.datasetRoot != "" {
		st, err := os.Stat(cfg.datasetRoot)
		if err != nil {
			return "", nil, err
		}
		if !st.IsDir() {
			return "", nil, fmt.Errorf("dataset is not a directory")
		}
		return cfg.datasetRoot, nil, nil
	}

	root, err = os.MkdirTemp("", "plainnas-bench-dataset-")
	if err != nil {
		return "", nil, err
	}

	if err := generateDataset(root, cfg); err != nil {
		_ = os.RemoveAll(root)
		return "", nil, err
	}

	if cfg.keep {
		return root, nil, nil
	}
	return root, func() error { return os.RemoveAll(root) }, nil
}

func ensureDataDir(cfg benchConfig) (dir string, cleanup func() error, err error) {
	if cfg.dataDir != "" {
		if err := os.MkdirAll(cfg.dataDir, 0o755); err != nil {
			return "", nil, err
		}
		if cfg.keep {
			return cfg.dataDir, nil, nil
		}
		// For safety: if user explicitly sets --data-dir, do not delete it.
		return cfg.dataDir, nil, nil
	}
	p, err := os.MkdirTemp("", "plainnas-bench-data-")
	if err != nil {
		return "", nil, err
	}
	if cfg.keep {
		return p, nil, nil
	}
	return p, func() error { return os.RemoveAll(p) }, nil
}

func generateDataset(root string, cfg benchConfig) error {
	rng := rand.New(rand.NewSource(1))

	subdirs := make([]string, 0, cfg.genSubdirs)
	for i := 0; i < cfg.genSubdirs; i++ {
		d := filepath.Join(root, fmt.Sprintf("d%03d", i))
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
		subdirs = append(subdirs, d)
	}

	// Small files
	buf := make([]byte, cfg.fileBytes)
	for i := 0; i < len(buf); i++ {
		buf[i] = byte(rng.Intn(256))
	}

	for i := 0; i < cfg.genFiles; i++ {
		d := subdirs[i%len(subdirs)]
		name := fmt.Sprintf("file-%06d-%s.txt", i, randToken(rng, 10))
		p := filepath.Join(d, name)
		if err := os.WriteFile(p, buf, 0o644); err != nil {
			return err
		}
	}

	// Images
	imgDir := filepath.Join(root, "images")
	if err := os.MkdirAll(imgDir, 0o755); err != nil {
		return err
	}
	for i := 0; i < cfg.genImages; i++ {
		p := filepath.Join(imgDir, fmt.Sprintf("img-%06d-%s.jpg", i, randToken(rng, 8)))
		if err := writeJPEG(p, cfg.imgW, cfg.imgH, rng.Int63()); err != nil {
			return err
		}
	}

	// A few non-trivial names for tokenization realism (ASCII-only).
	_ = os.WriteFile(filepath.Join(imgDir, "album-test-001.jpg"), mustJPEGBytes(640, 360), 0o644)
	_ = os.WriteFile(filepath.Join(imgDir, "photo_beijing_winter.jpg"), mustJPEGBytes(640, 360), 0o644)
	return nil
}

func randToken(rng *rand.Rand, n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rng.Intn(len(letters))]
	}
	return string(b)
}

func writeJPEG(path string, w, h int, seed int64) error {
	if w <= 0 || h <= 0 {
		return fmt.Errorf("invalid image dims")
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	// Deterministic pseudo-pattern to avoid super-high compression ratios.
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := uint8((int64(x*131+y*17) + seed) % 251)
			img.SetRGBA(x, y, color.RGBA{R: v, G: uint8((int(v) * 7) % 255), B: uint8((int(v) * 13) % 255), A: 255})
		}
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	bw := bufio.NewWriterSize(f, 1<<20)
	defer bw.Flush()

	opt := &jpeg.Options{Quality: 92}
	return jpeg.Encode(bw, img, opt)
}

func mustJPEGBytes(w, h int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := uint8((x*7 + y*11) % 251)
			img.SetRGBA(x, y, color.RGBA{R: v, G: uint8((int(v) * 3) % 255), B: uint8((int(v) * 5) % 255), A: 255})
		}
	}
	b := &strings.Builder{}
	_ = b
	// Use temp file to avoid extra deps; this is only for tiny seeds.
	tmp, _ := os.CreateTemp("", "plainnas-jpeg-*.jpg")
	if tmp == nil {
		return nil
	}
	_ = jpeg.Encode(tmp, img, &jpeg.Options{Quality: 90})
	_ = tmp.Close()
	out, _ := os.ReadFile(tmp.Name())
	_ = os.Remove(tmp.Name())
	return out
}

func listImages(root string, limit int) []string {
	out := make([]string, 0, 1024)
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		switch ext {
		case ".jpg", ".jpeg", ".png", ".webp", ".gif":
			out = append(out, p)
			if limit > 0 && len(out) >= limit {
				return errors.New("limit")
			}
		}
		return nil
	})
	if len(out) > 0 && limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func buildSearchQueries(root string, n int) []string {
	// Sample file names and build token-like queries.
	sample := make([]string, 0, 512)
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if len(name) < 3 {
			return nil
		}
		sample = append(sample, name)
		if len(sample) >= 500 {
			return errors.New("limit")
		}
		return nil
	})
	if len(sample) == 0 {
		return nil
	}

	rng := rand.New(rand.NewSource(2))
	out := make([]string, 0, n)
	for len(out) < n {
		name := sample[rng.Intn(len(sample))]
		base := strings.TrimSuffix(name, filepath.Ext(name))
		base = strings.Trim(base, " -_")
		if len(base) < 3 {
			continue
		}
		// Use a small substring to mimic partial search.
		start := rng.Intn(len(base))
		end := start + 4
		if end > len(base) {
			end = len(base)
		}
		s := base[start:end]
		s = strings.Trim(s, " -_")
		if len(strings.TrimSpace(s)) < 2 {
			continue
		}
		out = append(out, s)
	}
	return out
}

func percentile(d []time.Duration, p int) time.Duration {
	if len(d) == 0 {
		return 0
	}
	cp := append([]time.Duration(nil), d...)
	sort.Slice(cp, func(i, j int) bool { return cp[i] < cp[j] })
	if p <= 0 {
		return cp[0]
	}
	if p >= 100 {
		return cp[len(cp)-1]
	}
	idx := (len(cp) - 1) * p / 100
	return cp[idx]
}

func avgDuration(d []time.Duration) time.Duration {
	if len(d) == 0 {
		return 0
	}
	var sum int64
	for _, v := range d {
		sum += int64(v)
	}
	return time.Duration(sum / int64(len(d)))
}

func avgInt(v []int) int {
	if len(v) == 0 {
		return 0
	}
	var sum int64
	for _, x := range v {
		sum += int64(x)
	}
	return int(sum / int64(len(v)))
}

func readBenchMem() benchMem {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	m := benchMem{
		RSSBytes:   readRSSBytesProc(),
		HeapAlloc:  ms.HeapAlloc,
		HeapSys:    ms.HeapSys,
		Sys:        ms.Sys,
		NumGC:      ms.NumGC,
		Goroutines: runtime.NumGoroutine(),
	}
	return m
}

func readRSSBytesProc() uint64 {
	b, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0
	}
	lines := strings.Split(string(b), "\n")
	for _, ln := range lines {
		if strings.HasPrefix(ln, "VmRSS:") {
			fields := strings.Fields(ln)
			if len(fields) >= 2 {
				kb, _ := strconv.ParseUint(fields[1], 10, 64)
				return kb * 1024
			}
		}
	}
	return 0
}

func dirSizeBytes(root string) uint64 {
	var total uint64
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		st, err := os.Stat(p)
		if err != nil {
			return nil
		}
		total += uint64(st.Size())
		return nil
	})
	return total
}

func detectCPUModel() string {
	b, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	for _, ln := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(strings.ToLower(ln), "model name") {
			if i := strings.Index(ln, ":"); i >= 0 {
				return strings.TrimSpace(ln[i+1:])
			}
		}
	}
	return ""
}

func fmtBytes(n uint64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case n >= GB:
		return fmt.Sprintf("%.2fGiB", float64(n)/float64(GB))
	case n >= MB:
		return fmt.Sprintf("%.2fMiB", float64(n)/float64(MB))
	case n >= KB:
		return fmt.Sprintf("%.2fKiB", float64(n)/float64(KB))
	default:
		return fmt.Sprintf("%dB", n)
	}
}

func printBenchHuman(res *benchResult) {
	fmt.Printf("PlainNAS Bench (%s)\n", res.WhenUTC)
	fmt.Printf("Go=%s  GOOS=%s  GOARCH=%s  GOMAXPROCS=%d\n", res.Go, res.GOOS, res.GOARCH, res.Procs)
	if res.CPU != "" {
		fmt.Printf("CPU=%s\n", res.CPU)
	}
	fmt.Printf("Dataset=%s\n", res.Dataset)
	fmt.Printf("DataDir=%s\n", res.DataDir)
	if len(res.Warnings) > 0 {
		for _, w := range res.Warnings {
			fmt.Printf("WARN: %s\n", w)
		}
	}
	fmt.Println()

	for _, p := range res.Phases {
		if p.Err != "" {
			fmt.Printf("[%s] ERROR: %s\n", p.Name, p.Err)
			continue
		}
		if p.Duration == 0 && p.Ops == 0 {
			fmt.Printf("[%s] rss=%s heap=%s goroutines=%d\n", p.Name, fmtBytes(p.MemAfter.RSSBytes), fmtBytes(p.MemAfter.HeapAlloc), p.MemAfter.Goroutines)
			continue
		}
		fmt.Printf("[%s] %s", p.Name, p.Duration)
		if p.Ops > 0 {
			fmt.Printf("  ops=%d  ops/s=%.2f", p.Ops, p.OpsPerSecond)
		}
		fmt.Printf("  rss=%s→%s  heap=%s→%s\n", fmtBytes(p.MemBefore.RSSBytes), fmtBytes(p.MemAfter.RSSBytes), fmtBytes(p.MemBefore.HeapAlloc), fmtBytes(p.MemAfter.HeapAlloc))
		if m, ok := p.Extra.(map[string]any); ok && len(m) > 0 {
			keys := make([]string, 0, len(m))
			for k := range m {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Printf("  - %s: %v\n", k, m[k])
			}
		}
	}

	fmt.Println()
	fmt.Println("Tips:")
	fmt.Println("  - For realistic thumbnail numbers, install libvips-tools (vipsthumbnail).")
	fmt.Println("  - Re-run with --json to paste results into docs/README.")
}
