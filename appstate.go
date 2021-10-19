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

func (cli *Client) FetchAppState(name appstate.WAPatchName) {
	state := appstate.NewHashState()
	hasMore := true
	for hasMore {
		patches, err := cli.fetchSyncdPatches(name, state.Version)
		if err != nil {
			cli.Log.Errorf("Failed to fetch app state sync patches: %v", err)
			return
		}
		hasMore = patches.HasMorePatches

		mutations, newState, err := cli.appStateProc.DecodePatches(patches, state, true)
		if err != nil {
			cli.Log.Errorf("Failed to decode patches: %v", err)
			break
		}
		state = newState
		fmt.Printf("%d %X\n", newState.Version, newState.Hash)
		for _, mutation := range mutations {
			fmt.Printf("%s %v %X %+v\n", mutation.Operation, mutation.Index, mutation.IndexMAC, mutation.Action)
		}
	}
}

func (cli *Client) fetchSyncdPatches(name appstate.WAPatchName, fromVersion uint64) (*appstate.PatchList, error) {
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
