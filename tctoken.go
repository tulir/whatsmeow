// Copyright (c) 2025 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"context"
	"fmt"
	"time"

	waBinary "go.mau.fi/whatsmeow/binary"
	"go.mau.fi/whatsmeow/types"
)

const (
	// tcTokenBucketDuration is the duration of a single bucket in seconds (7 days).
	// Matches WA Web AB prop tctoken_duration.
	tcTokenBucketDuration = 604800
	// tcTokenNumBuckets is the number of rolling buckets (4 = ~28-day window).
	// Matches WA Web AB prop tctoken_num_buckets.
	tcTokenNumBuckets      = 4
	tcTokenDBPruneInterval = 24 * time.Hour
)

func tcTokenCutoffTimestamp(now int64) int64 {
	currentBucket := now / tcTokenBucketDuration
	cutoffBucket := currentBucket - (tcTokenNumBuckets - 1)
	return cutoffBucket * tcTokenBucketDuration
}

func isTcTokenExpired(timestamp time.Time) bool {
	if timestamp.IsZero() {
		return true
	}
	return timestamp.Unix() < tcTokenCutoffTimestamp(time.Now().Unix())
}

func (cli *Client) resolveTcTokenStorageLID(ctx context.Context, jid types.JID) types.JID {
	storageJID := jid.ToNonAD()
	if storageJID.Server != types.DefaultUserServer || cli.Store == nil || cli.Store.LIDs == nil {
		return storageJID
	}
	lid, err := cli.Store.LIDs.GetLIDForPN(ctx, storageJID)
	if err != nil {
		cli.Log.Debugf("Failed to resolve LID for tctoken JID %s: %v", storageJID, err)
		return storageJID
	}
	if lid.IsEmpty() {
		return storageJID
	}
	return lid.ToNonAD()
}

func (cli *Client) hasValidTcTokenSenderTs(jid types.JID, storedSenderTimestamp time.Time) bool {
	key := jid.ToNonAD()
	if _, ok := cli.tcTokenSenderTs.Load(key); ok {
		return true
	}
	now := time.Now()
	if storedSenderTimestamp.IsZero() || storedSenderTimestamp.Unix() < tcTokenCutoffTimestamp(now.Unix()) {
		return false
	}
	// Hydrate from DB — this is a write, so sweep here
	cli.tcTokenSenderTs.Store(key, storedSenderTimestamp)
	cli.cleanupTcTokenSenderTsMapIfDue(now)
	return true
}

// shouldSendNewTcToken returns true when the current bucket is newer than the last issuance bucket.
func shouldSendNewTcToken(senderTimestamp time.Time) bool {
	if senderTimestamp.IsZero() {
		return true
	}
	now := time.Now().Unix()
	return now/tcTokenBucketDuration > senderTimestamp.Unix()/tcTokenBucketDuration
}

func shouldSendTcTokenInChatAction(jid types.JID) bool {
	jid = jid.ToNonAD()
	return (jid.Server == types.DefaultUserServer || jid.Server == types.HiddenUserServer) &&
		jid.User != types.PSAJID.User &&
		!jid.IsBot()
}

// getTcTokenSenderTs reads the in-memory sender timestamp for a JID.
func (cli *Client) getTcTokenSenderTs(jid types.JID) time.Time {
	if v, ok := cli.tcTokenSenderTs.Load(jid.ToNonAD()); ok {
		return v.(time.Time)
	}
	return time.Time{}
}

// setTcTokenSenderTs writes the in-memory sender timestamp for a JID.
func (cli *Client) setTcTokenSenderTs(jid types.JID, ts time.Time) {
	cli.tcTokenSenderTs.Store(jid.ToNonAD(), ts)
	cli.cleanupTcTokenSenderTsMapIfDue(time.Now())
}

func (cli *Client) cleanupTcTokenSenderTsMapIfDue(now time.Time) {
	if !cli.tcTokenSenderTsCleanupStarted.CompareAndSwap(false, true) {
		return
	}
	defer cli.tcTokenSenderTsCleanupStarted.Store(false)
	if now.Sub(cli.lastTcTokenSenderTsCleanup) < tcTokenBucketDuration*time.Second {
		return
	}
	cli.lastTcTokenSenderTsCleanup = now
	nowUnix := now.Unix()
	cutoffTimestamp := tcTokenCutoffTimestamp(nowUnix)
	cli.tcTokenSenderTs.Range(func(key, value any) bool {
		ts, ok := value.(time.Time)
		if !ok || ts.Unix() < cutoffTimestamp {
			cli.tcTokenSenderTs.Delete(key)
		}
		return true
	})
}

