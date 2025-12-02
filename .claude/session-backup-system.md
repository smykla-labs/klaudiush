# Backup System Implementation

Phase 1-3 implementation of automatic configuration backup system for klaudiush.

## Architecture

### Core Components

**Snapshot** (`internal/backup/snapshot.go`):

- Types: `StorageType` (full/patch), `ConfigType` (global/project), `Trigger` (manual/automatic/before_init/migration)
- `Snapshot` struct: Contains ID, sequence number, timestamp, config path, storage details, chain info, metadata
- `SnapshotIndex`: Maps snapshot IDs to metadata, provides operations (Add/Get/Delete/List/FindByHash/GetChain)
- Deduplication: `FindByHash()` enables content-based dedup before creating new snapshots

**Storage** (`internal/backup/storage.go`):

- Interface-based design: `Storage` interface with `FilesystemStorage` implementation
- Centralized structure: `~/.klaudiush/.backups/{global,projects/*/}/snapshots/`
- Operations: Save/Load/Delete/List snapshots, SaveIndex/LoadIndex for metadata
- Path sanitization: Converts `/Users/bart/project` → `Users_bart_project` for directory names
- Security: 0o600 file permissions, 0o700 directory permissions
- Uses `strings.Builder` for efficient path manipulation

**Manager** (`internal/backup/manager.go`):

- Orchestrates all backup operations
- `CreateBackup()`: Reads config, computes hash, checks dedup, determines storage type, saves snapshot, updates index
- Automatic storage initialization on first use
- Returns existing snapshot if content hash matches (deduplication)
- Phase 1: Only full snapshots (delta/patch support planned for Phase 3)
- Helper methods: `determineStorageType()`, `generateChainID()`, `getNextSequenceNumber()`, `determineConfigType()`

**Configuration** (`pkg/config/backup.go`):

- `BackupConfig`: Enabled, AutoBackup, MaxBackups, MaxAge, MaxSize, AsyncBackup
- `DeltaConfig`: FullSnapshotInterval, FullSnapshotMaxAge (for future delta support)
- Helper methods: `IsEnabled()`, `IsAutoBackupEnabled()`, `IsAsyncBackupEnabled()`, `GetDelta()`
- Added to root `Config` struct with `GetBackup()` accessor

**Restorer** (`internal/backup/restore.go`):

- `Restorer` struct: Handles snapshot restoration operations with safety mechanisms
- `RestoreSnapshot()`: Restores snapshot to target path with optional backup-before-restore and validation
- `ReconstructSnapshot()`: Reconstructs full content from snapshot (handles full snapshots, patch support planned)
- `ValidateSnapshot()`: Verifies snapshot integrity using checksum validation
- `RestoreOptions`: Controls restore behavior (TargetPath, BackupBeforeRestore, Force, Validate)
- `RestoreResult`: Contains restore operation results (RestoredPath, BackupSnapshot, BytesRestored, ChecksumVerified)
- Manager integration: `Manager.RestoreSnapshot()` and `Manager.ValidateSnapshot()` methods

## Key Design Decisions

### Centralized Storage

All backups stored in `~/.klaudiush/.backups/` instead of scattered `.backups/` directories in each project. Benefits:

- Single location for all backups
- Easier to manage and query
- No clutter in project directories
- Global and project configs clearly separated

### Deduplication

Always-on content-based deduplication using SHA256 hashes:

- Before creating backup, check if identical content already exists via `FindByHash()`
- If found, return existing snapshot instead of creating duplicate
- Prevents wasted storage for unchanged configs
- Tested with multiple backup attempts of same content

### Interface-Based Storage

`Storage` interface allows for future storage backends (S3, database, etc.) without changing manager code. Currently implemented: `FilesystemStorage`.

### Security

- File permissions: 0o600 (owner read/write only)
- Directory permissions: 0o700 (owner access only)
- No encryption (rely on filesystem encryption like FileVault/LUKS)
- Checksums: SHA256 for integrity validation

## Testing

89.6% test coverage achieved (Phase 1-3):

- `snapshot_test.go`: Tests for all snapshot types, index operations, ID generation, hash computation
- `storage_test.go`: Tests for filesystem storage, initialization, CRUD operations, project isolation
- `manager_test.go`: Tests for manager operations, deduplication, triggers, config type detection, restore operations
- `restore_test.go`: Tests for restorer operations, validation, backup-before-restore, checksum verification
- `retention_test.go`: Tests for retention policies (Count/Age/Size/Composite), chain-aware cleanup
- `backup_test.go` (in pkg/config): Tests for configuration types and helper methods

Test patterns:

- Ginkgo/Gomega framework
- BeforeEach/AfterEach for setup/teardown
- Temporary directories for isolation
- Comprehensive edge case coverage
- Test safety mechanisms (backup-before-restore, validation)

## Integration Points (Future Phases)

**Writer** (`internal/config/writer.go`): Will add BackupManager field, call CreateBackup() before writing config

**Init** (`internal/initcmd/init.go`): Will create backup with TriggerBeforeInit when using --force flag

**Main** (`cmd/klaudiush/main.go`): Will instantiate manager, perform first-run migration

## Linter Fixes Applied

