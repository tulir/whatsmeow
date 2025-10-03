// Copyright (c) 2025 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package sqlstore contains an SQL-backed implementation of the interfaces in the store package.
package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/rs/zerolog"
	"go.mau.fi/util/dbutil"
	"go.mau.fi/util/exslices"

	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
)

type CachedLIDMap struct {
	db *dbutil.Database

	pnToLIDCache map[string]string
	lidToPNCache map[string]string
	cacheFilled  bool
	lidCacheLock sync.RWMutex
}

var _ store.LIDStore = (*CachedLIDMap)(nil)

func NewCachedLIDMap(db *dbutil.Database) *CachedLIDMap {
	return &CachedLIDMap{
		db: db,

		pnToLIDCache: make(map[string]string),
		lidToPNCache: make(map[string]string),
	}
}

const (
	deleteExistingLIDMappingQuery = `DELETE FROM whatsmeow_lid_map WHERE (lid<>$1 AND pn=$2)`
	putLIDMappingQuery            = `
		INSERT INTO whatsmeow_lid_map (lid, pn)
		VALUES ($1, $2)
		ON CONFLICT (lid) DO UPDATE SET pn=excluded.pn WHERE whatsmeow_lid_map.pn<>excluded.pn
	`
	getLIDForPNQuery       = `SELECT lid FROM whatsmeow_lid_map WHERE pn=$1`
	getPNForLIDQuery       = `SELECT pn FROM whatsmeow_lid_map WHERE lid=$1`
	getAllLIDMappingsQuery = `SELECT lid, pn FROM whatsmeow_lid_map`
)

func (s *CachedLIDMap) FillCache(ctx context.Context) error {
	s.lidCacheLock.Lock()
	defer s.lidCacheLock.Unlock()
	rows, err := s.db.Query(ctx, getAllLIDMappingsQuery)
	if err != nil {
		return err
	}
	err = s.scanManyLids(rows, nil)
	s.cacheFilled = err == nil
	return err
}

func (s *CachedLIDMap) scanManyLids(rows dbutil.Rows, fn func(lid, pn string)) error {
	if fn == nil {
		fn = func(lid, pn string) {}
	}
	for rows.Next() {
		var lid, pn string
		err := rows.Scan(&lid, &pn)
		if err != nil {
			return err
		}
		s.pnToLIDCache[pn] = lid
		s.lidToPNCache[lid] = pn
		fn(lid, pn)
	}
	err := rows.Close()
	if err != nil {
		return err
	}
	return rows.Err()
}

func (s *CachedLIDMap) getLIDMapping(ctx context.Context, source types.JID, targetServer, query string, sourceToTarget, targetToSource map[string]string) (types.JID, error) {
	s.lidCacheLock.RLock()
	targetUser, ok := sourceToTarget[source.User]
	cacheFilled := s.cacheFilled
	s.lidCacheLock.RUnlock()
	if ok || cacheFilled {
		if targetUser == "" {
			return types.JID{}, nil
		}
		return types.JID{User: targetUser, Device: source.Device, Server: targetServer}, nil
	}
	s.lidCacheLock.Lock()
	defer s.lidCacheLock.Unlock()
	err := s.db.QueryRow(ctx, query, source.User).Scan(&targetUser)
	if errors.Is(err, sql.ErrNoRows) {
		// continue with empty result
	} else if err != nil {
		return types.JID{}, err
	}
	sourceToTarget[source.User] = targetUser
	if targetUser != "" {
		targetToSource[targetUser] = source.User
		return types.JID{User: targetUser, Device: source.Device, Server: targetServer}, nil
	}
	return types.JID{}, nil
}

func (s *CachedLIDMap) GetLIDForPN(ctx context.Context, pn types.JID) (types.JID, error) {
	if pn.Server != types.DefaultUserServer {
		return types.JID{}, fmt.Errorf("invalid GetLIDForPN call with non-PN JID %s", pn)
	}
	return s.getLIDMapping(
		ctx, pn, types.HiddenUserServer, getLIDForPNQuery,
		s.pnToLIDCache, s.lidToPNCache,
	)
}

func (s *CachedLIDMap) GetPNForLID(ctx context.Context, lid types.JID) (types.JID, error) {
	if lid.Server != types.HiddenUserServer {
		return types.JID{}, fmt.Errorf("invalid GetPNForLID call with non-LID JID %s", lid)
	}
	return s.getLIDMapping(
		ctx, lid, types.DefaultUserServer, getPNForLIDQuery,
		s.lidToPNCache, s.pnToLIDCache,
	)
}

