package contentful

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// AssetResponse models the localized CMA asset response
type AssetResponse struct {
	Metadata struct {
		Tags     []any `json:"tags"`
		Concepts []any `json:"concepts"`
	} `json:"metadata"`
	Sys struct {
		Space struct {
			Sys struct {
				Type     string `json:"type"`
				LinkType string `json:"linkType"`
				ID       string `json:"id"`
			} `json:"sys"`
		} `json:"space"`
		ID          string    `json:"id"`
		Type        string    `json:"type"`
		CreatedAt   time.Time `json:"createdAt"`
		UpdatedAt   time.Time `json:"updatedAt"`
		Environment struct {
			Sys struct {
				ID       string `json:"id"`
				Type     string `json:"type"`
				LinkType string `json:"linkType"`
			} `json:"sys"`
		} `json:"environment"`
		PublishedVersion int       `json:"publishedVersion"`
		PublishedAt      time.Time `json:"publishedAt"`
		FirstPublishedAt time.Time `json:"firstPublishedAt"`
		CreatedBy        struct {
			Sys struct {
				Type     string `json:"type"`
				LinkType string `json:"linkType"`
				ID       string `json:"id"`
			} `json:"sys"`
		} `json:"createdBy"`
		UpdatedBy struct {
			Sys struct {
				Type     string `json:"type"`
				LinkType string `json:"linkType"`
				ID       string `json:"id"`
			} `json:"sys"`
		} `json:"updatedBy"`
		PublishedCounter int `json:"publishedCounter"`
		Version          int `json:"version"`
		PublishedBy      struct {
			Sys struct {
				Type     string `json:"type"`
				LinkType string `json:"linkType"`
				ID       string `json:"id"`
			} `json:"sys"`
		} `json:"publishedBy"`
		FieldStatus map[string]map[string]string `json:"fieldStatus"`
		Urn         string                       `json:"urn"`
		// Optional archived fields - only present when asset is archived
		ArchivedAt *time.Time `json:"archivedAt,omitempty"`
		ArchivedBy *struct {
			Sys struct {
				Type     string `json:"type"`
				LinkType string `json:"linkType"`
				ID       string `json:"id"`
			} `json:"sys"`
		} `json:"archivedBy,omitempty"`
		ArchivedVersion *int `json:"archivedVersion,omitempty"`
	} `json:"sys"`
	Fields struct {
		Title       map[string]string `json:"title"`
		Description map[string]string `json:"description"`
		File        map[string]struct {
			URL     string `json:"url"`
			Details struct {
				Size int64 `json:"size"`
			} `json:"details"`
			FileName    string `json:"fileName"`
			ContentType string `json:"contentType"`
		} `json:"file"`
	} `json:"fields"`
}

type Asset struct {
	ID          string
	Version     int
	FileName    string
	FileURL     string
	ContentType string
	Title       string
	Description string
	CreatedAt   time.Time
}

// CreateAssetRequest contains all the parameters needed to create a new asset
type CreateAssetRequest struct {
	Asset             Asset
	SpaceID           string
	Environment       string
	Locale            string
	FilePath          string
	HeaderName        string
	Scheme            string
	Token             string
	OriginalCreatedAt time.Time // Original asset creation timestamp
}

// FetchAssetRequest contains all the parameters needed to fetch an asset
type FetchAssetRequest struct {
	SpaceID     string
	Environment string
	AssetID     string
	HeaderName  string
	Scheme      string
	Token       string
}

// ArchiveAssetRequest contains all the parameters needed to archive an asset
type ArchiveAssetRequest struct {
	SpaceID     string
	Environment string
	AssetID     string
	Version     int
	HeaderName  string
	Scheme      string
	Token       string
}

// UnpublishAssetRequest contains all the parameters needed to unpublish an asset
type UnpublishAssetRequest struct {
	SpaceID     string
	Environment string
	AssetID     string
	Version     int
	HeaderName  string
	Scheme      string
	Token       string
}

// DownloadAssetRequest contains all the parameters needed to download an asset
type DownloadAssetRequest struct {
	Asset   Asset
	DestDir string
}

