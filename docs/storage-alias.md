# Storage Volume Aliases

PlainNAS supports user-defined aliases for storage volumes exposed via GraphQL.

Notes:
- `StorageVolume.id` is a stable identifier. Local disks use the filesystem UUID (`<uuid>`); remote mounts use `remote:<src>`.

- Mutation: set or clear an alias (empty string clears)

```
mutation {
  setStorageVolumeAlias(id: "XXXX-XXXX", alias: "Media Drive")
}
```

- Query: volumes include an optional `alias` field

```
query {
  storageVolumes { id name alias mountPoint fsType totalBytes }
}
```

Aliases are persisted in the embedded Pebble DB (`storage:volume_alias`) and survive restarts.
