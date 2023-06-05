package database

import (
	"context"
	"database/sql"
	"testing"
)

type execer interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

func mustExec(ctx context.Context, t *testing.T, db execer, query string, args ...interface{}) {
	t.Helper()
	_, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		t.Fatalf("failed to execute %q: %v", query, err)
	}
}

func TestTruncateAll(t *testing.T) {
	SkipIfNoTestDatabase(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("normal", func(t *testing.T) {
		db, cleanup := SetupTestDB()
		defer cleanup()

		mustExec(ctx, t, db, "CREATE TABLE `users` (`id` BIGINT unsigned NOT NULL AUTO_INCREMENT, `name` VARCHAR(191) NOT NULL, PRIMARY KEY (`id`))")
		mustExec(ctx, t, db, "INSERT INTO `users` (`name`) VALUES (?), (?), (?)", "Alice", "Bob", "Charlie")
		mustExec(ctx, t, db, "CREATE VIEW `user_ids` AS SELECT `id` FROM `users`")

		var count uint64
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM `users`").Scan(&count); err != nil {
			t.Fatalf("failed to count rows: %v", err)
		}
		if count != 3 {
			t.Errorf("want 3 rows, got %d rows", count)
		}

		err := TruncateAll(ctx, db)
		if err != nil {
			t.Fatalf("failed to truncate: %v", err)
		}

		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM `users`").Scan(&count); err != nil {
			t.Fatalf("failed to count rows: %v", err)
		}
		if count != 0 {
			t.Errorf("want no row, got %d rows", count)
		}
	})

	t.Run("forgeign key", func(t *testing.T) {
		db, cleanup := SetupTestDB()
		defer cleanup()

		mustExec(ctx, t, db, "CREATE TABLE `users` (`id` BIGINT unsigned NOT NULL AUTO_INCREMENT, `name` VARCHAR(191) NOT NULL, PRIMARY KEY (`id`))")
		mustExec(ctx, t, db, "INSERT INTO `users` (`name`) VALUES (?), (?), (?)", "Alice", "Bob", "Charlie")
		mustExec(ctx, t, db, "CREATE TABLE `items` (`id` BIGINT unsigned NOT NULL AUTO_INCREMENT, `name` VARCHAR(191) NOT NULL, PRIMARY KEY (`id`))")
		mustExec(ctx, t, db, "INSERT INTO `items` (`name`) VALUES (?), (?), (?)", "item1", "item2", "item3")

		mustExec(ctx, t, db, "CREATE TABLE `user_items` ("+
			"`user_id` BIGINT unsigned NOT NULL, "+
			"`item_id` BIGINT unsigned NOT NULL, "+
			"PRIMARY KEY (`user_id`, `item_id`), "+
			"FOREIGN KEY (`user_id`) REFERENCES `users`(`id`), "+
			"FOREIGN KEY (`item_id`) REFERENCES `items`(`id`)"+
			")")
		mustExec(ctx, t, db, "INSERT INTO `user_items` (`user_id`, `item_id`) VALUES (?, ?), (?, ?), (?, ?)", 1, 1, 2, 1, 3, 3)

		var count uint64
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM `users`").Scan(&count); err != nil {
			t.Fatalf("failed to count rows: %v", err)
		}
		if count != 3 {
			t.Errorf("want 3 rows, got %d rows", count)
		}

		err := TruncateAll(ctx, db)
		if err != nil {
			t.Fatalf("failed to truncate: %v", err)
		}

		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM `users`").Scan(&count); err != nil {
			t.Fatalf("failed to count rows: %v", err)
		}
		if count != 0 {
			t.Errorf("want no row, got %d rows", count)
		}
	})
}

func TestDropAll(t *testing.T) {
	SkipIfNoTestDatabase(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t.Run("normal", func(t *testing.T) {
		db, cleanup := SetupTestDB()
		defer cleanup()

		// prepare tables
		mustExec(ctx, t, db, "CREATE TABLE `users` (`id` BIGINT unsigned NOT NULL AUTO_INCREMENT, `name` VARCHAR(191) NOT NULL, PRIMARY KEY (`id`))")
		mustExec(ctx, t, db, "INSERT INTO `users` (`name`) VALUES (?), (?), (?)", "Alice", "Bob", "Charlie")
		mustExec(ctx, t, db, "CREATE VIEW `user_ids` AS SELECT `id` FROM `users`")

		tables, views, err := ListTables(ctx, db)
		if err != nil {
			t.Fatal(err)
		}
		if len(tables) != 1 {
			t.Errorf("want one table, got %d tables", len(tables))
		}
		if len(views) != 1 {
			t.Errorf("want one view, got %d views", len(views))
		}

		// drop all tables
		if err := DropAll(ctx, db); err != nil {
			t.Fatalf("failed to drop: %v", err)
		}

		// check the result
		tables, views, err = ListTables(ctx, db)
		if err != nil {
			t.Fatal(err)
		}
		if len(tables) != 0 {
			t.Errorf("want no table, got %d tables", len(tables))
		}
		if len(views) != 0 {
			t.Errorf("want no view, got %d views", len(views))
		}
	})

	t.Run("foreign key", func(t *testing.T) {
		db, cleanup := SetupTestDB()
		defer cleanup()

		// prepare tables
		mustExec(ctx, t, db, "CREATE TABLE `users` (`id` BIGINT unsigned NOT NULL AUTO_INCREMENT, `name` VARCHAR(191) NOT NULL, PRIMARY KEY (`id`))")
		mustExec(ctx, t, db, "INSERT INTO `users` (`name`) VALUES (?), (?), (?)", "Alice", "Bob", "Charlie")
		mustExec(ctx, t, db, "CREATE TABLE `items` (`id` BIGINT unsigned NOT NULL AUTO_INCREMENT, `name` VARCHAR(191) NOT NULL, PRIMARY KEY (`id`))")
		mustExec(ctx, t, db, "INSERT INTO `items` (`name`) VALUES (?), (?), (?)", "item1", "item2", "item3")

		mustExec(ctx, t, db, "CREATE TABLE `user_items` ("+
			"`user_id` BIGINT unsigned NOT NULL, "+
			"`item_id` BIGINT unsigned NOT NULL, "+
			"PRIMARY KEY (`user_id`, `item_id`), "+
			"FOREIGN KEY (`user_id`) REFERENCES `users`(`id`), "+
			"FOREIGN KEY (`item_id`) REFERENCES `items`(`id`)"+
			")")
		mustExec(ctx, t, db, "INSERT INTO `user_items` (`user_id`, `item_id`) VALUES (?, ?), (?, ?), (?, ?)", 1, 1, 2, 1, 3, 3)

		tables, views, err := ListTables(ctx, db)
		if err != nil {
			t.Fatal(err)
		}
		if len(tables) != 3 {
			t.Errorf("want 3 tables, got %d tables", len(tables))
		}
		if len(views) != 0 {
			t.Errorf("want no view, got %d views", len(views))
		}

		// drop all tables
		if err := DropAll(ctx, db); err != nil {
			t.Fatalf("failed to drop: %v", err)
		}

		// check the result
		tables, views, err = ListTables(ctx, db)
		if err != nil {
			t.Fatal(err)
		}
		if len(tables) != 0 {
			t.Errorf("want no table, got %d tables", len(tables))
		}
		if len(views) != 0 {
			t.Errorf("want no view, got %d views", len(views))
		}
	})
}