// FetchAsset retrieves an asset from Contentful API
func FetchAsset(ctx context.Context, client *http.Client, req FetchAssetRequest) (Asset, int, error) {
	// Extract values from the request struct
	spaceID := req.SpaceID
	environment := req.Environment
	assetID := req.AssetID
	headerName := req.HeaderName
	scheme := req.Scheme
	token := req.Token

	// Build the asset URL
	url := fmt.Sprintf("https://api.contentful.com/spaces/%s/environments/%s/assets/%s", spaceID, environment, assetID)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, ensureHTTPS(url), nil)
	if err != nil {
		return Asset{}, 0, err
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set(headerName, strings.TrimSpace(scheme+" "+token))

	resp, err := client.Do(httpReq)
	if err != nil {
		return Asset{}, 0, err
	}
	defer resp.Body.Close()

	status := resp.StatusCode
	if status < 200 || status >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return Asset{}, status, fmt.Errorf("unexpected status %d: %s", status, strings.TrimSpace(string(body)))
	}

	var asset AssetResponse
	dec := json.NewDecoder(resp.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&asset); err != nil {
		return Asset{}, status, err
	}

	var fileURL string
	var fileName string
	var contentType string
	if f, ok := asset.Fields.File["en-US"]; ok {
		fileURL = f.URL
		fileName = f.FileName
		contentType = f.ContentType
	}

	var title string
	var description string
	if t, ok := asset.Fields.Title["en-US"]; ok {
		title = t
	}
	if d, ok := asset.Fields.Description["en-US"]; ok {
		description = d
	}

	return Asset{
		ID:          asset.Sys.ID,
		Version:     asset.Sys.Version,
		FileName:    fileName,
		FileURL:     fileURL,
		ContentType: contentType,
		Title:       title,
		Description: description,
		CreatedAt:   asset.Sys.CreatedAt,
	}, status, nil
}

