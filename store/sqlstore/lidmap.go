package sqlstore

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"go.mau.fi/util/exslices"

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
	err = s.scanManyLids(rows, nil)
	s.cacheFilled = err == nil
	return err
}

func (s *CachedLIDMap) scanManyLids(rows pgx.Rows, fn func(lid, pn string)) error {
	if fn == nil {
		fn = func(lid, pn string) {}
	}
	for rows.Next() {
		var lid, pn string
		if err := rows.Scan(&lid, &pn); err != nil {
			rows.Close()
			return err
		}
		s.pnToLIDCache[pn] = lid
		s.lidToPNCache[lid] = pn
		fn(lid, pn)
	}
	rows.Close()
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

	// Check cache again under write lock
	targetUser, ok = sourceToTarget[source.User]
	if ok {
		if targetUser == "" {
			return types.JID{}, nil
		}
		return types.JID{User: targetUser, Device: source.Device, Server: targetServer}, nil
	}

	rows, err := s.dbPool.Query(ctx, query, s.businessId, source.User)
	if err != nil {
		return types.JID{}, fmt.Errorf("error querying LID mapping: %w", err)
	}
	defer rows.Close()

	if rows.Next() {
		if err = rows.Scan(&targetUser); err != nil {
			return types.JID{}, fmt.Errorf("error scanning LID mapping: %w", err)
		}
	} else {
		return types.JID{}, nil
	}
	if err = rows.Err(); err != nil {
		return types.JID{}, fmt.Errorf("error iterating LID mapping results: %w", err)
	}

	sourceToTarget[source.User] = targetUser
	if targetUser != "" {
		targetToSource[targetUser] = source.User
	}
	return types.JID{User: targetUser, Device: source.Device, Server: targetServer}, nil
}

func (s *CachedLIDMap) GetLIDForPN(ctx context.Context, pn types.JID) (types.JID, error) {
	if pn.IsEmpty() {
		return types.JID{}, fmt.Errorf("empty JID provided")
	}
	if pn.Server != types.DefaultUserServer {
		return types.JID{}, fmt.Errorf("invalid GetLIDForPN call with non-PN JID %s", pn)
	}
	lid, err := s.getLIDMapping(ctx, pn, types.HiddenUserServer, getLIDForPNQuery, s.pnToLIDCache, s.lidToPNCache)
	if err != nil {
		return types.JID{}, fmt.Errorf("failed to get LID for PN %s: %w", pn, err)
	}
	return lid, nil
}

func (s *CachedLIDMap) GetPNForLID(ctx context.Context, lid types.JID) (types.JID, error) {
	if lid.IsEmpty() {
		return types.JID{}, fmt.Errorf("empty LID provided")
	}
	if lid.Server != types.HiddenUserServer {
		return types.JID{}, fmt.Errorf("invalid GetPNForLID call with non-LID JID %s", lid)
	}
	pn, err := s.getLIDMapping(ctx, lid, types.DefaultUserServer, getPNForLIDQuery, s.lidToPNCache, s.pnToLIDCache)
	if err != nil {
		return types.JID{}, fmt.Errorf("failed to get PN for LID %s: %w", lid, err)
	}
	return pn, nil
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

	placeholders := make([]string, len(missingPNs))
	params := make([]interface{}, len(missingPNs)+1)
	params[0] = s.businessId
	for i, pn := range missingPNs {
		placeholders[i] = fmt.Sprintf("$%d", i+2)
		params[i+1] = pn
	}

	query := fmt.Sprintf(
		"SELECT lid, pn FROM whatsmeow_lid_map WHERE business_id = $1 AND pn IN (%s)",
		strings.Join(placeholders, ","),
	)

	rows, err := s.dbPool.Query(ctx, query, params...)
	if err != nil {
		return nil, fmt.Errorf("error querying LID mappings: %w", err)
	}
	defer rows.Close()

	err = s.scanManyLids(rows, func(lid, pn string) {
		if devices, ok := missingPNDevices[pn]; ok {
			for _, dev := range devices {
				lidDev := dev
				lidDev.Server = types.HiddenUserServer
				lidDev.User = lid
				result[dev] = lidDev.ToNonAD()
			}
		}
	})

	if err != nil {
		return nil, fmt.Errorf("error scanning LID mappings: %w", err)
	}

	return result, nil
}

func (s *CachedLIDMap) putLIDMappingTx(ctx context.Context, tx pgx.Tx, lid, pn types.JID) error {
	if lid.Server != types.HiddenUserServer || pn.Server != types.DefaultUserServer {
		return fmt.Errorf("invalid PutLIDMapping call %s/%s", lid, pn)
	}
	if _, err := tx.Exec(ctx, deleteExistingLIDMappingQuery, s.businessId, lid.User, pn.User); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, putLIDMappingQuery, s.businessId, lid.User, pn.User); err != nil {
		return err
	}
	s.pnToLIDCache[pn.User] = lid.User
	s.lidToPNCache[lid.User] = pn.User
	return nil
}

func (s *CachedLIDMap) PutLIDMapping(ctx context.Context, lid, pn types.JID) error {
	s.lidCacheLock.Lock()
	defer s.lidCacheLock.Unlock()

	cachedLID, ok := s.pnToLIDCache[pn.User]
	if ok && cachedLID == lid.User {
		return nil
	}
	tx, err := s.dbPool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := s.putLIDMappingTx(ctx, tx, lid, pn); err != nil {
		return err
	}
	return tx.Commit(ctx)
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
		return ok && cachedLID == mapping.LID.User
	})
	mappings = exslices.DeduplicateUnsortedOverwrite(mappings)
	if len(mappings) == 0 {
		return nil
	}

	tx, err := s.dbPool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, mapping := range mappings {
		if err := s.putLIDMappingTx(ctx, tx, mapping.LID, mapping.PN); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
