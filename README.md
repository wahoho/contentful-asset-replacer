# Contentful Assets Replacer 

A Go program that processes Contentful entries and assets by downloading existing assets, creating new versions, and updating entry references. This tool is useful for asset migration, backup, or bulk processing operations in Contentful.

## What This Program Does

The program performs the following operations for each entry-asset pair in the input CSV:

1. **Fetches Entry and Asset**: Retrieves the specified entry and asset from Contentful using the Content Management API (CMA)
2. **Downloads Asset File**: Downloads the asset file to the local `downloaded/` directory
3. **Creates New Asset**: Creates a new asset from the downloaded file with the same metadata (title, description, content type)
4. **Publishes New Asset**: Automatically publishes the newly created asset
5. **Unpublishes Old Asset**: Unpublishes the original asset
6. **Archives Old Asset**: Archives the original asset to remove it from active use
7. **Updates Entry Reference**: Updates the entry to point to the new asset instead of the old one
8. **Publishes Entry**: Publishes the updated entry with the new asset reference

## Input Format

The program expects a CSV file with the following columns:
- `entry_id`: The Contentful entry ID
- `asset_id`: The Contentful asset ID to be replaced

Example CSV (`id.csv`):
```csv
entry_id,asset_id
XXXXX,YYYYY
```

## Output Files

The program generates two output CSV files:

### `success.csv`
Contains successfully processed entries with the following columns:
- `entry_id`: The entry ID that was processed
- `old_asset_id`: The original asset ID that was replaced
- `new_asset_id`: The newly created asset ID

### `failed.csv`
Contains failed operations with the following columns:
- `entry_id`: The entry ID that failed to process
- `old_asset_id`: The original asset ID
- `new_asset_id`: The new asset ID (if created before failure)
- `error`: Description of the error that occurred

## Command Line Arguments

| Argument | Type | Default | Required | Description |
|----------|------|---------|----------|-------------|
| `-csv` | string | `id.csv` | Yes | Path to CSV file containing entry_id and asset_id columns |
| `-token` | string | `$API_TOKEN` | Yes | Bearer token for Contentful API authentication (can also be set via API_TOKEN environment variable) |
| `-space-id` | string | `$SPACE_ID` | Yes | Contentful space ID (or set SPACE_ID env var) |
| `-environment` | string | `testing_env` | No | Contentful environment to use for the base URL |
| `-auth-header` | string | `Authorization` | No | Authorization header name |
| `-scheme` | string | `Bearer` | No | Authorization scheme prefix (e.g., Bearer) |
| `-timeout` | duration | `20s` | No | HTTP client timeout duration |

## Usage Examples

### Basic Usage
```bash
go run main.go -space-id ZZZZZZ -csv id.csv -token your_contentful_token
```

### With Custom Environment and Timeout
```bash
go run main.go -space-id ZZZZZZ -csv id.csv -token your_token -environment production -timeout 30s
```

### Using Environment Variables
```bash
export API_TOKEN=your_contentful_token
export SPACE_ID=ZZZZZZ
go run main.go -csv id.csv
```

### Using Environment Variable for Token Only
```bash
export API_TOKEN=your_contentful_token
go run main.go -space-id ZZZZZZ -csv id.csv
```

## Prerequisites

- Go 1.19 or later
- Valid Contentful API token with appropriate permissions
- Network access to Contentful API endpoints

## Required Permissions

Your Contentful API token must have the following permissions:
- Read access to entries and assets
- Write access to create new assets
- Publish/unpublish permissions for assets and entries
- Archive permissions for assets

## Error Handling

The program includes comprehensive error handling:
- Continues processing other entries if one fails
- Logs warnings for individual failures
- Records all failures in `failed.csv` with detailed error messages
- Validates required parameters before processing

## File Structure

```
contentful-assets-list/
├── main.go                 # Main program entry point
├── contentful/
│   ├── asset.go           # Asset management functions
│   └── entry.go           # Entry management functions
├── downloaded/            # Directory for downloaded asset files
├── id.csv                # Input CSV file (example)
├── success.csv           # Output: successfully processed entries
├── failed.csv            # Output: failed operations
└── README.md             # This file
```