// CreateAndPublishAssetFromFile uploads a binary file and creates a new Asset referencing it, setting title and description. Returns new asset ID.
func CreateAndPublishAssetFromFile(ctx context.Context, client *http.Client, req CreateAssetRequest) (string, int, error) {
	// Extract values from the request struct
	spaceID := req.SpaceID
	environment := req.Environment
	locale := req.Locale
	filePath := req.FilePath
	fileName := req.Asset.FileName
	contentType := req.Asset.ContentType
	title := req.Asset.Title
	description := req.Asset.Description
	headerName := req.HeaderName
	scheme := req.Scheme
	token := req.Token
	originalCreatedAt := req.OriginalCreatedAt.Format("20060102_150405")

	if strings.TrimSpace(fileName) == "" {
		fileName = filepath.Base(strings.TrimSpace(filePath))
	}

	// Remove timestamp from filename if it was added during download
	fileName = removeTimestampFromFilename(fileName, originalCreatedAt)

	if strings.TrimSpace(locale) == "" {
		locale = "en-US"
	}

	// 1) Upload binary
	f, err := os.Open(filePath)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()

	uploadURL := fmt.Sprintf("https://upload.contentful.com/spaces/%s/uploads", spaceID)
	upReq, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, f)
	if err != nil {
		return "", 0, err
	}
	upReq.Header.Set("Content-Type", "application/octet-stream")
	upReq.Header.Set(headerName, strings.TrimSpace(scheme+" "+token))
	upResp, err := client.Do(upReq)
	if err != nil {
		return "", 0, err
	}
	defer upResp.Body.Close()
	if upResp.StatusCode < 200 || upResp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(upResp.Body, 4096))
		return "", upResp.StatusCode, fmt.Errorf("upload failed: %s", strings.TrimSpace(string(body)))
	}
	var uploadRes struct {
		Sys struct {
			ID string `json:"id"`
		} `json:"sys"`
	}
	if err := json.NewDecoder(upResp.Body).Decode(&uploadRes); err != nil {
		return "", upResp.StatusCode, err
	}

	// 2) Create asset referencing the upload
	createURL := fmt.Sprintf("https://api.contentful.com/spaces/%s/environments/%s/assets", spaceID, environment)
	if strings.TrimSpace(title) == "" {
		title = fileName
	}
	payload := map[string]any{
		"fields": map[string]any{
			"title":       map[string]string{locale: title},
			"description": map[string]string{locale: description},
			"file": map[string]any{locale: map[string]any{
				"fileName":    fileName,
				"contentType": contentType,
				"uploadFrom": map[string]any{"sys": map[string]string{
					"type":     "Link",
					"linkType": "Upload",
					"id":       uploadRes.Sys.ID,
				}},
			}},
		},
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return "", 0, err
	}
	crReq, err := http.NewRequestWithContext(ctx, http.MethodPost, createURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", 0, err
	}
	crReq.Header.Set("Content-Type", "application/vnd.contentful.management.v1+json")
	crReq.Header.Set(headerName, strings.TrimSpace(scheme+" "+token))
	crResp, err := client.Do(crReq)
	if err != nil {
		return "", 0, err
	}
	defer crResp.Body.Close()
	if crResp.StatusCode < 200 || crResp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(crResp.Body, 4096))
		return "", crResp.StatusCode, fmt.Errorf("create asset failed: %s", strings.TrimSpace(string(body)))
	}
	var created struct {
		Sys struct {
			ID string `json:"id"`
		} `json:"sys"`
	}
	if err := json.NewDecoder(crResp.Body).Decode(&created); err != nil {
		return "", crResp.StatusCode, err
	}
	newAssetID := created.Sys.ID

	// 3) Request processing of the file
	processURL := fmt.Sprintf("https://api.contentful.com/spaces/%s/environments/%s/assets/%s/files/%s/process", spaceID, environment, newAssetID, locale)
	prReq, err := http.NewRequestWithContext(ctx, http.MethodPut, processURL, nil)
	if err != nil {
		return newAssetID, 0, err
	}
	prReq.Header.Set(headerName, strings.TrimSpace(scheme+" "+token))
	prReq.Header.Set("Accept", "application/vnd.contentful.management.v1+json")
	prResp, err := client.Do(prReq)
	if err != nil {
		return newAssetID, 0, err
	}
	prResp.Body.Close()

	// 4) Poll until processing completes and file URL is available, capturing latest version
	getURL := fmt.Sprintf("https://api.contentful.com/spaces/%s/environments/%s/assets/%s", spaceID, environment, newAssetID)
	var latestVersion int
	var hasURL bool
	for i := 0; i < 60; i++ { // up to ~60s
		gr, err := http.NewRequestWithContext(ctx, http.MethodGet, getURL, nil)
		if err != nil {
			return newAssetID, 0, err
		}
		gr.Header.Set("Accept", "application/vnd.contentful.management.v1+json")
		gr.Header.Set(headerName, strings.TrimSpace(scheme+" "+token))
		gv, err := client.Do(gr)
		if err != nil {
			return newAssetID, 0, err
		}
		if gv.StatusCode < 200 || gv.StatusCode >= 300 {
			body, _ := io.ReadAll(io.LimitReader(gv.Body, 4096))
			gv.Body.Close()
			return newAssetID, gv.StatusCode, fmt.Errorf("get created asset failed: %s", strings.TrimSpace(string(body)))
		}
		var createdAsset struct {
			Sys struct {
				Version int `json:"version"`
			} `json:"sys"`
			Fields map[string]any `json:"fields"`
		}
		if err := json.NewDecoder(gv.Body).Decode(&createdAsset); err != nil {
			gv.Body.Close()
			return newAssetID, 0, err
		}
		gv.Body.Close()
		latestVersion = createdAsset.Sys.Version
		if fAny, ok := createdAsset.Fields["file"]; ok {
			if fMap, ok := fAny.(map[string]any); ok {
				if locAny, ok := fMap[locale]; ok {
					if locMap, ok := locAny.(map[string]any); ok {
						if urlAny, ok := locMap["url"]; ok {
							if urlStr, ok := urlAny.(string); ok && strings.TrimSpace(urlStr) != "" {
								hasURL = true
								break
							}
						}
					}
				}
			}
		}
		time.Sleep(1 * time.Second)
	}
	if !hasURL {
		return newAssetID, 0, fmt.Errorf("asset processing did not complete: file URL missing")
	}

	// 5) Publish the asset
	publishURL := fmt.Sprintf("https://api.contentful.com/spaces/%s/environments/%s/assets/%s/published", spaceID, environment, newAssetID)
	pubReq, err := http.NewRequestWithContext(ctx, http.MethodPut, publishURL, nil)
	if err != nil {
		return newAssetID, 0, err
	}
	pubReq.Header.Set("Accept", "application/vnd.contentful.management.v1+json")
	pubReq.Header.Set("X-Contentful-Version", fmt.Sprintf("%d", latestVersion))
	pubReq.Header.Set(headerName, strings.TrimSpace(scheme+" "+token))
	pubResp, err := client.Do(pubReq)
	if err != nil {
		return newAssetID, 0, err
	}
	defer pubResp.Body.Close()
	if pubResp.StatusCode < 200 || pubResp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(pubResp.Body, 4096))
		return newAssetID, pubResp.StatusCode, fmt.Errorf("publish asset failed: %s", strings.TrimSpace(string(body)))
	}

	return newAssetID, pubResp.StatusCode, nil
}

