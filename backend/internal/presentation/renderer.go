package presentation

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// Renderer turns slides into PNG images or a PDF using headless Chromium.
// The same HTML/CSS template source is used for the on-screen preview, so the
// exported file matches what the admin reviewed (PRD F-07).
type Renderer struct {
	templateDir string
	chromeBin   string
	timeout     time.Duration
}

// RendererConfig configures the renderer.
type RendererConfig struct {
	TemplateDir string
	ChromeBin   string
	Timeout     time.Duration
}

// NewRenderer verifies the template assets exist.
func NewRenderer(cfg RendererConfig) (*Renderer, error) {
	if _, err := os.Stat(filepath.Join(cfg.TemplateDir, "slide.html")); err != nil {
		return nil, fmt.Errorf("renderer: slide template not found in %s: %w", cfg.TemplateDir, err)
	}
	if _, err := os.Stat(filepath.Join(cfg.TemplateDir, "slide", "style.css")); err != nil {
		return nil, fmt.Errorf("renderer: stylesheet not found in %s: %w", cfg.TemplateDir, err)
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	return &Renderer{templateDir: cfg.TemplateDir, chromeBin: cfg.ChromeBin, timeout: cfg.Timeout}, nil
}

// slideView is the data passed to the slide template.
type slideView struct {
	Title     string
	Template  string
	Note      string
	Items     []SlideItem
	PageLabel string
}

// BuildDocument renders all slides into one HTML document with the stylesheet
// inlined, so Chromium needs no network access to render it.
func (r *Renderer) BuildDocument(slides []Slide, note string) (string, error) {
	css, err := os.ReadFile(filepath.Join(r.templateDir, "slide", "style.css"))
	if err != nil {
		return "", fmt.Errorf("renderer: read stylesheet: %w", err)
	}
	tmpl, err := template.New("slide.html").ParseFiles(filepath.Join(r.templateDir, "slide.html"))
	if err != nil {
		return "", fmt.Errorf("renderer: parse template: %w", err)
	}

	var body strings.Builder
	for i, s := range slides {
		view := slideView{
			Title:     s.Title,
			Template:  s.Template,
			Note:      firstNonEmpty(s.Note, note),
			Items:     s.Items,
			PageLabel: fmt.Sprintf("%d / %d", i+1, len(slides)),
		}
		if err := tmpl.Execute(&body, view); err != nil {
			return "", fmt.Errorf("renderer: execute template: %w", err)
		}
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html><head><meta charset="utf-8">
<style>
%s
html, body { width: 1920px; height: auto; overflow: visible; background: #000000; }
.slide { position: relative; width: 1920px; height: 1080px; page-break-after: always; break-after: page; }
.slide:last-child { page-break-after: auto; break-after: auto; }
@media print {
  @page { size: 1920px 1080px; margin: 0; }
  html, body { width: 1920px; height: auto; }
}
</style></head><body>%s</body></html>`, css, body.String()), nil
}

// allocatorOptions builds Chromium flags tuned for a small VPS (R-05): no GPU,
// no sandbox, and a fixed device scale so output is pixel-exact.
func (r *Renderer) allocatorOptions() []chromedp.ExecAllocatorOption {
	opts := []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Headless,
		chromedp.DisableGPU,
		chromedp.NoSandbox,
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("hide-scrollbars", true),
		chromedp.Flag("force-device-scale-factor", "1"),
		chromedp.Flag("font-render-hinting", "none"),
		chromedp.WindowSize(1920, 1080),
	}
	if r.chromeBin != "" {
		opts = append(opts, chromedp.ExecPath(r.chromeBin))
	}
	return opts
}

// withBrowser runs fn inside a Chromium session bounded by the export timeout.
func (r *Renderer) withBrowser(ctx context.Context, fn func(ctx context.Context) error) error {
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, r.allocatorOptions()...)
	defer cancelAlloc()

	runCtx, cancelRun := chromedp.NewContext(allocCtx)
	defer cancelRun()

	timeoutCtx, cancelTimeout := context.WithTimeout(runCtx, r.timeout)
	defer cancelTimeout()

	return fn(timeoutCtx)
}

// dataURL embeds the document so Chromium loads it without a file server.
func dataURL(doc string) string {
	return "data:text/html;base64," + base64.StdEncoding.EncodeToString([]byte(doc))
}

// RenderPNG returns one PNG per slide at exactly 1920x1080.
func (r *Renderer) RenderPNG(ctx context.Context, slides []Slide, note string) ([][]byte, error) {
	if len(slides) == 0 {
		return nil, fmt.Errorf("renderer: no slides to render")
	}
	doc, err := r.BuildDocument(slides, note)
	if err != nil {
		return nil, err
	}

	shots := make([][]byte, 0, len(slides))
	err = r.withBrowser(ctx, func(bctx context.Context) error {
		// Emulate a viewport as tall as the whole stack so every slide is
		// painted, then clip each slide out by vertical offset.
		height := int64(len(slides)) * 1080
		if err := chromedp.Do(bctx,
			chromedp.EmulateViewport(1920, height, chromedp.EmulateScale(1)),
			chromedp.Navigate(dataURL(doc)),
			chromedp.WaitReady(chromedp.CSS("body")),
			chromedp.Sleep(400*time.Millisecond),
		); err != nil {
			return fmt.Errorf("renderer: load document: %w", err)
		}

		for i := range slides {
			y := float64(i * 1080)
			capture := func(ctx context.Context, t *chromedp.Target) ([]byte, error) {
				res, err := cdp.Call(ctx, t, page.CaptureScreenshot, page.CaptureScreenshotParams{
					Format:                page.CaptureScreenshotFormatPng,
					CaptureBeyondViewport: boolPtr(true),
					Clip: &page.Viewport{
						X: 0, Y: y, Width: 1920, Height: 1080, Scale: 1,
					},
				})
				if err != nil {
					return nil, err
				}
				return res.Data, nil
			}

			buf, err := chromedp.Run(bctx, capture)
			if err != nil {
				return fmt.Errorf("renderer: capture slide %d: %w", i+1, err)
			}
			shots = append(shots, buf)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return shots, nil
}

// RenderPDF returns a single PDF with one page per slide.
func (r *Renderer) RenderPDF(ctx context.Context, slides []Slide, note string) ([]byte, error) {
	if len(slides) == 0 {
		return nil, fmt.Errorf("renderer: no slides to render")
	}
	doc, err := r.BuildDocument(slides, note)
	if err != nil {
		return nil, err
	}

	var pdf []byte
	err = r.withBrowser(ctx, func(bctx context.Context) error {
		if err := chromedp.Do(bctx,
			chromedp.Navigate(dataURL(doc)),
			chromedp.WaitReady(chromedp.CSS("body")),
			chromedp.Sleep(400*time.Millisecond),
		); err != nil {
			return fmt.Errorf("renderer: load document: %w", err)
		}

		out, err := chromedp.Run(bctx, chromedp.PrintToPDF(
			chromedp.PDFPrintBackground(),
			chromedp.PDFPreferCSSPageSize(),
			chromedp.PDFMargins(0, 0, 0, 0),
		))
		if err != nil {
			return fmt.Errorf("renderer: print to pdf: %w", err)
		}
		pdf = out
		return nil
	})
	if err != nil {
		return nil, err
	}
	return pdf, nil
}

// ZipPNGs bundles per-slide PNGs into a single archive (PRD F-07: one file per
// slide, zipped).
func ZipPNGs(files [][]byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for i, data := range files {
		w, err := zw.Create(fmt.Sprintf("slide-%02d.png", i+1))
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(data); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func boolPtr(b bool) *bool { return &b }
