# Dataset API Documentation

## List Datasets

### GET /api/v1/dataset

- **Description**: Fetches a list of all datasets.
- **Request Body**:

```json
{
    "name": "optional_dataset_name",
    "recursive": false,
    "depth": 0,
    "properties": ["name", "used", "available"],
    "parsable": true,
    "type": "all"
}
```

- **Response**:

```json
{
    "datasets": {
        "tank/fs1": {
            "name": "tank/fs1",
            "type": "filesystem",
            "pool": "tank",
            "properties": {
                "used": {
                    "value": "10G",
                    "source": { "type": "local", "data": "user" }
                }
            }
        }
    }
}
```
- **Error Codes**:
    - `2000`: ZFS command execution failed.
    - `2004`: Dataset not found.

## Destroy Dataset
### DELETE /api/v1/dataset
- **Description**: Deletes a dataset.
- **Request Body**:
```json
{
    "name": "tank/fs1",
    "recursive_destroy_dependents": true,
    "force": true
}
```
- **Response**: `204 No Content`
- **Error Codes**:
    - `2003`: Failed to destroy dataset.

## Rename Dataset
### POST /api/v1/dataset/rename
- **Description**: Renames a dataset.
- **Request Body**:
```json
{
    "name": "tank/fs1",
    "new_name": "tank/fs2",
    "force": true,
    "recursive": false
}
```
- **Response**: `200 OK`
- **Error Codes**:
    - `2005`: Failed to rename dataset.

## Get Dataset Differences
### POST /api/v1/dataset/diff
- **Description**: Fetches differences between two datasets.
- **Request Body**:
```json
{
    "name": "tank/fs1",
    "snapshot": "snap1",
    "snapshot2": "snap2"
}
```
- **Response**:
```json
{
    "result": [
        {
            "type": "modified",
            "path": "/path/to/file"
        },
        {
            "type": "added",
            "path": "/new/path/to/file"
        }
    ]
}
```
- **Error Codes**:
    - `2013`: Failed to fetch differences.

## List Dataset Properties
### GET /api/v1/dataset/properties
- **Description**: Lists all properties of a dataset.
- **Request Body**:
```json
{
    "name": "tank/fs1"
}
```
- **Response**:
```json
{
    "result": {
        "tank/fs1": {
            "properties": {
                "compression": {
                    "value": "on",
                    "source": { "type": "local", "data": "user" }
                }
            }
        }
    }
}
```
- **Error Codes**:
    - `2006`: Failed to retrieve properties.

## Get a Specific Dataset Property
### GET /api/v1/dataset/property
- **Description**: Retrieves a specific property for a dataset.
- **Request Body**:
```json
{
    "name": "tank/fs1",
    "property": "compression"
}
```
- **Response**:
```json
{
    "result": {
        "compression": {
            "value": "on",
            "source": { "type": "local", "data": "user" }
        }
    }
}
```
- **Error Codes**:
    - `2007`: Property not found.

## Set Dataset Property
### PUT /api/v1/dataset/property
- **Description**: Sets a property for a dataset.
- **Request Body**:
```json
{
    "name": "tank/fs1",
    "property": "compression",
    "value": "lz4"
}
```
- **Response**: `201 Created`
- **Error Codes**:
    - `2008`: Failed to set property.

## Inherit Dataset Property
### PUT /api/v1/dataset/property/inherit
- **Description**: Inherits a property for a dataset.
- **Request Body**:
```json
{
    "name": "tank/fs1",
    "property": "compression"
}
```
- **Response**: `201 Created`
- **Error Codes**:
    - `2009`: Failed to inherit property.

## List Filesystems
### GET /api/v1/dataset/filesystems
- **Description**: Fetches a list of all filesystems.
- **Request Body**:
```json
{
    "recursive": true
}
```
- **Response**:
```json
{
    "filesystems": [
        "tank/fs1",
        "tank/fs2"
    ]
}
```
- **Error Codes**:
    - `2010`: Failed to list filesystems.

## Create Filesystem
### POST /api/v1/dataset/filesystem
- **Description**: Creates a new filesystem.
- **Request Body**:
```json
{
    "name": "tank/fs1",
    "properties": {
        "compression": "on"
    },
    "parents": true
}
```
- **Response**: `201 Created`
- **Error Codes**:
    - `2011`: Failed to create filesystem.

## Mount Filesystem
### POST /api/v1/dataset/filesystem/mount
- **Description**: Mounts a filesystem.
- **Request Body**:
```json
{
    "name": "tank/fs1",
    "force": true
}
```
- **Response**: `200 OK`
- **Error Codes**:
    - `2012`: Failed to mount filesystem.