// ArchiveAsset archives an asset using Contentful Management API
func ArchiveAsset(ctx context.Context, client *http.Client, req ArchiveAssetRequest) (int, error) {
	// Extract values from the request struct
	spaceID := req.SpaceID
	environment := req.Environment
	assetID := req.AssetID
	version := req.Version
	headerName := req.HeaderName
	scheme := req.Scheme
	token := req.Token

	// Build the archive URL
	archiveURL := fmt.Sprintf("https://api.contentful.com/spaces/%s/environments/%s/assets/%s/archived?version=%d",
		spaceID, environment, assetID, version)

	// Get the current asset to create the archive payload
	getURL := fmt.Sprintf("https://api.contentful.com/spaces/%s/environments/%s/assets/%s",
		spaceID, environment, assetID)

	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, getURL, nil)
	if err != nil {
		return 0, err
	}
	getReq.Header.Set("Accept", "application/vnd.contentful.management.v1+json")
	getReq.Header.Set(headerName, strings.TrimSpace(scheme+" "+token))

	getResp, err := client.Do(getReq)
	if err != nil {
		return 0, err
	}
	defer getResp.Body.Close()

	if getResp.StatusCode < 200 || getResp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(getResp.Body, 4096))
		return getResp.StatusCode, fmt.Errorf("get asset failed with status %d: %s", getResp.StatusCode, strings.TrimSpace(string(body)))
	}

	// Parse the asset to create archive payload
	var asset map[string]interface{}
	if err := json.NewDecoder(getResp.Body).Decode(&asset); err != nil {
		return 0, err
	}

	sys, ok := asset["sys"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("invalid asset response: missing sys")
	}

	// Create archive payload according to Contentful API specification
	// The payload should contain the entire asset with sys.archivedAt set
	payload := make(map[string]interface{})
	for k, v := range asset {
		payload[k] = v
	}

	// Update the sys object to include archivedAt timestamp
	sysPayload := make(map[string]interface{})
	for k, v := range sys {
		sysPayload[k] = v
	}
	sysPayload["archivedAt"] = time.Now().Format(time.RFC3339)
	payload["sys"] = sysPayload

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, archiveURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return 0, err
	}

	httpReq.Header.Set("Content-Type", "application/vnd.contentful.management.v1+json")
	httpReq.Header.Set("X-Contentful-Version", fmt.Sprintf("%d", version))
	httpReq.Header.Set(headerName, strings.TrimSpace(scheme+" "+token))

	resp, err := client.Do(httpReq)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	status := resp.StatusCode
	if status < 200 || status >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return status, fmt.Errorf("archive failed with status %d: %s", status, strings.TrimSpace(string(body)))
	}

	return status, nil
}

