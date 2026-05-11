package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	_ "github.com/foodsea/core/ent/runtime"
	imagesinfra "github.com/foodsea/core/internal/modules/images/infrastructure"
	"github.com/foodsea/core/internal/platform/config"
	"github.com/foodsea/core/internal/platform/database"
	s3platform "github.com/foodsea/core/internal/platform/s3"
)

const (
	defaultManifest = "../../reports/generated_product_images/manifest.jsonl"
	defaultStatuses = "likely_wrong,uncertain,no_image"
)

type manifestRow struct {
	OK         bool   `json:"ok"`
	ProductID  string `json:"product_id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	OutputPath string `json:"output_path"`
	MimeType   string `json:"mime_type"`
}

type importItem struct {
	ProductID  uuid.UUID
	Name       string
	Status     string
	OutputPath string
	MimeType   string
}

type importStats struct {
	Processed int
	Uploaded  int
	Skipped   int
	Errors    int
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "import-generated-images: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		manifestPath = flag.String("manifest", defaultManifest, "Path to manifest.jsonl from generated images")
		statusesRaw  = flag.String("statuses", defaultStatuses, "Comma-separated statuses to import; use 'all' to disable filtering")
		limit        = flag.Int("limit", 0, "Optional limit for imported products")
		dryRun       = flag.Bool("dry-run", false, "Prepare import list and print summary without uploading or updating DB")
	)
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	statuses := parseStatuses(*statusesRaw)
	items, err := loadImportItems(*manifestPath, statuses, *limit)
	if err != nil {
		return err
	}

	fmt.Printf("manifest=%s\n", *manifestPath)
	fmt.Printf("statuses=%s\n", formatStatuses(statuses))
	fmt.Printf("to_import=%d\n", len(items))

	if *dryRun {
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	entClient, sqlDB, err := database.Open(ctx, cfg.DB, log)
	if err != nil {
		return err
	}
	defer func() {
		_ = sqlDB.Close()
		_ = entClient.Close()
	}()

	s3Client, err := s3platform.NewClient(ctx, s3platform.Config{
		Endpoint:        cfg.S3.Endpoint,
		AccessKeyID:     cfg.S3.AccessKeyID,
		SecretAccessKey: cfg.S3.SecretAccessKey,
		BucketName:      cfg.S3.BucketName,
		UseSSL:          cfg.S3.UseSSL,
		PublicBaseURL:   cfg.S3.PublicBaseURL,
	})
	if err != nil {
		return fmt.Errorf("init s3 client: %w", err)
	}

	repo := imagesinfra.NewProductImageRepo(entClient)
	stats := importStats{}
	total := len(items)

	for idx, item := range items {
		stats.Processed++
		if err := importOne(ctx, s3Client, repo, item); err != nil {
			stats.Errors++
			fmt.Fprintf(os.Stderr, "[%d/%d] error product_id=%s name=%q: %v\n", idx+1, total, item.ProductID, item.Name, err)
		} else {
			stats.Uploaded++
		}

		if shouldPrintProgress(idx+1, total) {
			printProgress(idx+1, total, stats)
		}
	}

	fmt.Printf("processed=%d uploaded=%d skipped=%d errors=%d\n", stats.Processed, stats.Uploaded, stats.Skipped, stats.Errors)
	if stats.Errors > 0 {
		return fmt.Errorf("completed with %d errors", stats.Errors)
	}
	return nil
}

func importOne(ctx context.Context, s3Client *s3platform.Client, repo *imagesinfra.ProductImageRepo, item importItem) error {
	file, err := os.Open(item.OutputPath)
	if err != nil {
		return fmt.Errorf("open image: %w", err)
	}
	defer file.Close()

	contentType := item.MimeType
	if contentType == "" {
		contentType = mime.TypeByExtension(strings.ToLower(filepath.Ext(item.OutputPath)))
	}
	if contentType == "" {
		contentType = "image/png"
	}

	filename := filepath.Base(item.OutputPath)
	key := fmt.Sprintf("products/%s/%s", item.ProductID, filename)

	url, err := s3Client.Upload(ctx, key, file, contentType)
	if err != nil {
		return err
	}

	if err := repo.SetImageURL(ctx, item.ProductID, url); err != nil {
		return fmt.Errorf("persist image_url: %w", err)
	}

	return nil
}

func loadImportItems(path string, statuses map[string]struct{}, limit int) ([]importItem, error) {
	manifestAbs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve manifest path: %w", err)
	}
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(manifestAbs)))

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open manifest: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	buffer := make([]byte, 0, 1024*1024)
	scanner.Buffer(buffer, 4*1024*1024)

	order := make([]string, 0)
	seen := make(map[string]struct{})
	latest := make(map[string]importItem)
	lineNo := 0

	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var row manifestRow
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return nil, fmt.Errorf("parse manifest line %d: %w", lineNo, err)
		}

		if !row.OK || row.ProductID == "" || row.OutputPath == "" {
			continue
		}
		if !matchesStatus(row.Status, statuses) {
			continue
		}

		productID, err := uuid.Parse(row.ProductID)
		if err != nil {
			return nil, fmt.Errorf("invalid product_id on line %d: %w", lineNo, err)
		}

		resolvedOutputPath, err := resolveOutputPath(repoRoot, row.OutputPath)
		if err != nil {
			return nil, fmt.Errorf("resolve output_path on line %d: %w", lineNo, err)
		}

		if _, err := os.Stat(resolvedOutputPath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("stat output_path on line %d: %w", lineNo, err)
		}

		if _, ok := seen[row.ProductID]; !ok {
			seen[row.ProductID] = struct{}{}
			order = append(order, row.ProductID)
		}

		latest[row.ProductID] = importItem{
			ProductID:  productID,
			Name:       row.Name,
			Status:     row.Status,
			OutputPath: resolvedOutputPath,
			MimeType:   row.MimeType,
		}
	}

	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("scan manifest: %w", err)
	}

	items := make([]importItem, 0, len(order))
	for _, productID := range order {
		item, ok := latest[productID]
		if !ok {
			continue
		}
		items = append(items, item)
		if limit > 0 && len(items) >= limit {
			break
		}
	}

	return items, nil
}

func resolveOutputPath(repoRoot, outputPath string) (string, error) {
	if outputPath == "" {
		return "", fmt.Errorf("empty output_path")
	}
	if filepath.IsAbs(outputPath) {
		return outputPath, nil
	}

	candidates := []string{
		outputPath,
		filepath.Join(repoRoot, outputPath),
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return filepath.Join(repoRoot, outputPath), nil
}

func parseStatuses(raw string) map[string]struct{} {
	result := make(map[string]struct{})
	for _, part := range strings.Split(raw, ",") {
		status := strings.TrimSpace(part)
		if status == "" {
			continue
		}
		if strings.EqualFold(status, "all") {
			return nil
		}
		result[status] = struct{}{}
	}
	return result
}

func matchesStatus(status string, allowed map[string]struct{}) bool {
	if allowed == nil {
		return true
	}
	_, ok := allowed[status]
	return ok
}

func formatStatuses(statuses map[string]struct{}) string {
	if statuses == nil {
		return "all"
	}
	parts := make([]string, 0, len(statuses))
	for status := range statuses {
		parts = append(parts, status)
	}
	return strings.Join(parts, ",")
}

func shouldPrintProgress(done, total int) bool {
	if done == total {
		return true
	}
	if done <= 10 {
		return true
	}
	return done%25 == 0
}

func printProgress(done, total int, stats importStats) {
	percent := 0.0
	if total > 0 {
		percent = float64(done) / float64(total) * 100
	}
	fmt.Printf("progress=%d/%d (%.1f%%) uploaded=%d errors=%d\n", done, total, percent, stats.Uploaded, stats.Errors)
}