func (cli *Client) cleanupExpiredTcTokensFromDBIfDue(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	if !cli.tcTokenDBPruneStarted.CompareAndSwap(false, true) {
		return
	}
	now := time.Now()
	if now.Sub(cli.lastTcTokenDBPrune) < tcTokenDBPruneInterval {
		cli.tcTokenDBPruneStarted.Store(false)
		return
	}
	cli.lastTcTokenDBPrune = now
	go func() {
		defer cli.tcTokenDBPruneStarted.Store(false)
		cutoff := time.Unix(tcTokenCutoffTimestamp(time.Now().Unix()), 0)
		deleted, err := cli.Store.PrivacyTokens.DeleteExpiredPrivacyTokens(context.WithoutCancel(ctx), cutoff)
		if err != nil {
			cli.Log.Warnf("Failed to clean expired tctokens from DB: %v", err)
		} else if deleted > 0 {
			cli.Log.Debugf("Cleaned %d expired tctokens from DB", deleted)
		}
	}()
}

// issuePrivacyToken sends an IQ to the server to issue a privacy token for the given JID.
// Matches WAWebSetPrivacyTokensJob.issuePrivacyToken(targetJID, tokenTypes, timestamp).
func (cli *Client) issuePrivacyToken(ctx context.Context, jid types.JID, timestamp int64) (*waBinary.Node, error) {
	return cli.sendIQ(ctx, infoQuery{
		Namespace: "privacy",
		Type:      iqSet,
		To:        types.ServerJID,
		Content: []waBinary.Node{{
			Tag: "tokens",
			Content: []waBinary.Node{{
				Tag: "token",
				Attrs: waBinary.Attrs{
					"jid":  jid.ToNonAD(),
					"t":    fmt.Sprintf("%d", timestamp),
					"type": "trusted_contact",
				},
			}},
		}},
	})
}

// ensureTcToken returns a stored non-expired tctoken for the given JID, if available.
func (cli *Client) ensureTcToken(ctx context.Context, jid types.JID) (token []byte, err error) {
	cli.cleanupExpiredTcTokensFromDBIfDue(ctx)
	storageJID := cli.resolveTcTokenStorageLID(ctx, jid)
	existing, err := cli.Store.PrivacyTokens.GetPrivacyToken(ctx, storageJID)
	if err != nil {
		return nil, fmt.Errorf("failed to get privacy token: %w", err)
	}
	if existing == nil {
		return nil, nil
	}
	cli.hasValidTcTokenSenderTs(storageJID, existing.SenderTimestamp)
	if len(existing.Token) > 0 && !isTcTokenExpired(existing.Timestamp) {
		return existing.Token, nil
	}
	return nil, nil
}

// Only called when a bucket boundary has been crossed since the last issuance.
func (cli *Client) fireAndForgetTcTokenIssuance(ctx context.Context, jid types.JID, issueTimestamp int64) {
	go func(ctx context.Context) {
		storageJID := jid.ToNonAD()
		_, err := cli.issuePrivacyToken(ctx, storageJID, issueTimestamp)
		if err != nil {
			cli.Log.Debugf("Fire-and-forget tctoken issuance failed for %s: %v", jid, err)
			return
		}
		senderTimestamp := time.Unix(issueTimestamp, 0)
		cli.setTcTokenSenderTs(storageJID, senderTimestamp)
		existing, err := cli.Store.PrivacyTokens.GetPrivacyToken(ctx, storageJID)
		if err != nil {
			cli.Log.Debugf("Failed to load tctoken while persisting sender timestamp for %s: %v", jid, err)
			return
		}
		if existing == nil || len(existing.Token) == 0 {
			return
		}
		existing.SenderTimestamp = senderTimestamp
		if err = cli.Store.PrivacyTokens.PutPrivacyTokens(ctx, *existing); err != nil {
			cli.Log.Debugf("Failed to persist fire-and-forget sender timestamp for %s: %v", jid, err)
		}
	}(context.WithoutCancel(ctx))
}