func (s *CachedLIDMap) GetManyLIDsForPNs(ctx context.Context, pns []types.JID) (map[types.JID]types.JID, error) {
	if len(pns) == 0 {
		return nil, nil
	}

	result := make(map[types.JID]types.JID, len(pns))

	s.lidCacheLock.RLock()
	missingPNs := make([]string, 0, len(pns))
	missingPNDevices := make(map[string][]types.JID)
	for _, pn := range pns {
		if pn.Server != types.DefaultUserServer {
			continue
		}
		if lidUser, ok := s.pnToLIDCache[pn.User]; ok && lidUser != "" {
			result[pn] = types.JID{User: lidUser, Device: pn.Device, Server: types.HiddenUserServer}
		} else if !s.cacheFilled {
			missingPNs = append(missingPNs, pn.User)
			missingPNDevices[pn.User] = append(missingPNDevices[pn.User], pn)
		}
	}
	s.lidCacheLock.RUnlock()

	if len(missingPNs) == 0 {
		return result, nil
	}

	s.lidCacheLock.Lock()
	defer s.lidCacheLock.Unlock()

	var rows dbutil.Rows
	var err error
	if s.db.Dialect == dbutil.Postgres && PostgresArrayWrapper != nil {
		rows, err = s.db.Query(
			ctx,
			`SELECT lid, pn FROM whatsmeow_lid_map WHERE pn = ANY($1)`,
			PostgresArrayWrapper(missingPNs),
		)
	} else {
		placeholders := make([]string, len(missingPNs))
		for i := range missingPNs {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
		}
		rows, err = s.db.Query(
			ctx,
			fmt.Sprintf(`SELECT lid, pn FROM whatsmeow_lid_map WHERE pn IN (%s)`, strings.Join(placeholders, ",")),
			exslices.CastToAny(missingPNs)...,
		)
	}
	if err != nil {
		return nil, err
	}
	err = s.scanManyLids(rows, func(lid, pn string) {
		for _, dev := range missingPNDevices[pn] {
			lidDev := dev
			lidDev.Server = types.HiddenUserServer
			lidDev.User = lid
			result[dev] = lidDev.ToNonAD()
		}
	})
	return result, err
}

func (s *CachedLIDMap) PutLIDMapping(ctx context.Context, lid, pn types.JID) error {
	if lid.Server != types.HiddenUserServer || pn.Server != types.DefaultUserServer {
		return fmt.Errorf("invalid PutLIDMapping call %s/%s", lid, pn)
	}
	s.lidCacheLock.Lock()
	defer s.lidCacheLock.Unlock()
	cachedLID, ok := s.pnToLIDCache[pn.User]
	if ok && cachedLID == lid.User {
		return nil
	}
	return s.db.DoTxn(ctx, nil, func(ctx context.Context) error {
		return s.unlockedPutLIDMapping(ctx, lid, pn)
	})
}

func (s *CachedLIDMap) PutManyLIDMappings(ctx context.Context, mappings []store.LIDMapping) error {
	s.lidCacheLock.Lock()
	defer s.lidCacheLock.Unlock()
	mappings = slices.DeleteFunc(mappings, func(mapping store.LIDMapping) bool {
		if mapping.LID.Server != types.HiddenUserServer || mapping.PN.Server != types.DefaultUserServer {
			zerolog.Ctx(ctx).Debug().
				Stringer("entry_lid", mapping.LID).
				Stringer("entry_pn", mapping.PN).
				Msg("Ignoring invalid entry in PutManyLIDMappings")
			return true
		}
		cachedLID, ok := s.pnToLIDCache[mapping.PN.User]
		if ok && cachedLID == mapping.LID.User {
			return true
		}
		return false
	})
	mappings = exslices.DeduplicateUnsortedOverwrite(mappings)
	if len(mappings) == 0 {
		return nil
	}
	return s.db.DoTxn(ctx, nil, func(ctx context.Context) error {
		for _, mapping := range mappings {
			err := s.unlockedPutLIDMapping(ctx, mapping.LID, mapping.PN)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *CachedLIDMap) unlockedPutLIDMapping(ctx context.Context, lid, pn types.JID) error {
	if lid.Server != types.HiddenUserServer || pn.Server != types.DefaultUserServer {
		return fmt.Errorf("invalid PutLIDMapping call %s/%s", lid, pn)
	}
	_, err := s.db.Exec(ctx, deleteExistingLIDMappingQuery, lid.User, pn.User)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(ctx, putLIDMappingQuery, lid.User, pn.User)
	if err != nil {
		return err
	}
	s.pnToLIDCache[pn.User] = lid.User
	s.lidToPNCache[lid.User] = pn.User
	return nil
}
