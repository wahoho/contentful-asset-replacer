package main

import (
	"contentful-asset-replacer/contentful"
	"context"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	csvPath := flag.String("csv", "id.csv", "Path to CSV file containing only an 'id' column (first column)")
	token := flag.String("token", os.Getenv("API_TOKEN"), "Bearer token to use for Authorization header (or set API_TOKEN env var)")
	headerName := flag.String("auth-header", "Authorization", "Authorization header name")
	scheme := flag.String("scheme", "Bearer", "Authorization scheme prefix, e.g. Bearer")
	environment := flag.String("environment", "testing_env", "Environment to use for the base URL")
	spaceID := flag.String("space-id", os.Getenv("SPACE_ID"), "Contentful space ID (or set SPACE_ID env var)")
	timeout := flag.Duration("timeout", 20*time.Second, "HTTP client timeout")
	flag.Parse()

	if strings.TrimSpace(*csvPath) == "" {
		fatalf("missing -csv <path> argument")
	}
	if token == nil || strings.TrimSpace(*token) == "" {
		fatalf("missing token: provide -token or set API_TOKEN env var")
	}
	if strings.TrimSpace(*spaceID) == "" {
		fatalf("missing -space-id argument or SPACE_ID environment variable")
	}

	file, err := os.Open(*csvPath)
	if err != nil {
		fatalf("open csv: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true

	// Prepare success and failed CSV outputs (append mode)
	successF, err := os.OpenFile("success.csv", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fatalf("open success.csv: %v", err)
	}
	defer successF.Close()
	successW := csv.NewWriter(successF)
	defer successW.Flush()

	// Check if success.csv is empty and write header if needed
	if stat, err := successF.Stat(); err == nil && stat.Size() == 0 {
		_ = successW.Write([]string{"entry_id", "old_asset_id", "new_asset_id"})
	}

	failedF, err := os.OpenFile("failed.csv", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fatalf("open failed.csv: %v", err)
	}
	defer failedF.Close()
	failedW := csv.NewWriter(failedF)
	defer failedW.Flush()

	// Check if failed.csv is empty and write header if needed
	if stat, err := failedF.Stat(); err == nil && stat.Size() == 0 {
		_ = failedW.Write([]string{"entry_id", "old_asset_id", "new_asset_id", "error"})
	}

	client := &http.Client{Timeout: *timeout}
	ctx := context.Background()

	rowNum := 0

	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		rowNum++
		if err != nil {
			warnf("row %d: read: %v", rowNum, err)
			continue
		}
		if len(record) == 0 {
			warnf("row %d: empty record", rowNum)
			continue
		}
		entryID := strings.TrimSpace(record[0])
		assetID := strings.TrimSpace(record[1])

		if entryID == "" || assetID == "" {
			warnf("row %d: require entry_id and asset_id", rowNum)
			continue
		}
		if rowNum == 1 && (strings.EqualFold(entryID, "entry_id") || strings.EqualFold(assetID, "asset_id")) {
			// header row, skip
			continue
		}

		// First, fetch the entry and asset to get their information including versions

		fetchEntryReq := contentful.FetchEntryRequest{
			SpaceID:     *spaceID,
			Environment: *environment,
			EntryID:     entryID,
			HeaderName:  *headerName,
			Scheme:      *scheme,
			Token:       *token,
		}
		entry, entryStatus, err := contentful.FetchEntry(ctx, client, fetchEntryReq)
		if err != nil {
			warnf("row %d: fetch entry %s -> status %d: %v", rowNum, entryID, entryStatus, err)
			_ = failedW.Write([]string{entryID, assetID, "", fmt.Sprintf("fetch entry: %v", err)})
			continue
		}

		fetchAssetReq := contentful.FetchAssetRequest{
			SpaceID:     *spaceID,
			Environment: *environment,
			AssetID:     assetID,
			HeaderName:  *headerName,
			Scheme:      *scheme,
			Token:       *token,
		}
		asset, fetchStatus, err := contentful.FetchAsset(ctx, client, fetchAssetReq)
		if err != nil {
			warnf("row %d: fetch asset %s -> status %d: %v", rowNum, assetID, fetchStatus, err)
			_ = failedW.Write([]string{entryID, assetID, "", fmt.Sprintf("fetch asset: %v", err)})
			continue
		}

		// Download the asset file via contentful module
		var savedPath string
		if strings.TrimSpace(asset.FileURL) != "" {
			downloadReq := contentful.DownloadAssetRequest{
				Asset:   asset,
				DestDir: "downloaded",
			}
			if p, _, derr := contentful.DownloadAssetFile(ctx, client, downloadReq); derr != nil {
				warnf("row %d: download asset file: %v", rowNum, derr)
				_ = failedW.Write([]string{entryID, assetID, "", fmt.Sprintf("download file: %v", derr)})
				continue
			} else {
				savedPath = p
			}
		} else {
			_ = failedW.Write([]string{entryID, assetID, "", "asset has empty file URL"})
			continue
		}

		// Create a new asset from the downloaded file BEFORE unpublishing the old asset
		var newAssetID string
		if savedPath != "" {
			createReq := contentful.CreateAssetRequest{
				Asset:             asset,
				SpaceID:           *spaceID,
				Environment:       *environment,
				Locale:            "en-US",
				FilePath:          savedPath,
				HeaderName:        *headerName,
				Scheme:            *scheme,
				Token:             *token,
				OriginalCreatedAt: asset.CreatedAt,
			}
			if nid, _, cerr := contentful.CreateAndPublishAssetFromFile(ctx, client, createReq); cerr != nil {
				warnf("row %d: create new asset from file: %v", rowNum, cerr)
				_ = failedW.Write([]string{entryID, assetID, nid, fmt.Sprintf("create new asset: %v", cerr)})
				continue
			} else {
				newAssetID = nid
			}
		} else {
			_ = failedW.Write([]string{entryID, assetID, "", "no savedPath for new asset"})
			continue
		}

		// Unpublish the old asset first
		unpublishReq := contentful.UnpublishAssetRequest{
			SpaceID:     *spaceID,
			Environment: *environment,
			AssetID:     assetID,
			Version:     asset.Version,
			HeaderName:  *headerName,
			Scheme:      *scheme,
			Token:       *token,
		}
		unpublishStatus, err := contentful.UnpublishAsset(ctx, client, unpublishReq)
		if err != nil {
			warnf("row %d: unpublish asset %s -> status %d: %v", rowNum, assetID, unpublishStatus, err)
			_ = failedW.Write([]string{entryID, assetID, newAssetID, fmt.Sprintf("unpublish old asset: %v", err)})
			continue
		}

		// Then archive the old asset
		archiveReq := contentful.ArchiveAssetRequest{
			SpaceID:     *spaceID,
			Environment: *environment,
			AssetID:     assetID,
			Version:     asset.Version,
			HeaderName:  *headerName,
			Scheme:      *scheme,
			Token:       *token,
		}
		archiveStatus, err := contentful.ArchiveAsset(ctx, client, archiveReq)
		if err != nil {
			warnf("row %d: archive asset %s -> status %d: %v", rowNum, assetID, archiveStatus, err)
			_ = failedW.Write([]string{entryID, assetID, newAssetID, fmt.Sprintf("archive old asset: %v", err)})
			continue
		}

		// Patch the entry to point to the new asset, then publish
		if newAssetID != "" {
			patchReq := contentful.PatchEntryAssetLinkRequest{
				SpaceID:     *spaceID,
				Environment: *environment,
				EntryID:     entryID,
				FieldKey:    "downloadableFile",
				Locale:      "en-US",
				NewAssetID:  newAssetID,
				Version:     entry.Version,
				HeaderName:  *headerName,
				Scheme:      *scheme,
				Token:       *token,
			}
			newVersion, updStatus, uerr := contentful.PatchEntryAssetLink(ctx, client, patchReq)
			if uerr != nil {
				warnf("row %d: patch entry %s -> status %d: %v", rowNum, entryID, updStatus, uerr)
				_ = failedW.Write([]string{entryID, assetID, newAssetID, fmt.Sprintf("patch entry: %v", uerr)})
				continue
			}
			publishReq := contentful.PublishEntryRequest{
				SpaceID:     *spaceID,
				Environment: *environment,
				EntryID:     entryID,
				Version:     newVersion,
				HeaderName:  *headerName,
				Scheme:      *scheme,
				Token:       *token,
			}
			if pubStatus, perr := contentful.PublishEntry(ctx, client, publishReq); perr != nil {
				warnf("row %d: publish entry %s -> status %d: %v", rowNum, entryID, pubStatus, perr)
				_ = failedW.Write([]string{entryID, assetID, newAssetID, fmt.Sprintf("publish entry: %v", perr)})
				continue
			}

			// Validate that the published entry contains the new asset ID
			if validateAssetReplacement(ctx, client, entryID, newAssetID, *spaceID, *environment, *headerName, *scheme, *token, rowNum, successW, failedW, assetID) {
				continue
			}
		} else {
			_ = failedW.Write([]string{entryID, assetID, newAssetID, "missing new asset id"})
			continue
		}
	}

}

// validateAssetReplacement validates that the published entry contains the expected new asset ID
// Returns true if validation failed and processing should continue to next iteration
func validateAssetReplacement(ctx context.Context, client *http.Client, entryID, newAssetID, spaceID, environment, headerName, scheme, token string, rowNum int, successW, failedW *csv.Writer, oldAssetID string) bool {
	validateEntryReq := contentful.FetchEntryRequest{
		SpaceID:     spaceID,
		Environment: environment,
		EntryID:     entryID,
		HeaderName:  headerName,
		Scheme:      scheme,
		Token:       token,
	}
	validatedEntry, validateStatus, verr := contentful.FetchEntry(ctx, client, validateEntryReq)
	if verr != nil {
		warnf("row %d: validate entry %s -> status %d: %v", rowNum, entryID, validateStatus, verr)
		_ = failedW.Write([]string{entryID, oldAssetID, newAssetID, fmt.Sprintf("validation fetch entry: %v", verr)})
		return true
	}

	// Check if the validated entry contains the new asset ID
	if validatedEntry.AssetID == newAssetID {
		// Success: record entry_id, old asset id, and new asset id
		_ = successW.Write([]string{entryID, oldAssetID, newAssetID})
		return false
	} else {
		// Validation failed: the entry doesn't contain the expected new asset ID
		warnf("row %d: validation failed for entry %s - expected asset %s but found %s", rowNum, entryID, newAssetID, validatedEntry.AssetID)
		_ = failedW.Write([]string{entryID, oldAssetID, newAssetID, fmt.Sprintf("validation failed: expected asset %s but found %s", newAssetID, validatedEntry.AssetID)})
		return true
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "ERROR: "+format+"\n", args...)
	os.Exit(1)
}

func warnf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "WARN: "+format+"\n", args...)
}
