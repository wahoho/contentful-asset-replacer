package contentful

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// EntryResponse models the localized CMA entry response
type EntryResponse struct {
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
		FieldStatus    map[string]map[string]string `json:"fieldStatus"`
		AutomationTags []any                        `json:"automationTags"`
		ContentType    struct {
			Sys struct {
				Type     string `json:"type"`
				LinkType string `json:"linkType"`
				ID       string `json:"id"`
			} `json:"sys"`
		} `json:"contentType"`
		Urn string `json:"urn"`
	} `json:"sys"`
	Fields map[string]any `json:"fields"`
}

// Entry is a minimal DTO for callers
type Entry struct {
	ID            string
	Version       int
	ContentTypeID string
	AssetID       string
	FieldStatus   map[string]map[string]string
}

// FetchEntryRequest contains all the parameters needed to fetch an entry
type FetchEntryRequest struct {
	SpaceID     string
	Environment string
	EntryID     string
	HeaderName  string
	Scheme      string
	Token       string
}

// UpdateEntryAssetLinkRequest contains all the parameters needed to update an entry's asset link
type UpdateEntryAssetLinkRequest struct {
	SpaceID     string
	Environment string
	EntryID     string
	FieldKey    string
	Locale      string
	NewAssetID  string
	Version     int
	HeaderName  string
	Scheme      string
	Token       string
}

// PublishEntryRequest contains all the parameters needed to publish an entry
type PublishEntryRequest struct {
	SpaceID     string
	Environment string
	EntryID     string
	Version     int
	HeaderName  string
	Scheme      string
	Token       string
}

// PatchEntryAssetLinkRequest contains all the parameters needed to patch an entry's asset link
type PatchEntryAssetLinkRequest struct {
	SpaceID     string
	Environment string
	EntryID     string
	FieldKey    string
	Locale      string
	NewAssetID  string
	Version     int
	HeaderName  string
	Scheme      string
	Token       string
}

// FetchEntry retrieves a single Entry by ID using CMA
func FetchEntry(ctx context.Context, client *http.Client, req FetchEntryRequest) (Entry, int, error) {
	// Extract values from the request struct
	spaceID := req.SpaceID
	environment := req.Environment
	entryID := req.EntryID
	headerName := req.HeaderName
	scheme := req.Scheme
	token := req.Token

	url := fmt.Sprintf("https://api.contentful.com/spaces/%s/environments/%s/entries/%s", spaceID, environment, entryID)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Entry{}, 0, err
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set(headerName, strings.TrimSpace(scheme+" "+token))

	resp, err := client.Do(httpReq)
	if err != nil {
		return Entry{}, 0, err
	}
	defer resp.Body.Close()

	status := resp.StatusCode
	if status < 200 || status >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return Entry{}, status, fmt.Errorf("unexpected status %d: %s", status, strings.TrimSpace(string(body)))
	}

	var er EntryResponse
	dec := json.NewDecoder(resp.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&er); err != nil {
		return Entry{}, status, err
	}

	ctID := ""
	if er.Sys.ContentType.Sys.ID != "" {
		ctID = er.Sys.ContentType.Sys.ID
	}

	// Extract AssetID from fields.downloadableFile["en-US"].sys.id
	assetID := ""
	if df, ok := er.Fields["downloadableFile"]; ok {
		if dfMap, ok := df.(map[string]any); ok {
			if localized, ok := dfMap["en-US"]; ok {
				if locMap, ok := localized.(map[string]any); ok {
					if sys, ok := locMap["sys"].(map[string]any); ok {
						if id, ok := sys["id"].(string); ok {
							assetID = id
						}
					}
				}
			}
		}
	}

	return Entry{
		ID:            er.Sys.ID,
		Version:       er.Sys.Version,
		ContentTypeID: ctID,
		AssetID:       assetID,
		FieldStatus:   er.Sys.FieldStatus,
	}, status, nil
}