// UnpublishAsset unpublishes an asset using Contentful Management API
func UnpublishAsset(ctx context.Context, client *http.Client, req UnpublishAssetRequest) (int, error) {
	// Extract values from the request struct
	spaceID := req.SpaceID
	environment := req.Environment
	assetID := req.AssetID
	version := req.Version
	headerName := req.HeaderName
	scheme := req.Scheme
	token := req.Token

	// Build the unpublish URL
	unpublishURL := fmt.Sprintf("https://api.contentful.com/spaces/%s/environments/%s/assets/%s/published",
		spaceID, environment, assetID)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, unpublishURL, nil)
	if err != nil {
		return 0, err
	}

	httpReq.Header.Set("Accept", "application/vnd.contentful.management.v1+json")
	httpReq.Header.Set("X-Contentful-Version", fmt.Sprintf("%d", version))
	httpReq.Header.Set(headerName, strings.TrimSpace(scheme+" "+token))

	resp, err := client.Do(httpReq)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	status := resp.StatusCode
	if status < 200 || status >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return status, fmt.Errorf("unpublish asset failed with status %d: %s", status, strings.TrimSpace(string(body)))
	}

	return status, nil
}

// ensureHTTPS ensures the URL uses HTTPS protocol
func ensureHTTPS(u string) string {
	s := strings.TrimSpace(u)
	if strings.HasPrefix(s, "//") {
		return "https:" + s
	}
	if strings.HasPrefix(strings.ToLower(s), "http://") {
		return "https://" + strings.TrimPrefix(s, "http://")
	}
	return s
}

// removeTimestampFromFilename removes timestamp from filename that was added during download
// Expected format: filename_YYYYMMDD_HHMMSS.ext -> filename.ext
func removeTimestampFromFilename(filename string, originalCreatedAt string) string {
	// If originalCreatedAt is empty, return the filename as-is
	if originalCreatedAt == "" {
		return filename
	}

	// Look for the timestamp pattern: _YYYYMMDD_HHMMSS
	timestampPattern := "_" + originalCreatedAt

	// Check if the filename ends with this timestamp pattern
	if strings.HasSuffix(filename, timestampPattern) {
		// Find the position where the timestamp pattern starts
		ext := filepath.Ext(filename)
		nameWithoutExt := strings.TrimSuffix(filename, ext)

		// Remove the timestamp pattern and add back the extension
		cleanName := strings.TrimSuffix(nameWithoutExt, timestampPattern)
		return cleanName + ext
	}

	return filename
}

// DownloadAssetFile downloads the asset's file to destDir and returns the saved path.
// It derives filename from Asset.FileName, falling back to the URL basename or Asset.ID.
// A timestamp is added to the filename to prevent duplicates.
func DownloadAssetFile(ctx context.Context, client *http.Client, req DownloadAssetRequest) (string, int, error) {
	// Extract values from the request struct
	asset := req.Asset
	destDir := req.DestDir

	if strings.TrimSpace(asset.FileURL) == "" {
		return "", 0, fmt.Errorf("empty asset file URL")
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", 0, err
	}

	fileName := strings.TrimSpace(asset.FileName)
	if fileName == "" {
		if parsed, err := url.Parse(ensureHTTPS(asset.FileURL)); err == nil {
			base := filepath.Base(parsed.Path)
			if base != "." && base != "/" && base != "" {
				fileName = base
			}
		}
		if fileName == "" {
			fileName = asset.ID
		}
	}

	// Add timestamp to filename to prevent duplicates using the asset's creation time
	timestamp := asset.CreatedAt.Format("20060102_150405")
	ext := filepath.Ext(fileName)
	nameWithoutExt := strings.TrimSuffix(fileName, ext)
	fileNameWithTimestamp := fmt.Sprintf("%s_%s%s", nameWithoutExt, timestamp, ext)

	destPath := filepath.Join(destDir, fileNameWithTimestamp)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, ensureHTTPS(asset.FileURL), nil)
	if err != nil {
		return "", 0, err
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", resp.StatusCode, fmt.Errorf("download status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	out, err := os.Create(destPath)
	if err != nil {
		return "", resp.StatusCode, err
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		return "", resp.StatusCode, err
	}

	return destPath, resp.StatusCode, nil
}
