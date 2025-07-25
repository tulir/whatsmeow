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
	"github.com/jackc/pgx/v5/pgxpool"
	"slices"
	"sync"

	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
)

type CachedLIDMap struct {
	businessId string
	dbPool     *pgxpool.Pool

	pnToLIDCache map[string]string
	lidToPNCache map[string]string
	cacheFilled  bool
	lidCacheLock sync.RWMutex
}

var _ store.LIDStore = (*CachedLIDMap)(nil)

func NewCachedLIDMap(dbPool *pgxpool.Pool, businessId string) *CachedLIDMap {
	return &CachedLIDMap{
		businessId:   businessId,
		dbPool:       dbPool,
		pnToLIDCache: make(map[string]string),
		lidToPNCache: make(map[string]string),
	}
}

const (
	deleteExistingLIDMappingQuery = `DELETE FROM whatsmeow_lid_map WHERE business_id = $1 AND (lid<>$2 AND pn=$3)`
	putLIDMappingQuery            = `
		INSERT INTO whatsmeow_lid_map (business_id, lid, pn)
		VALUES ($1, $2, $3)
		ON CONFLICT (business_id, lid) DO UPDATE SET pn=excluded.pn WHERE whatsmeow_lid_map.pn<>excluded.pn
	`
	getLIDForPNQuery       = `SELECT lid FROM whatsmeow_lid_map WHERE business_id=$1 AND pn=$2`
	getPNForLIDQuery       = `SELECT pn FROM whatsmeow_lid_map WHERE business_id=$1 AND lid=$2`
	getAllLIDMappingsQuery = `SELECT lid, pn FROM whatsmeow_lid_map WHERE business_id=$1`
)

func (s *CachedLIDMap) FillCache() error {
	s.lidCacheLock.Lock()
	defer s.lidCacheLock.Unlock()
	rows, err := s.dbPool.Query(context.Background(), getAllLIDMappingsQuery, s.businessId)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var lid, pn string
		err = rows.Scan(&lid, &pn)
		if err != nil {
			return err
		}
		s.pnToLIDCache[pn] = lid
		s.lidToPNCache[lid] = pn
	}
	s.cacheFilled = true
	return nil
}

func (s *CachedLIDMap) getLIDMapping(source types.JID, targetServer string, query string, cacheKey string,
	sourceToTarget, targetToSource map[string]string) (types.JID, error) {

	// cacheKey is e.g. s.businessId
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
	// Double check after getting exclusive lock.
	targetUser, ok = sourceToTarget[source.User]
	if ok {
		if targetUser == "" {
			return types.JID{}, nil
		}
		return types.JID{User: targetUser, Device: source.Device, Server: targetServer}, nil
	}

	var queryVal string
	row := s.dbPool.QueryRow(context.Background(), query, s.businessId, source.User)
	err := row.Scan(&queryVal)
	if errors.Is(err, sql.ErrNoRows) {
		// cache miss, empty result
		queryVal = ""
	} else if err != nil {
		return types.JID{}, err
	}
	sourceToTarget[source.User] = queryVal
	if queryVal != "" {
		targetToSource[queryVal] = source.User
		return types.JID{User: queryVal, Device: source.Device, Server: targetServer}, nil
	}
	return types.JID{}, nil
}

func (s *CachedLIDMap) GetLIDForPN(pn types.JID) (types.JID, error) {
	if pn.Server != types.DefaultUserServer {
		return types.JID{}, fmt.Errorf("invalid GetLIDForPN call with non-PN JID %s", pn)
	}
	return s.getLIDMapping(
		pn, types.HiddenUserServer, getLIDForPNQuery, s.businessId,
		s.pnToLIDCache, s.lidToPNCache,
	)
}

func (s *CachedLIDMap) GetPNForLID(lid types.JID) (types.JID, error) {
	if lid.Server != types.HiddenUserServer {
		return types.JID{}, fmt.Errorf("invalid GetPNForLID call with non-LID JID %s", lid)
	}
	return s.getLIDMapping(
		lid, types.DefaultUserServer, getPNForLIDQuery, s.businessId,
		s.lidToPNCache, s.pnToLIDCache,
	)
}

func (s *CachedLIDMap) PutLIDMapping(lid, pn types.JID) error {
	if lid.Server != types.HiddenUserServer || pn.Server != types.DefaultUserServer {
		return fmt.Errorf("invalid PutLIDMapping call %s/%s", lid, pn)
	}
	s.lidCacheLock.Lock()
	defer s.lidCacheLock.Unlock()
	cachedLID, ok := s.pnToLIDCache[pn.User]
	if ok && cachedLID == lid.User {
		return nil
	}
	// Transaction is not strictly needed for a single row, but can be done for safety
	tx, err := s.dbPool.Begin(context.Background())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	_, err = tx.Exec(context.Background(), deleteExistingLIDMappingQuery, s.businessId, lid.User, pn.User)
	if err != nil {
		return err
	}
	_, err = tx.Exec(context.Background(), putLIDMappingQuery, s.businessId, lid.User, pn.User)
	if err != nil {
		return err
	}
	s.pnToLIDCache[pn.User] = lid.User
	s.lidToPNCache[lid.User] = pn.User
	return tx.Commit(context.Background())
}

func (s *CachedLIDMap) PutManyLIDMappings(mappings []store.LIDMapping) error {
	s.lidCacheLock.Lock()
	defer s.lidCacheLock.Unlock()
	mappings = slices.DeleteFunc(mappings, func(mapping store.LIDMapping) bool {
		if mapping.LID.Server != types.HiddenUserServer || mapping.PN.Server != types.DefaultUserServer {
			return true
		}
		cachedLID, ok := s.pnToLIDCache[mapping.PN.User]
		if ok && cachedLID == mapping.LID.User {
			return true
		}
		return false
	})
	if len(mappings) == 0 {
		return nil
	}
	tx, err := s.dbPool.Begin(context.Background())
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	for _, mapping := range mappings {
		_, err := tx.Exec(context.Background(), deleteExistingLIDMappingQuery, s.businessId, mapping.LID.User, mapping.PN.User)
		if err != nil {
			return err
		}
		_, err = tx.Exec(context.Background(), putLIDMappingQuery, s.businessId, mapping.LID.User, mapping.PN.User)
		if err != nil {
			return err
		}
		s.pnToLIDCache[mapping.PN.User] = mapping.LID.User
		s.lidToPNCache[mapping.LID.User] = mapping.PN.User
	}
	return tx.Commit(context.Background())
}
