# Contentful Asset Replacer 

A Go program that provides multiple modes for processing Contentful entries and assets. This tool supports asset replacement, listing, publishing, and archive status checking operations in Contentful.

## Modes

The program supports four different operation modes:

### 1. Update Mode (Default)
Replaces assets by downloading existing assets, creating new versions, and updating entry references. This mode is useful for asset migration, backup, or bulk processing operations.

**Process Flow:**
1. **Fetches Entry**: Retrieves the specified entry from Contentful
2. **Extracts Asset ID**: Gets the asset ID from the entry's `downloadableFile` field
3. **Fetches Asset**: Retrieves the asset using the extracted asset ID
4. **Downloads Asset File**: Downloads the asset file to the local `downloaded/` directory
5. **Creates New Asset**: Creates a new asset from the downloaded file with the same metadata
6. **Publishes New Asset**: Automatically publishes the newly created asset
7. **Unpublishes Old Asset**: Unpublishes the original asset
8. **Archives Old Asset**: Archives the original asset to remove it from active use
9. **Updates Entry Reference**: Updates the entry to point to the new asset
10. **Publishes Entry**: Publishes the updated entry with the new asset reference

### 2. List Mode
Generates a listing of entries and their associated assets, showing entry status and asset information.

### 3. Publish Mode
Publishes entries that are currently in draft state.

### 4. Archived-List Mode
Checks the archive status of assets by providing asset IDs and returning whether each asset is archived along with metadata.

## Input Format

The program expects different CSV formats depending on the mode:

### Update Mode
- **File**: `id.csv` (or custom path)
- **Columns**: `entry_id` only
- **Description**: The asset ID will be automatically extracted from each entry's `downloadableFile` field

Example CSV for Update Mode:
```csv
entry_id
6Xz36thDZMNh5FfRcnwB75
18N5nEKsDYQWNXbnD9mGtq
```

### List Mode
- **File**: `id.csv` (or custom path)
- **Columns**: `entry_id` only
- **Description**: Lists entry information and associated assets

### Publish Mode
- **File**: `id.csv` (or custom path)
- **Columns**: `entry_id` only
- **Description**: Publishes the specified entries

### Archived-List Mode
- **File**: `asset_ids.csv` (or custom path)
- **Columns**: `asset_id` only
- **Description**: Checks archive status of the specified assets

Example CSV for Archived-List Mode:
```csv
asset_id
5kpzfHsn5zDxBsUBZXEDtg
1bgKtdYYIEfeQk6lQwprpM
```

## Output Files

The program generates different output files depending on the mode:

### Update Mode Outputs

#### `success.csv`
Contains successfully processed entries with the following columns:
- `entry_id`: The entry ID that was processed
- `old_asset_id`: The original asset ID that was replaced
- `new_asset_id`: The newly created asset ID

#### `failed.csv`
Contains failed operations with the following columns:
- `entry_id`: The entry ID that failed to process
- `old_asset_id`: The original asset ID
- `new_asset_id`: The new asset ID (if created before failure)
- `error`: Description of the error that occurred

### List Mode Output

#### `entry_asset_list.csv`
Contains entry and asset information with the following columns:
- `entry_id`: The entry ID
- `entry_status`: The status of the entry
- `asset_id`: The associated asset ID

### Publish Mode Outputs

#### `publish_success.csv`
Contains successfully published entries with the following columns:
- `entry_id`: The entry ID that was published
- `version`: The version number before publishing
- `published_version`: The version number after publishing

#### `publish_failed.csv`
Contains failed publish operations with the following columns:
- `entry_id`: The entry ID that failed to publish
- `error`: Description of the error that occurred

### Archived-List Mode Output

#### `archived_asset_list.csv`
Contains asset archive status information with the following columns:
- `asset_id`: The asset ID
- `is_archived`: "true" or "false" indicating archive status
- `archived_at`: RFC3339 timestamp when archived (empty if not archived)
- `title`: The asset title
- `file_url`: The file URL with HTTPS protocol

## Command Line Arguments

| Argument | Type | Default | Required | Description |
|----------|------|---------|----------|-------------|
| `-csv` | string | `id.csv` | Yes | Path to CSV file containing entry_id column (asset_id will be retrieved from entry's downloadableFile for update mode) |
| `-token` | string | `$API_TOKEN` | Yes | Bearer token for Contentful API authentication (can also be set via API_TOKEN environment variable) |
| `-space-id` | string | `$SPACE_ID` | Yes | Contentful space ID (or set SPACE_ID env var) |
| `-mode` | string | `update` | No | Operation mode: 'update' to replace assets, 'list' to generate entry/asset listing, 'publish' to publish entries, or 'archived-list' to check if assets are archived |
| `-environment` | string | `yap_env2` | No | Contentful environment to use for the base URL |
| `-auth-header` | string | `Authorization` | No | Authorization header name |
| `-scheme` | string | `Bearer` | No | Authorization scheme prefix (e.g., Bearer) |
| `-timeout` | duration | `20s` | No | HTTP client timeout duration |

## Usage Examples

### Update Mode (Default)
Replace assets by downloading and recreating them:
```bash
go run main.go -space-id ZZZZZZ -csv id.csv -token your_contentful_token
```

### List Mode
Generate a listing of entries and their associated assets:
```bash
go run main.go -mode list -space-id ZZZZZZ -csv id.csv -token your_contentful_token
```

### Publish Mode
Publish entries that are currently in draft state:
```bash
go run main.go -mode publish -space-id ZZZZZZ -csv id.csv -token your_contentful_token
```

### Archived-List Mode
Check the archive status of assets:
```bash
go run main.go -mode archived-list -space-id ZZZZZZ -csv asset_ids.csv -token your_contentful_token
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
contentful-asset-replacer/
├── main.go                      # Main program entry point
├── contentful/
│   ├── asset.go                 # Asset management functions
│   └── entry.go                 # Entry management functions
├── downloaded/                  # Directory for downloaded asset files
├── id.csv                       # Input CSV file for update/list/publish modes (example)
├── asset_ids.csv               # Input CSV file for archived-list mode (example)
├── success.csv                 # Output: successfully processed entries (update mode)
├── failed.csv                  # Output: failed operations (update mode)
├── entry_asset_list.csv        # Output: entry and asset listing (list mode)
├── publish_success.csv         # Output: successfully published entries (publish mode)
├── publish_failed.csv          # Output: failed publish operations (publish mode)
├── archived_asset_list.csv     # Output: asset archive status (archived-list mode)
└── README.md                   # This file
```
