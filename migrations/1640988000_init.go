// Package migrations contains the system PocketBase DB migrations.
package migrations

import (
	"path/filepath"
	"runtime"

	"github.com/jojokbh/pocketbase/daos"
	"github.com/jojokbh/pocketbase/models"
	"github.com/jojokbh/pocketbase/models/schema"
	"github.com/jojokbh/pocketbase/models/settings"
	"github.com/jojokbh/pocketbase/tools/migrate"
	"github.com/jojokbh/pocketbase/tools/types"
	"github.com/pocketbase/dbx"
)

var AppMigrations migrate.MigrationsList

// Register is a short alias for `AppMigrations.Register()`
// that is usually used in external/user defined migrations.
func Register(
	up func(db dbx.Builder) error,
	down func(db dbx.Builder) error,
	optFilename ...string,
) {
	var optFiles []string
	if len(optFilename) > 0 {
		optFiles = optFilename
	} else {
		_, path, _, _ := runtime.Caller(1)
		optFiles = append(optFiles, filepath.Base(path))
	}
	AppMigrations.Register(up, down, optFiles...)
}

func init() {
	AppMigrations.Register(func(db dbx.Builder) error {
		_, tablesErr := db.NewQuery(`
			CREATE TABLE IF NOT EXISTS _admins (
				id              UUID PRIMARY KEY NOT NULL,
				avatar          INTEGER DEFAULT 0 NOT NULL,
				email           TEXT UNIQUE NOT NULL,
				"tokenKey"        TEXT UNIQUE NOT NULL,
				"passwordHash"    TEXT NOT NULL,
				"lastResetSentAt" TEXT DEFAULT '' NOT NULL,
				created         TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
				updated         TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
			);

			CREATE TABLE IF NOT EXISTS _collections (
				id         character varying(255) PRIMARY KEY NOT NULL,
				system     BOOLEAN DEFAULT FALSE NOT NULL,
				type       TEXT DEFAULT 'base' NOT NULL,
				name       TEXT UNIQUE NOT NULL,
				schema     JSONB DEFAULT '[]' NOT NULL,
				indexes    JSONB DEFAULT '[]' NOT NULL,
				"listRule"  TEXT DEFAULT NULL,
				"viewRule"   TEXT DEFAULT NULL,
				"createRule" TEXT DEFAULT NULL,
				"updateRule" TEXT DEFAULT NULL,
				"deleteRule" TEXT DEFAULT NULL,
				options    JSONB DEFAULT '{}' NOT NULL,
				created    TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
				updated    TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
			);

			CREATE TABLE IF NOT EXISTS _params (
				id      UUID PRIMARY KEY NOT NULL,
				key     TEXT UNIQUE NOT NULL,
				value   JSONB DEFAULT NULL,
				created TEXT DEFAULT '' NOT NULL,
				updated TEXT DEFAULT '' NOT NULL
			);

			CREATE TABLE IF NOT EXISTS "_externalAuths" (
				id           UUID PRIMARY KEY NOT NULL,
				"collectionId" character varying(255) NOT NULL,
				"recordId"     UUID NOT NULL,
				provider     TEXT NOT NULL,
				"providerId"   TEXT NOT NULL,
				created      TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
				updated      TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
				---
				FOREIGN KEY ("collectionId") REFERENCES _collections (id) ON UPDATE CASCADE ON DELETE CASCADE
			);

			CREATE UNIQUE INDEX IF NOT EXISTS "_externalAuths_record_provider_idx" on "_externalAuths" ("collectionId", "recordId", provider);
			CREATE UNIQUE INDEX IF NOT EXISTS "_externalAuths_provider_providerId_idx" on "_externalAuths" (provider, "providerId");
		`).Execute()
		if tablesErr != nil {
			return tablesErr
		}

		dao := daos.New(db, "postgres")

		// inserts default settings
		// -----------------------------------------------------------
		defaultSettings := settings.New()
		if err := dao.SaveSettings(defaultSettings); err != nil {
			return err
		}

		// inserts the system users collection
		// -----------------------------------------------------------
		usersCollection := &models.Collection{}
		usersCollection.MarkAsNew()
		usersCollection.Id = "_pb_users_auth_"
		usersCollection.Name = "users"
		usersCollection.Type = models.CollectionTypeAuth
		usersCollection.ListRule = types.Pointer("id = @request.auth.id")
		usersCollection.ViewRule = types.Pointer("id = @request.auth.id")
		usersCollection.CreateRule = types.Pointer("")
		usersCollection.UpdateRule = types.Pointer("id = @request.auth.id")
		usersCollection.DeleteRule = types.Pointer("id = @request.auth.id")

		// set auth options
		usersCollection.SetOptions(models.CollectionAuthOptions{
			ManageRule:        nil,
			AllowOAuth2Auth:   true,
			AllowUsernameAuth: true,
			AllowEmailAuth:    true,
			MinPasswordLength: 8,
			RequireEmail:      false,
		})

		// set optional default fields
		usersCollection.Schema = schema.NewSchema(
			&schema.SchemaField{
				Id:      "users_name",
				Type:    schema.FieldTypeText,
				Name:    "name",
				Options: &schema.TextOptions{},
			},
			&schema.SchemaField{
				Id:   "users_avatar",
				Type: schema.FieldTypeFile,
				Name: "avatar",
				Options: &schema.FileOptions{
					MaxSelect: 1,
					MaxSize:   5242880,
					MimeTypes: []string{
						"image/jpeg",
						"image/png",
						"image/svg+xml",
						"image/gif",
						"image/webp",
					},
				},
			},
		)

		return dao.SaveCollection(usersCollection)
	}, func(db dbx.Builder) error {
		tables := []string{
			"users",
			"_externalAuths",
			"_params",
			"_collections",
			"_admins",
		}

		for _, name := range tables {
			if _, err := db.DropTable(name).Execute(); err != nil {
				return err
			}
		}

		return nil
	})
}
