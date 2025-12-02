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

## Phase 4: Integration Implementation

### Writer Integration (`internal/config/writer.go`)

**Backup Manager Field**: Added optional `backupManager *backup.Manager` field to Writer struct:

- `NewWriterWithBackup()`: Creates writer with backup manager
- `NewWriterWithDirsAndBackup()`: For testing with custom directories and backup
- Maintains backward compatibility with existing `NewWriter()` and `NewWriterWithDirs()`

**Automatic Backups**: `WriteFile()` method updated with `backupBeforeWrite()` helper:

- Checks if backupManager is configured (nil = no backups)
- Validates backup is enabled via `cfg.Backup.IsEnabled()`
- Validates auto_backup is enabled via `cfg.Backup.IsAutoBackupEnabled()`
- Only backs up if target file exists (nothing to backup on first write)
- Supports async backups: `cfg.Backup.IsAsyncBackupEnabled()` runs backup in goroutine
- Supports sync backups: Waits for completion, returns error if backup fails
- Uses `TriggerAutomatic` trigger for all automatic backups

**Integration Pattern**:

```go
// With backup manager
writer := config.NewWriterWithBackup(backupMgr)
err := writer.WriteFile(path, cfg) // Automatically backs up before write

// Without backup manager (backward compatible)
writer := config.NewWriter()
err := writer.WriteFile(path, cfg) // No backup
```

### Init Command Integration (`cmd/klaudiush/init.go`)

**Backup Before Force**: `backupBeforeForce()` function creates backup when `--force` flag used:

- Detects first-run by checking if config already exists
- Creates appropriate storage for global or project config
- Uses `TriggerBeforeInit` trigger
- Includes "init --force" in snapshot metadata
- Logs backup snapshot ID on success
- Non-blocking: Logs warning on failure but continues with init

**Modified Flow**:

1. Check if config exists (`checkExistingConfig()` now returns existence flag)
2. If `--force` and config exists, call `backupBeforeForce()`
3. Continue with normal init flow (TUI, write config)

### Main Entry Point Integration (`cmd/klaudiush/main.go`)

**First-Run Migration**: `performFirstRunMigration()` creates initial backups for existing users:

- Uses marker file `~/.klaudiush/.migration_v1` to track completion
- Runs once on first execution after upgrade
- Backs up both global and project configs if they exist
- Uses `TriggerMigration` trigger
- Includes "first-run migration" in snapshot metadata
- Non-blocking: Logs errors but continues execution
- Creates marker file after successful migration

**Migration Flow**:

1. Check for migration marker file
2. If not exists, backup global config (if present)
3. Backup project config in current directory (if present)
4. Create marker file to prevent re-running
5. Log completion

**Helper Function**: `backupConfigIfExists()` handles individual config backup:

- Checks if config file exists
- Creates appropriate storage (global or project)
- Creates backup manager with default config
- Uses `TriggerMigration` trigger
- Logs snapshot ID on success

### Testing

**Writer Integration Tests** (`internal/config/writer_test.go`):

- WriteFile without backup manager (backward compatibility)
- WriteFile with no existing file (no backup created)
- WriteFile with existing file and sync backup (backup created with TriggerAutomatic)
- WriteFile with existing file and async backup (non-blocking)
- WriteFile with backup disabled (no backup created)
- WriteFile with auto_backup disabled (no backup created)
- WriteGlobal with backup (integration test)
- WriteProject with backup (integration test)

Coverage: 75.0% for internal/config package (added ~11% from Phase 4 integration)

### Integration Patterns

**Automatic Backups on Config Changes**:

```go
// Setup
storage, _ := backup.NewFilesystemStorage(baseDir, configType, projectPath)
manager, _ := backup.NewManager(storage, &config.BackupConfig{})
writer := config.NewWriterWithBackup(manager)

// Write triggers automatic backup
cfg := &config.Config{Backup: &config.BackupConfig{}}
writer.WriteFile(path, cfg) // Backs up existing file, then writes
```

**Manual Backup Before Risky Operations**:

```go
// Init with --force
if forceFlag && configExists {
    backupBeforeForce(configPath) // Explicit backup
}
runInitForm() // Then proceed with overwrite
```

**First-Run Migration for Existing Users**:

```go
// On first execution after upgrade
performFirstRunMigration(homeDir, log)
// Creates backups of existing configs
// Marker prevents re-running
```

### Error Handling

- Async backup errors: Silently ignored (background operation)
- Sync backup errors: Returned to caller, prevents write
- Init backup errors: Logged as warning, init continues
- Migration errors: Logged, dispatcher continues

### Backward Compatibility

- All existing code works without changes
- BackupManager is optional (nil = no backups)
- Writers without backup manager behave identically to before
- New constructors added, old ones unchanged

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

## Phase 6: Audit System Implementation (2025-12-02)

### Overview

Implemented comprehensive audit logging for backup operations using JSONL format. All backup operations (create, restore, delete, prune) are now logged with metadata for accountability and troubleshooting.

### Core Components

**AuditLogger Interface** (`internal/backup/audit.go`):

- `Log(entry AuditEntry)` - Records audit entry
- `Query(filter AuditFilter)` - Retrieves entries with filters
- `Close()` - Cleanup (no-op for JSONL implementation)

**AuditEntry Structure**:

