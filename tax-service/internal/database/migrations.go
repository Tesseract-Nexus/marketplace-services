package database

import (
	"embed"
	"fmt"
	"log"
	"sort"
	"strings"

	"tax-service/internal/models"
	"gorm.io/gorm"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// MigrationRecord tracks which migrations have been applied
type MigrationRecord struct {
	ID        uint   `gorm:"primaryKey"`
	Version   string `gorm:"uniqueIndex;size:255"`
	AppliedAt int64  `gorm:"autoCreateTime"`
}

// RunMigrations runs all pending database migrations
func RunMigrations(db *gorm.DB) error {
	log.Println("Starting database migrations...")

	// Step 1: Create migration tracking table
	if err := db.AutoMigrate(&MigrationRecord{}); err != nil {
		return fmt.Errorf("failed to create migration tracking table: %w", err)
	}

	// Step 1.5: Run reset migration if needed (drops old tables for clean start)
	log.Println("  → Checking for reset migration...")
	if err := runResetMigration(db); err != nil {
		return fmt.Errorf("failed to run reset migration: %w", err)
	}

	// Step 2: Run GORM AutoMigrate for model schema (one by one for better error handling)
	log.Println("  → Running schema migrations...")
	modelsToMigrate := []struct {
		name  string
		model interface{}
	}{
		{"TaxJurisdiction", &models.TaxJurisdiction{}},
		{"TaxRate", &models.TaxRate{}},
		{"ProductTaxCategory", &models.ProductTaxCategory{}},
		{"TaxRateCategoryOverride", &models.TaxRateCategoryOverride{}},
		{"TaxExemptionCertificate", &models.TaxExemptionCertificate{}},
		{"TaxCalculationCache", &models.TaxCalculationCache{}},
		{"TaxNexus", &models.TaxNexus{}},
		{"TaxReport", &models.TaxReport{}},
	}
	for _, m := range modelsToMigrate {
		log.Printf("    → Migrating %s...", m.name)
		if err := db.AutoMigrate(m.model); err != nil {
			return fmt.Errorf("failed to auto-migrate %s: %w", m.name, err)
		}
		log.Printf("    ✓ %s migrated", m.name)
	}
	log.Println("  ✓ Schema migrations complete")

	// Step 2.5: Ensure unique indexes exist for ON CONFLICT clauses
	// GORM AutoMigrate doesn't add indexes to existing tables, so we create them explicitly
	log.Println("  → Ensuring unique indexes exist...")
	if err := ensureUniqueIndexes(db); err != nil {
		return fmt.Errorf("failed to create unique indexes: %w", err)
	}
	log.Println("  ✓ Unique indexes verified")

	// Step 3: Run SQL seed migrations
	log.Println("  → Running seed migrations...")
	if err := runSQLMigrations(db); err != nil {
		return fmt.Errorf("failed to run SQL migrations: %w", err)
	}
	log.Println("  ✓ Seed migrations complete")

	log.Println("✓ All database migrations complete")
	return nil
}

// runResetMigration runs the 000_reset_tax_tables.sql
// This migration is special - we ALWAYS reset if 004_seed_global_tax_data.sql hasn't been applied
// This ensures consistent IDs across all migrations
func runResetMigration(db *gorm.DB) error {
	resetFile := "000_reset_tax_tables.sql"
	globalDataFile := "004_seed_global_tax_data.sql"

	// Check if 004 (the last migration) was successfully applied
	// If not, we MUST reset to ensure consistent IDs
	var globalDataRecord MigrationRecord
	if err := db.Where("version = ?", globalDataFile).First(&globalDataRecord).Error; err == nil {
		// 004 was applied successfully, database is complete
		log.Printf("    → Skipping reset (004 already applied, database complete)")
		return nil
	}

	// 004 not applied - force a complete reset
	log.Printf("    → Database needs reset (004 not applied)")

	// Delete all migration records to force fresh start
	db.Where("version != ?", resetFile).Delete(&MigrationRecord{})
	db.Where("version = ?", resetFile).Delete(&MigrationRecord{})

	// Read and execute reset migration
	content, err := migrationsFS.ReadFile("migrations/" + resetFile)
	if err != nil {
		log.Printf("    → No reset migration found, skipping")
		return nil
	}

	log.Printf("    → Applying %s (clean start)...", resetFile)
	if err := executeSQLStatements(db, string(content)); err != nil {
		return fmt.Errorf("failed to execute reset migration: %w", err)
	}

	// Record migration as applied
	if err := db.Create(&MigrationRecord{Version: resetFile}).Error; err != nil {
		return fmt.Errorf("failed to record reset migration: %w", err)
	}

	log.Printf("    ✓ Applied %s", resetFile)

	// Explicitly delete all non-reset migration records (safety net)
	if err := db.Where("version != ?", resetFile).Delete(&MigrationRecord{}).Error; err != nil {
		log.Printf("    (warning: could not clear old migration records: %v)", err)
	}

	return nil
}

// runSQLMigrations executes embedded SQL migration files in order
func runSQLMigrations(db *gorm.DB) error {
	// Read all SQL files from embedded filesystem
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	// Sort files by name (ensures order: 001_, 002_, etc.)
	// Skip 001_create_tax_tables.sql since GORM AutoMigrate handles schema
	var fileNames []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			// Skip schema migration - GORM AutoMigrate handles table creation
			if strings.HasPrefix(entry.Name(), "001_") {
				continue
			}
			fileNames = append(fileNames, entry.Name())
		}
	}
	sort.Strings(fileNames)

	// Run each migration
	for _, fileName := range fileNames {
		// Check if migration already applied
		var record MigrationRecord
		result := db.Where("version = ?", fileName).First(&record)
		if result.Error == nil {
			log.Printf("    → Skipping %s (already applied)", fileName)
			continue
		}

		// Read and execute SQL file
		content, err := migrationsFS.ReadFile("migrations/" + fileName)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", fileName, err)
		}

		log.Printf("    → Applying %s...", fileName)

		// Execute the SQL (split by semicolon for multiple statements)
		if err := executeSQLStatements(db, string(content)); err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", fileName, err)
		}

		// Record migration as applied
		if err := db.Create(&MigrationRecord{Version: fileName}).Error; err != nil {
			return fmt.Errorf("failed to record migration %s: %w", fileName, err)
		}

		log.Printf("    ✓ Applied %s", fileName)
	}

	return nil
}

