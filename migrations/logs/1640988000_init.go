package logs

import (
	"github.com/jojokbh/pocketbase/tools/migrate"
	"github.com/pocketbase/dbx"
)

var LogsMigrations migrate.MigrationsList

func init() {
	LogsMigrations.Register(func(db dbx.Builder) (err error) {
		_, err = db.NewQuery(`
			CREATE TABLE IF NOT EXISTS {{_requests}} (
				id        UUID PRIMARY KEY NOT NULL,
				url       TEXT DEFAULT '' NOT NULL,
				method    TEXT DEFAULT 'get' NOT NULL,
				status    INTEGER DEFAULT 200 NOT NULL,
				auth      TEXT DEFAULT 'guest' NOT NULL,
				ip        TEXT DEFAULT '127.0.0.1' NOT NULL,
				referer   TEXT DEFAULT '' NOT NULL,
				"userAgent" TEXT DEFAULT '' NOT NULL,
				meta      JSON DEFAULT '{}' NOT NULL,
				created   TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
				updated   TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
			);

			CREATE INDEX IF NOT EXISTS _request_status_idx on {{_requests}} (status);
			CREATE INDEX IF NOT EXISTS _request_auth_idx on {{_requests}} (auth);
			CREATE INDEX IF NOT EXISTS _request_ip_idx on {{_requests}} (ip);
			CREATE INDEX IF NOT EXISTS _request_created_hour_idx on {{_requests}} (created));
		`).Execute()

		return err
	}, func(db dbx.Builder) error {
		_, err := db.DropTable("_requests").Execute()
		return err
	})
}