```go
type AuditEntry struct {
    Timestamp  time.Time      // When operation occurred
    Operation  string         // create/restore/delete/prune/list/get
    ConfigPath string         // Config file path (optional)
    SnapshotID string         // Snapshot ID (optional)
    User       string         // Username
    Hostname   string         // Machine hostname
    Success    bool           // Operation success/failure
    Error      string         // Error message (if failed)
    Extra      map[string]any // Operation-specific metadata
}
```

**AuditFilter Structure**:

```go
type AuditFilter struct {
    Operation  string    // Filter by operation type
    Since      time.Time // Entries after this time
    SnapshotID string    // Filter by snapshot ID
    Success    *bool     // Filter by success/failure (nil = all)
    Limit      int       // Max entries to return (0 = all)
}
```

**JSONLAuditLogger Implementation**:

- Writes to `~/.klaudiush/.backups/audit.jsonl`
- Thread-safe with mutex protection
- Append-only JSONL format (one JSON object per line)
- Skips invalid entries during query (resilient to corruption)
- Auto-creates directory structure with secure permissions (0o700/0o600)

### Manager Integration

**NewManagerWithAudit Constructor**:

```go
manager, err := backup.NewManagerWithAudit(storage, cfg, auditLogger)
```

**Audit Logging Points**:

1. **CreateBackup**: Logs both success and failure
   - Success: Includes size, storage_type, trigger in Extra
   - Failure: Includes error message (e.g., failed to save index)

2. **RestoreSnapshot**: Logs restore operations
   - Success: Includes bytes_restored, checksum_verified, backup_created
   - Failure: Includes error message

3. **ApplyRetention**: Logs pruning operations
   - Success: Includes snapshots_removed, chains_removed, bytes_freed
   - Failure: Includes error message

**Helper Methods**:

- `logAuditEntry()` - Best-effort logging (ignores errors)
- `getCurrentUser()` - Gets username from USER/USERNAME env
- `getHostname()` - Gets machine hostname

### CLI Integration

**New Audit Subcommand** (`klaudiush backup audit`):

```bash
# List all audit entries
klaudiush backup audit

# Filter by operation type
klaudiush backup audit --operation create
klaudiush backup audit --operation restore

# Filter by time
klaudiush backup audit --since "2025-01-01T00:00:00Z"

# Filter by snapshot
klaudiush backup audit --snapshot abc123

# Filter by success/failure
klaudiush backup audit --success
klaudiush backup audit --failure

# Limit results
klaudiush backup audit --limit 20

# JSON output
klaudiush backup audit --json
```

**Output Formats**:

- **Table**: Human-readable with timestamps, operations, status (✅/❌)
- **JSON**: Machine-readable for scripting/parsing

### Key Design Decisions

**Best-Effort Logging**: Audit logging is best-effort - failures don't block operations:

- Prevents audit system from breaking backup functionality
- Errors are silently discarded in `logAuditEntry()`
- Appropriate for audit logs (observability, not critical path)

**JSONL Format**: One JSON object per line:

- Easy to append without reading entire file
- Resilient to partial writes (corruption affects single line)
- Query operation skips invalid lines automatically
- Standard format for log aggregation tools

**Thread Safety**: Mutex protection for concurrent operations:

- Single mutex for both Log and Query operations
- Prevents concurrent writes corrupting file
- Tested with 100 concurrent goroutines (10 operations each)

**Optional Integration**: AuditLogger is optional in Manager:

- Backward compatible with existing code
- NewManager() creates manager without audit logging
- NewManagerWithAudit() enables audit logging
- CLI commands don't yet use audit logging (future enhancement)

### Testing Strategy

**Comprehensive Test Coverage** (85.1% overall):

1. **Basic Operations**:
   - Creating logger with valid/invalid paths
   - Logging entries with various fields
   - Querying with no filters (returns all)
   - Empty log file handling

2. **Filtering**:
   - By operation type (create, restore, delete, prune)
   - By time (Since filter)
   - By snapshot ID
   - By success/failure status
   - Multiple filters combined
   - Limit results

3. **Error Handling**:
   - Nonexistent log file (returns empty list)
   - Invalid JSON entries (skipped during query)
   - Directory creation on first write

4. **Concurrency**:
   - 100 concurrent writers (1000 total entries)
   - 10 concurrent readers
   - No race conditions detected

### Integration with Existing Code

**Manager Refactoring**:

- Extracted helper methods to reduce CreateBackup length (funlen linter)
- `createSnapshotRecord()` - Creates snapshot structure
- `saveSnapshotToIndex()` - Adds to index and saves
- `logCreateSuccess()` - Logs successful creation
- Function now 89 lines (under 100-line limit)

**No Breaking Changes**:

- Existing NewManager() still works
- NewManagerWithAudit() is additive
- All existing tests pass without modification

### Future Enhancements

**CLI Integration** (Phase 7 - Doctor):

- Enable audit logging in setupBackupManagers()
- Audit log rotation and cleanup
- Audit log integrity checking in doctor

**Advanced Filtering**:

- Filter by user
- Filter by hostname
- Date range queries
- Full-text search in error messages

**Retention**:

- Automatic cleanup of old audit entries
- Configurable retention period
- Log rotation based on size

### Files Modified/Created

**Created**:

- `internal/backup/audit.go` - AuditLogger interface and implementation
- `internal/backup/audit_test.go` - Comprehensive test suite

**Modified**:

- `internal/backup/manager.go` - Added audit logging integration
- `cmd/klaudiush/backup.go` - Added audit subcommand

### Metrics

- **Test Coverage**: 85.1% (internal/backup)
- **Lint Issues**: 0
- **Tests**: All passing
- **Audit Tests**: 15 test cases covering all functionality