// executeSQLStatements executes a SQL script with multiple statements
func executeSQLStatements(db *gorm.DB, sql string) error {
	// Split by semicolon but be careful with strings containing semicolons
	statements := splitSQLStatements(sql)

	log.Printf("      (executing %d statements)", len(statements))
	executed := 0
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}

		// Strip leading comment lines (comments can precede SQL statements in the same "statement")
		lines := strings.Split(stmt, "\n")
		var sqlLines []string
		for _, line := range lines {
			trimmedLine := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmedLine, "--") && trimmedLine != "" {
				sqlLines = append(sqlLines, line)
			}
		}
		stmt = strings.TrimSpace(strings.Join(sqlLines, "\n"))
		if stmt == "" {
			continue
		}
		executed++

		// Log first 100 chars of each statement for debugging
		preview := stmt
		if len(preview) > 100 {
			preview = preview[:100] + "..."
		}
		log.Printf("      [%d/%d] %s", executed, len(statements), preview)

		result := db.Exec(stmt)
		if result.Error != nil {
			// Log the error but continue for non-critical errors like duplicate key
			if strings.Contains(result.Error.Error(), "duplicate key") ||
				strings.Contains(result.Error.Error(), "already exists") {
				log.Printf("      [%d/%d] SKIP (duplicate)", executed, len(statements))
				continue
			}
			log.Printf("      [%d/%d] FAIL: %v", executed, len(statements), result.Error)
			return result.Error
		}
		log.Printf("      [%d/%d] OK (rows: %d)", executed, len(statements), result.RowsAffected)

		// Debug: verify tax_jurisdictions count after INSERT
		if strings.Contains(stmt, "tax_jurisdictions") && strings.Contains(stmt, "INSERT") {
			var count int64
			db.Raw("SELECT COUNT(*) FROM tax_jurisdictions").Scan(&count)
			log.Printf("      [%d/%d] DEBUG: tax_jurisdictions now has %d rows", executed, len(statements), count)
		}
	}

	return nil
}

