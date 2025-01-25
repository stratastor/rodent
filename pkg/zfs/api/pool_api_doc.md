# Pool API Documentation

## Create Pool

### POST /api/v1/pools

- **Description**: Creates a new ZFS pool.
- **Request Body**:

```json
{
    "name": "tank",
    "vdevs": [
        {
            "type": "mirror",
            "devices": ["/dev/sda", "/dev/sdb"]
        }
    ],
    "properties": {
        "ashift": "12",
        "autoexpand": "on"
    }
}
```

- **Response**: `201 Created`
- **Error Codes**:
    - `3001`: Failed to create pool.

## List Pools

### GET /api/v1/pools

- **Description**: Lists all ZFS pools.
- **Request Body**: None
- **Response**:

```json
{
    "pools": [
        "tank",
        "backup"
    ]
}
```

- **Error Codes**:
    - `3002`: Failed to list pools.

## Destroy Pool

### DELETE /api/v1/pools/:name

- **Description**: Destroys a ZFS pool.
- **Request Body**: None
- **Response**: `204 No Content`
- **Error Codes**:
    - `3003`: Failed to destroy pool.

## Import Pool

### POST /api/v1/pools/import

- **Description**: Imports an existing ZFS pool.
- **Request Body**:

```json
{
    "name": "tank",
    "guid": "1234567890",
    "readonly": false
}
```

- **Response**: `200 OK`
- **Error Codes**:
    - `3004`: Failed to import pool.

## Export Pool

### POST /api/v1/pools/:name/export

- **Description**: Exports a ZFS pool.
- **Request Body**:

```json
{
    "force": true
}
```

- **Response**: `200 OK`
- **Error Codes**:
    - `3005`: Failed to export pool.

## Get Pool Status

### GET /api/v1/pools/:name/status

- **Description**: Retrieves the status of a ZFS pool.
- **Request Body**: None
- **Response**:

```json
{
    "status": {
        "name": "tank",
        "health": "ONLINE",
        "vdevs": [
            {
                "type": "mirror",
                "devices": [
                    { "name": "/dev/sda", "state": "ONLINE" },
                    { "name": "/dev/sdb", "state": "ONLINE" }
                ]
            }
        ]
    }
}
```

- **Error Codes**:
    - `3006`: Failed to retrieve pool status.

## Get Pool Property

### GET /api/v1/pools/:name/properties/:property

- **Description**: Retrieves a specific property of a ZFS pool.
- **Request Body**: None
- **Response**:

```json
{
    "property": {
        "name": "ashift",
        "value": "12",
        "source": "local"
    }
}
```

- **Error Codes**:
    - `3007`: Failed to retrieve pool property.

## Set Pool Property

### PUT /api/v1/pools/:name/properties/:property

- **Description**: Sets a specific property of a ZFS pool.
- **Request Body**:

```json
{
    "value": "13"
}
```

- **Response**: `200 OK`
- **Error Codes**:
    - `3008`: Failed to set pool property.

## Scrub Pool

### POST /api/v1/pools/:name/scrub

- **Description**: Initiates a scrub operation on a ZFS pool.
- **Request Body**: None
- **Response**: `200 OK`
- **Error Codes**:
    - `3009`: Failed to scrub pool.

## Resilver Pool

### POST /api/v1/pools/:name/resilver

- **Description**: Initiates a resilver operation on a ZFS pool.
- **Request Body**: None
- **Response**: `200 OK`
- **Error Codes**:
    - `3010`: Failed to resilver pool.

## Attach Device

### POST /api/v1/pools/:name/devices/attach

- **Description**: Attaches a device to a ZFS pool.
- **Request Body**:

```json
{
    "vdev": "mirror",
    "devices": ["/dev/sdc", "/dev/sdd"]
}
```

- **Response**: `200 OK`
- **Error Codes**:
    - `3011`: Failed to attach device.

## Detach Device

### POST /api/v1/pools/:name/devices/detach

- **Description**: Detaches a device from a ZFS pool.
- **Request Body**:

```json
{
    "device": "/dev/sdc"
}
```

- **Response**: `200 OK`
- **Error Codes**:
    - `3012`: Failed to detach device.

## Replace Device

### POST /api/v1/pools/:name/devices/replace

- **Description**: Replaces a device in a ZFS pool.
- **Request Body**:

```json
{
    "old_device": "/dev/sdc",
    "new_device": "/dev/sde"
}
```

- **Response**: `200 OK`
- **Error Codes**:
    - `3013`: Failed to replace device.
