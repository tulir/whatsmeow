// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"fmt"

	"google.golang.org/protobuf/proto"

	waBinary "go.mau.fi/whatsmeow/binary"
	waProto "go.mau.fi/whatsmeow/binary/proto"
)

type patchList struct {
	Name           string
	HasMorePatches bool
	Patches        []*waProto.SyncdPatch
}

type AppStateMutation struct {
	Action    *waProto.SyncActionValue
	Index     []string
	IndexMAC  []byte
	ValueMAC  []byte
	Operation int
}

type HashState struct {
	Version   int
	Hash      []byte
	Mutations []AppStateMutation
}

func (cli *Client) FetchAppState(name string, fromVersion int) (*patchList, error) {
	resp, err := cli.sendIQ(infoQuery{
		Namespace: "w:sync:app:state",
		Type:      "set",
		To:        waBinary.ServerJID,
		Content: []waBinary.Node{{
			Tag: "sync",
			Content: []waBinary.Node{{
				Tag: "collection",
				Attrs: waBinary.Attrs{
					"name":            name,
					"version":         fromVersion,
					"return_snapshot": false,
				},
			}},
		}},
	})
	if err != nil {
		return nil, err
	}
	collection := resp.GetChildByTag("sync", "collection")
	ag := collection.AttrGetter()
	patchesNode := collection.GetChildByTag("patches")
	patchNodes := patchesNode.GetChildren()
	patches := make([]*waProto.SyncdPatch, 0, len(patchNodes))
	for _, patchNode := range patchNodes {
		rawPatch, ok := patchNode.Content.([]byte)
		if patchNode.Tag != "patch" || !ok {
			continue
		}
		var patch waProto.SyncdPatch
		err = proto.Unmarshal(rawPatch, &patch)
		if err != nil {
			cli.Log.Warnf("Failed to unmarshal app state patch: %v", err)
			continue
		}
		patches = append(patches, &patch)
		fmt.Printf("%+v\n", &patch)
	}
	return &patchList{
		Name:           ag.String("name"),
		HasMorePatches: ag.OptionalBool("has_more_patches"),
		Patches:        patches,
	}, nil
}

//func (cli *Client) decodeSyncdPatches(patches []*waProto.SyncdPatch)
