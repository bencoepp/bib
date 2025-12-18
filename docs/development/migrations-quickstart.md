# Quick Start: Database Migrations

## Running Migrations

Migrations run automatically when bibd starts. To verify:

```bash
# Check current migration status
bib admin migrate status

# List all migrations
bib admin migrate list
```

## Creating a New Migration

### 1. Create Migration Files

For PostgreSQL:
```bash
# Up migration
touch internal/storage/migrate/migrations/postgres/000006_my_feature.up.sql

# Down migration  
touch internal/storage/migrate/migrations/postgres/000006_my_feature.down.sql
```

For SQLite (if applicable):
```bash
touch internal/storage/migrate/migrations/sqlite/000006_my_feature.up.sql
touch internal/storage/migrate/migrations/sqlite/000006_my_feature.down.sql
```

### 2. Write the Up Migration

```sql
-- 000006_my_feature.up.sql
-- Add new feature table

CREATE TABLE IF NOT EXISTS my_feature (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Add index
CREATE INDEX idx_my_feature_status ON my_feature(status);

-- Add trigger for updated_at
DROP TRIGGER IF EXISTS update_my_feature_updated_at ON my_feature;
CREATE TRIGGER update_my_feature_updated_at
    BEFORE UPDATE ON my_feature
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Add RLS policy
ALTER TABLE my_feature ENABLE ROW LEVEL SECURITY;

CREATE POLICY my_feature_read_all ON my_feature
    FOR SELECT
    USING (true);

CREATE POLICY my_feature_write_admin ON my_feature
    FOR ALL
    USING (current_user IN ('bibd_admin', 'bibd_admin_jobs'))
    WITH CHECK (current_user IN ('bibd_admin', 'bibd_admin_jobs'));
```

### 3. Write the Down Migration (Safe Rollback)

```sql
-- 000006_my_feature.down.sql
-- Safe rollback: preserve data

-- Drop RLS policies
DROP POLICY IF EXISTS my_feature_write_admin ON my_feature;
DROP POLICY IF EXISTS my_feature_read_all ON my_feature;
ALTER TABLE my_feature DISABLE ROW LEVEL SECURITY;

-- Drop triggers
DROP TRIGGER IF EXISTS update_my_feature_updated_at ON my_feature;

-- Drop indexes
DROP INDEX IF EXISTS idx_my_feature_status;

-- Rename table for safe keeping (instead of DROP)
DO $$
DECLARE
    backup_suffix TEXT := '_backup_' || to_char(NOW(), 'YYYYMMDD_HH24MISS');
BEGIN
    EXECUTE 'ALTER TABLE IF EXISTS my_feature RENAME TO my_feature' || backup_suffix;
    RAISE NOTICE 'Table renamed to: my_feature%', backup_suffix;
    RAISE NOTICE 'To permanently delete, manually: DROP TABLE my_feature%', backup_suffix;
END $$;
```

### 4. Test the Migration

```bash
# Apply migration
bib admin migrate up

# Check status
bib admin migrate status

# List migrations (verify it shows as applied)
bib admin migrate list

# Test rollback
bib admin migrate down --confirm

# Re-apply
bib admin migrate up
```

### 5. For SQLite

Adapt the PostgreSQL migration for SQLite:

```sql
-- SQLite doesn't have:
-- - UUID type (use TEXT)
-- - gen_random_uuid() (generate in application)
-- - TIMESTAMPTZ (use TEXT with ISO8601 format)
-- - DO blocks (use direct statements)
-- - RLS policies (skip or use triggers)

-- Example SQLite version:
CREATE TABLE IF NOT EXISTS my_feature (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive')),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX idx_my_feature_status ON my_feature(status);

-- Down migration for SQLite is simpler:
DROP INDEX IF EXISTS idx_my_feature_status;
ALTER TABLE my_feature RENAME TO my_feature_backup;
```

## Common Patterns

### Adding a Column

```sql
-- Up
ALTER TABLE my_table ADD COLUMN new_field TEXT;

-- Down
ALTER TABLE my_table DROP COLUMN new_field;
```

### Adding an Index

```sql
-- Up
CREATE INDEX idx_table_field ON my_table(field);

-- Down
DROP INDEX IF EXISTS idx_table_field;
```

### Adding a Foreign Key

```sql
-- Up
ALTER TABLE child_table 
    ADD CONSTRAINT fk_parent 
    FOREIGN KEY (parent_id) REFERENCES parent_table(id) ON DELETE CASCADE;

-- Down
ALTER TABLE child_table DROP CONSTRAINT fk_parent;
```

### Modifying a Column (requires data migration)

```sql
-- Up: Create new column, copy data, drop old
ALTER TABLE my_table ADD COLUMN new_status TEXT;
UPDATE my_table SET new_status = old_status::TEXT;
ALTER TABLE my_table DROP COLUMN old_status;
ALTER TABLE my_table RENAME COLUMN new_status TO status;

-- Down: Reverse the process
ALTER TABLE my_table ADD COLUMN old_status INT;
UPDATE my_table SET old_status = status::INT;
ALTER TABLE my_table DROP COLUMN status;
ALTER TABLE my_table RENAME COLUMN old_status TO status;
```

## Troubleshooting

### "Migration checksum mismatch"

You modified an already-applied migration. **Never do this!** Instead:
1. Revert the change
2. Create a new migration with the fix

### "Dirty database"

A migration failed midway:
1. Connect to database manually
2. Check what was applied
3. Fix or revert partial changes
4. Force version: `bib admin migrate force <version>`
5. Resume: `bib admin migrate up`

### "Lock timeout"

Another process is running migrations:
1. Wait for it to complete
2. Or increase timeout in config:
   ```yaml
   database:
     migrations:
       lock_timeout_seconds: 30
   ```

## Best Practices

✅ **DO:**
- Keep migrations small and focused
- Test both up and down migrations
- Use safe down migrations (rename, not drop)
- Add comments explaining complex migrations
- Version control migration files with code
- Use CHECK constraints for validation
- Add indexes for query optimization

❌ **DON'T:**
- Modify applied migrations
- Drop tables in down migrations (rename instead)
- Make breaking schema changes without a migration path
- Skip testing down migrations
- Commit untested migrations
- Disable checksum verification in production

## Configuration

```yaml
# config.yaml
database:
  migrations:
    verify_checksums: true           # Always verify in production
    on_checksum_mismatch: "fail"     # Fail fast on tampering
    lock_timeout_seconds: 15         # Reasonable default
```

## Need Help?

- 📖 Full docs: `internal/storage/migrate/README.md`
- 🔍 Examples: See existing migrations in `internal/storage/migrate/migrations/`
- 🎯 Implementation details: `docs/storage/PHASE_2.6_IMPLEMENTATION.md`

