// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package sqlstore

import (
	"database/sql"
)

type upgradeFunc func(*sql.Tx, *Container) error

// Upgrades is a list of functions that will upgrade a database to the latest version.
//
// This may be of use if you want to manage the database fully manually, but in most cases you
// should just call Container.Upgrade to let the library handle everything.
var Upgrades = [...]upgradeFunc{upgradeV1, upgradeV2, upgradeV3, upgradeV4}

func (c *Container) getVersion() (int, error) {
	_, err := c.db.Exec(c.query.CreateTableVersion())
	if err != nil {
		return -1, err
	}

	version := 0
	row := c.db.QueryRow(c.query.GetVersion())
	if row != nil {
		_ = row.Scan(&version)
	}
	return version, nil
}

func (c *Container) setVersion(tx *sql.Tx, version int) error {
	_, err := tx.Exec(c.query.DeleteAllVersions())
	if err != nil {
		return err
	}
	_, err = tx.Exec(c.query.InsertNewVersion(), version)
	return err
}

// Upgrade upgrades the database from the current to the latest version available.
func (c *Container) Upgrade() error {
	version, err := c.getVersion()
	if err != nil {
		return err
	}

	for ; version < len(Upgrades); version++ {
		var tx *sql.Tx
		tx, err = c.db.Begin()
		if err != nil {
			return err
		}

		migrateFunc := Upgrades[version]
		c.log.Infof("Upgrading database to v%d", version+1)
		err = migrateFunc(tx, c)
		if err != nil {
			_ = tx.Rollback()
			return err
		}

		if err = c.setVersion(tx, version+1); err != nil {
			return err
		}

		if err = tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}

func upgradeV1(tx *sql.Tx, container *Container) error {
	_, err := tx.Exec(container.query.CreateTableDevice())
	if err != nil {
		return err
	}
	_, err = tx.Exec(container.query.CreateTableIdentityKeys())
	if err != nil {
		return err
	}
	_, err = tx.Exec(container.query.CreateTablePreKeys())
	if err != nil {
		return err
	}
	_, err = tx.Exec(container.query.CreateTableSessions())
	if err != nil {
		return err
	}
	_, err = tx.Exec(container.query.CreateTableSenderKeys())
	if err != nil {
		return err
	}
	_, err = tx.Exec(container.query.CreateTableStateSyncKeys())
	if err != nil {
		return err
	}
	_, err = tx.Exec(container.query.CreateTableStateVersion())
	if err != nil {
		return err
	}
	_, err = tx.Exec(container.query.CreateTableStateMutationMacs())
	if err != nil {
		return err
	}
	_, err = tx.Exec(container.query.CreateTableContacts())
	if err != nil {
		return err
	}
	_, err = tx.Exec(container.query.CreateTableChatSettings())
	if err != nil {
		return err
	}
	return nil
}

func upgradeV2(tx *sql.Tx, container *Container) error {
	_, err := tx.Exec(container.query.AlterTableDevice_AddColumnSigKey())
	if err != nil {
		return err
	}
	_, err = tx.Exec(container.query.FillSigKey())
	if err != nil {
		return err
	}
	_, err = tx.Exec(container.query.DeleteNullSigKeys())
	if err != nil {
		return err
	}
	_, err = tx.Exec(container.query.AlterTableDevice_SetNotNull())
	return err
}

func upgradeV3(tx *sql.Tx, container *Container) error {
	_, err := tx.Exec(container.query.CreateTableMessageSecrets())
	return err
}

func upgradeV4(tx *sql.Tx, container *Container) error {
	_, err := tx.Exec(container.query.CreateTablePrivacyTokens())
	return err
}