// ensureUniqueIndexes creates unique indexes required for ON CONFLICT clauses
// These indexes may not exist if tables were created before the GORM model tags were added
// GORM uses plural table names (tax_jurisdictions, tax_nexuses, etc.)
func ensureUniqueIndexes(db *gorm.DB) error {
	indexes := []struct {
		name  string
		sql   string
		table string
	}{
		// TaxJurisdiction: unique on (tenant_id, type, code) for ON CONFLICT clauses
		{
			name:  "idx_jurisdiction_unique",
			sql:   `CREATE UNIQUE INDEX IF NOT EXISTS idx_jurisdiction_unique ON tax_jurisdictions (tenant_id, type, code)`,
			table: "tax_jurisdictions",
		},
		// ProductTaxCategory: unique on (tenant_id, name)
		{
			name:  "idx_category_unique",
			sql:   `CREATE UNIQUE INDEX IF NOT EXISTS idx_category_unique ON product_tax_categories (tenant_id, name)`,
			table: "product_tax_categories",
		},
		// TaxExemptionCertificate: unique on (tenant_id, customer_id, certificate_number)
		{
			name:  "idx_exemption_unique",
			sql:   `CREATE UNIQUE INDEX IF NOT EXISTS idx_exemption_unique ON tax_exemption_certificates (tenant_id, customer_id, certificate_number)`,
			table: "tax_exemption_certificates",
		},
		// TaxNexus: unique on (tenant_id, jurisdiction_id)
		// GORM creates 'tax_nexuses' (plural)
		{
			name:  "idx_nexus_unique",
			sql:   `CREATE UNIQUE INDEX IF NOT EXISTS idx_nexus_unique ON tax_nexuses (tenant_id, jurisdiction_id)`,
			table: "tax_nexuses",
		},
	}

	for _, idx := range indexes {
		// Check if table exists before trying to create index
		var exists bool
		checkSQL := fmt.Sprintf("SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = '%s')", idx.table)
		if err := db.Raw(checkSQL).Scan(&exists).Error; err != nil {
			log.Printf("    (warning: could not check table %s: %v)", idx.table, err)
			continue
		}
		if !exists {
			log.Printf("    (skipping index %s: table %s does not exist)", idx.name, idx.table)
			continue
		}

		if err := db.Exec(idx.sql).Error; err != nil {
			// Log but don't fail if index already exists with different name
			if strings.Contains(err.Error(), "already exists") {
				log.Printf("    (index %s already exists, skipping)", idx.name)
				continue
			}
			return err
		}
		log.Printf("    ✓ Created/verified index %s", idx.name)
	}

	return nil
}

// splitSQLStatements splits SQL content into individual statements
func splitSQLStatements(sql string) []string {
	var statements []string
	var currentStmt strings.Builder
	inString := false
	stringChar := rune(0)

	for i, char := range sql {
		// Track string literals to avoid splitting on semicolons within strings
		if (char == '\'' || char == '"') && (i == 0 || sql[i-1] != '\\') {
			if !inString {
				inString = true
				stringChar = char
			} else if char == stringChar {
				inString = false
			}
		}

		if char == ';' && !inString {
			stmt := strings.TrimSpace(currentStmt.String())
			if stmt != "" {
				statements = append(statements, stmt)
			}
			currentStmt.Reset()
		} else {
			currentStmt.WriteRune(char)
		}
	}

	// Add final statement if any
	stmt := strings.TrimSpace(currentStmt.String())
	if stmt != "" {
		statements = append(statements, stmt)
	}

	return statements
}