// UpdateEntryAssetLink sets a single asset link field on the entry (e.g. downloadableFile)
func UpdateEntryAssetLink(ctx context.Context, client *http.Client, req UpdateEntryAssetLinkRequest) (int, int, error) {
	// Extract values from the request struct
	spaceID := req.SpaceID
	environment := req.Environment
	entryID := req.EntryID
	fieldKey := req.FieldKey
	locale := req.Locale
	newAssetID := req.NewAssetID
	version := req.Version
	headerName := req.HeaderName
	scheme := req.Scheme
	token := req.Token

	if locale == "" {
		locale = "en-US"
	}
	url := fmt.Sprintf("https://api.contentful.com/spaces/%s/environments/%s/entries/%s", spaceID, environment, entryID)
	payload := map[string]any{
		"fields": map[string]any{
			fieldKey: map[string]any{locale: map[string]any{
				"sys": map[string]string{
					"type": "Link", "linkType": "Asset", "id": newAssetID,
				},
			}},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, 0, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, url, strings.NewReader(string(body)))
	if err != nil {
		return 0, 0, err
	}
	httpReq.Header.Set("Content-Type", "application/vnd.contentful.management.v1+json")
	httpReq.Header.Set("X-Contentful-Version", fmt.Sprintf("%d", version))
	httpReq.Header.Set(headerName, strings.TrimSpace(scheme+" "+token))
	resp, err := client.Do(httpReq)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()
	status := resp.StatusCode
	if status < 200 || status >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return 0, status, fmt.Errorf("update entry failed: %s", strings.TrimSpace(string(b)))
	}
	var er EntryResponse
	if err := json.NewDecoder(resp.Body).Decode(&er); err != nil {
		return 0, status, err
	}
	return er.Sys.Version, status, nil
}

// PublishEntry publishes an entry with the supplied version
func PublishEntry(ctx context.Context, client *http.Client, req PublishEntryRequest) (int, error) {
	// Extract values from the request struct
	spaceID := req.SpaceID
	environment := req.Environment
	entryID := req.EntryID
	version := req.Version
	headerName := req.HeaderName
	scheme := req.Scheme
	token := req.Token

	url := fmt.Sprintf("https://api.contentful.com/spaces/%s/environments/%s/entries/%s/published", spaceID, environment, entryID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, url, nil)
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
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return resp.StatusCode, fmt.Errorf("publish entry failed: %s", strings.TrimSpace(string(b)))
	}
	return resp.StatusCode, nil
}

// PatchEntryAssetLink applies a JSON Patch to set fields.{fieldKey}.{locale} to a new Asset link
func PatchEntryAssetLink(ctx context.Context, client *http.Client, req PatchEntryAssetLinkRequest) (int, int, error) {
	// Extract values from the request struct
	spaceID := req.SpaceID
	environment := req.Environment
	entryID := req.EntryID
	fieldKey := req.FieldKey
	locale := req.Locale
	newAssetID := req.NewAssetID
	version := req.Version
	headerName := req.HeaderName
	scheme := req.Scheme
	token := req.Token

	if locale == "" {
		locale = "en-US"
	}
	url := fmt.Sprintf("https://api.contentful.com/spaces/%s/environments/%s/entries/%s", spaceID, environment, entryID)
	patch := []map[string]any{
		{
			"op":   "replace",
			"path": fmt.Sprintf("/fields/%s/%s", fieldKey, locale),
			"value": map[string]any{
				"sys": map[string]any{
					"type":     "Link",
					"linkType": "Asset",
					"id":       newAssetID,
				},
			},
		},
	}
	body, err := json.Marshal(patch)
	if err != nil {
		return 0, 0, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, strings.NewReader(string(body)))
	if err != nil {
		return 0, 0, err
	}
	httpReq.Header.Set("Content-Type", "application/json-patch+json")
	httpReq.Header.Set("X-Contentful-Version", fmt.Sprintf("%d", version))
	httpReq.Header.Set(headerName, strings.TrimSpace(scheme+" "+token))
	resp, err := client.Do(httpReq)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()
	status := resp.StatusCode
	if status < 200 || status >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return 0, status, fmt.Errorf("patch entry failed: %s", strings.TrimSpace(string(b)))
	}
	var er EntryResponse
	if err := json.NewDecoder(resp.Body).Decode(&er); err != nil {
		return 0, status, err
	}
	return er.Sys.Version, status, nil
}