## Unmount Filesystem
### POST /api/v1/dataset/filesystem/unmount
- **Description**: Unmounts a filesystem.
- **Request Body**:
```json
{
    "name": "tank/fs1",
    "force": true
}
```
- **Response**: `204 No Content`
- **Error Codes**:
    - `2013`: Failed to unmount filesystem.

## List Volumes

### GET /api/v1/dataset/volumes

- **Description**: Fetches a list of all volumes.
- **Request Body**:

```json
{
    "recursive": true
}
```

- **Response**:

```json
{
    "volumes": [
        "tank/vol1",
        "tank/vol2"
    ]
}
```

- **Error Codes**:
    - `2014`: Failed to list volumes.

## Create Volume

### POST /api/v1/dataset/volume

- **Description**: Creates a new volume.
- **Request Body**:

```json
{
    "name": "tank/vol1",
    "size": "10G",
    "properties": {
        "volblocksize": "128k"
    },
    "sparse": false
}
```

- **Response**: `201 Created`
- **Error Codes**:
    - `2015`: Failed to create volume.

## List Snapshots

### GET /api/v1/dataset/snapshots

- **Description**: Fetches a list of snapshots for a dataset.
- **Request Body**:

```json
{
    "name": "tank/fs1",
    "recursive": true
}
```

- **Response**:

```json
{
    "snapshots": [
        "tank/fs1@snap1",
        "tank/fs1@snap2"
    ]
}
```

- **Error Codes**:
    - `2016`: Failed to list snapshots.

## Create Snapshot

### POST /api/v1/dataset/snapshot

- **Description**: Creates a snapshot for a dataset.
- **Request Body**:

```json
{
    "name": "tank/fs1",
    "snap_name": "snap1",
    "recursive": true
}
```

- **Response**: `201 Created`
- **Error Codes**:
    - `2017`: Failed to create snapshot.

## Rollback Snapshot

### POST /api/v1/dataset/snapshot/rollback

- **Description**: Rolls back a dataset to a specified snapshot.
- **Request Body**:

```json
{
    "name": "tank/fs1@snap1",
    "destroy_recent": true
}
```

- **Response**: `200 OK`
- **Error Codes**:
    - `2018`: Failed to rollback snapshot.

## Create Clone

### POST /api/v1/dataset/clone

- **Description**: Creates a clone of a snapshot.
- **Request Body**:

```json
{
    "name": "tank/fs1@snap1",
    "clone_name": "tank/clone1",
    "properties": {
        "compression": "on"
    }
}
```

- **Response**: `201 Created`
- **Error Codes**:
    - `2019`: Failed to create clone.

## Promote Clone

### POST /api/v1/dataset/clone/promote

- **Description**: Promotes a clone to a primary dataset.
- **Request Body**:

```json
{
    "name": "tank/clone1"
}
```

- **Response**: `200 OK`
- **Error Codes**:
    - `2020`: Failed to promote clone.

## List Bookmarks

### GET /api/v1/dataset/bookmarks

- **Description**: Lists all bookmarks for a dataset.
- **Request Body**:

```json
{
    "name": "tank/fs1"
}
```

- **Response**:

```json
{
    "bookmarks": [
        "tank/fs1#bookmark1",
        "tank/fs1#bookmark2"
    ]
}
```

- **Error Codes**:
    - `2021`: Failed to list bookmarks.

## Create Bookmark

### POST /api/v1/dataset/bookmark

- **Description**: Creates a bookmark for a dataset snapshot.
- **Request Body**:

```json
{
    "name": "tank/fs1@snap1",
    "bookmark_name": "bookmark1"
}
```

- **Response**: `201 Created`
- **Error Codes**:
    - `2022`: Failed to create bookmark.

## Send Dataset

### POST /api/v1/dataset/transfer/send

- **Description**: Sends a dataset to another system or file.
- **Request Body**:

```json
{
    "name": "tank/fs1",
    "destination": "/path/to/destination",
    "incremental": true,
    "resume": false
}
```

- **Response**: `200 OK`
- **Error Codes**:
    - `2023`: Failed to send dataset.

## Get Transfer Resume Token

### GET /api/v1/dataset/transfer/resume-token

- **Description**: Retrieves a resume token for a dataset transfer.
- **Request Body**:

```json
{
    "name": "tank/fs1"
}
```

- **Response**:

```json
{
    "resume_token": "token_string"
}
```

- **Error Codes**:
    - `2024`: Failed to get resume token.