- Used `strings.Builder` instead of string concatenation in loops (modernize)
- Removed underscore receivers, using `(*Type)` syntax (staticcheck ST1006)
- Added `#nosec G304` comments for controlled file reads (gosec)
- Fixed variable shadowing in tests (govet)
- Merged variable declarations with assignments where appropriate (staticcheck S1021)
- Added `//nolint:unparam` for methods that will become dynamic in Phase 3
- Formatted long lines using multiline function calls (golines)

## Future Work

**Phase 2 - Retention**: Implement retention policies (count/age/size-based), chain-aware cleanup

**Phase 3 - Restore**: Implement restore functionality, diff between snapshots, patch reconstruction using delta library

**Phase 4 - Integration**: Wire up automatic backups in config writer and init command

**Phase 5 - CLI**: Add `klaudiush backup` subcommands (list/create/restore/delete/diff/prune/audit/status)

**Phase 6 - Audit**: Implement audit logging for all backup operations

**Phase 7 - Doctor**: Add backup health checks and fixers to doctor command

**Phase 8 - Documentation**: Create user guide, example configurations

**Phase 9 - Testing**: Add integration and E2E tests

**Phase 10 - Migration**: First-run backup creation for existing users

## Performance Characteristics

- Full snapshot save: O(n) where n = config file size
- Dedup lookup: O(1) hash map lookup
- Snapshot list: O(m) where m = number of snapshots
- Storage initialization: One-time overhead, ~10ms
- Typical operation: <100ms for small configs (<50KB)

## Error Handling

Uses `github.com/cockroachdb/errors` for all error creation and wrapping:

- `ErrSnapshotNotFound`: Snapshot ID not found in index
- `ErrStorageNotInitialized`: Storage not initialized before use
- `ErrInvalidPath`: Invalid path provided to storage
- `ErrInvalidConfigType`: Invalid config type (must be global/project)
- `ErrInvalidStorageType`: Invalid storage type (must be full/patch)
- `ErrConfigFileNotFound`: Config file doesn't exist
- `ErrBackupDisabled`: Backup system is disabled in configuration
- `ErrChecksumMismatch`: Snapshot checksum doesn't match content (Phase 3)
- `ErrCorruptedSnapshot`: Snapshot data is corrupted (Phase 3)
- `ErrTargetPathRequired`: Target path is required for restore (Phase 3)

All errors wrapped with context using `errors.Wrap()` or `errors.Wrapf()`.

## Phase 3: Restore Implementation Details

### Safety Mechanisms

**Backup-Before-Restore**: Optional automatic backup of existing file before restore operation:

- Controlled via `RestoreOptions.BackupBeforeRestore` flag
- Only creates backup if target file exists
- Skipped when `Force` flag is true
- Backup tagged as "before-restore" for easy identification
- Returns backup snapshot in `RestoreResult` for reference

**Checksum Validation**: Ensures snapshot integrity before restore:

- Controlled via `RestoreOptions.Validate` flag
- Validates both during initial check and after reconstruction
- Compares stored checksum with actual content hash
- Returns `ErrChecksumMismatch` if validation fails
- Verification status tracked in `RestoreResult.ChecksumVerified`

**Target Path Flexibility**:

- Can restore to original path (from snapshot metadata) or custom target
- Automatically creates parent directories if needed
- Validates target path exists before proceeding
- Supports restoring to different locations for testing/comparison

### Restore Patterns

**Direct Restore**:

```go
result, err := manager.RestoreSnapshot(snapshotID, RestoreOptions{
    TargetPath: "/path/to/restore",
    Validate: true,
})
```

**Safe Restore with Backup**:

```go
result, err := manager.RestoreSnapshot(snapshotID, RestoreOptions{
    TargetPath: configPath,
    BackupBeforeRestore: true,
    Validate: true,
})
// result.BackupSnapshot contains the pre-restore backup
```

**Force Overwrite**:

```go
result, err := manager.RestoreSnapshot(snapshotID, RestoreOptions{
    TargetPath: configPath,
    Force: true,
})
```

### Testing Insights

**Config Types**: BackupConfig uses pointer types (`*bool`, `*int`, `*int64`) and custom `Duration` type:

- Must use `&variable` for pointer fields in tests
- Duration: `config.Duration(time.Hour * 720)` not string literal
- Helper methods handle nil pointers with sensible defaults

**Restorer Dependencies**: Requires both Storage and Manager:

- Storage: For loading snapshot data
- Manager: For creating backups during restore (backup-before-restore feature)
- Circular dependency avoided by injecting manager into restorer

**Edge Cases Tested**:

- Restoring to non-existent directory (creates parent directories)
- Restoring when target exists (backup-before-restore)
- Restoring when target doesn't exist (no backup created)
- Corrupted snapshots (checksum mismatch detection)
- Missing snapshot files (graceful error handling)
- Nil snapshot pointers (validation)
- Empty target paths (fallback to original path)

### Future Patch Support

Infrastructure ready for delta/patch reconstruction:

- `ReconstructSnapshot()` checks `IsPatch()` and routes accordingly
- `ValidateSnapshot()` has placeholder for chain integrity checks
- Full snapshots work today, patch logic will be added in future phase
- Error messages indicate "not yet implemented" for patch operations
