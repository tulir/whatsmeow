// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"fmt"

	"go.mau.fi/whatsmeow/appstate"
	waBinary "go.mau.fi/whatsmeow/binary"
)

func (cli *Client) FetchAppState(name appstate.WAPatchName, fullSync bool) error {
	if fullSync {
		err := cli.Store.AppState.DeleteAppStateVersion(string(name))
		if err != nil {
			return fmt.Errorf("failed to reset app state %s version: %w", name, err)
		}
	}
	version, hash, err := cli.Store.AppState.GetAppStateVersion(string(name))
	if err != nil {
		return fmt.Errorf("failed to get app state %s version: %w", name, err)
	}
	state := appstate.HashState{Version: version, Hash: hash}
	hasMore := true
	for hasMore {
		patches, err := cli.fetchAppStatePatches(name, state.Version)
		if err != nil {
			return fmt.Errorf("failed to fetch app state %s patches: %w", name, err)
		}
		hasMore = patches.HasMorePatches

		mutations, newState, err := cli.appStateProc.DecodePatches(patches, state, true)
		if err != nil {
			return fmt.Errorf("failed to decode app state %s patches: %w", name, err)
		}
		state = newState
		fmt.Printf("%d %X\n", newState.Version, newState.Hash)
		for _, mutation := range mutations {
			fmt.Printf("%s %v %X %+v\n", mutation.Operation, mutation.Index, mutation.IndexMAC, mutation.Action)
		}
	}
	return nil
}

func (cli *Client) fetchAppStatePatches(name appstate.WAPatchName, fromVersion uint64) (*appstate.PatchList, error) {
	resp, err := cli.sendIQ(infoQuery{
		Namespace: "w:sync:app:state",
		Type:      "set",
		To:        waBinary.ServerJID,
		Content: []waBinary.Node{{
			Tag: "sync",
			Content: []waBinary.Node{{
				Tag: "collection",
				Attrs: waBinary.Attrs{
					"name":            string(name),
					"version":         fromVersion,
					"return_snapshot": false,
				},
			}},
		}},
	})
	if err != nil {
		return nil, err
	}
	return appstate.ParsePatchList(resp)
}
