// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow

import (
	"bytes"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"

	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/socket"
	"go.mau.fi/whatsmeow/util/keys"
)

const NoiseHandshakeResponseTimeout = 20 * time.Second

// doHandshake implements the Noise_XX_25519_AESGCM_SHA256 handshake for the WhatsApp web API.
func (cli *Client) doHandshake(fs *socket.FrameSocket, ephemeralKP keys.KeyPair) error {
	nh := socket.NewNoiseHandshake()
	nh.Start(socket.NoiseStartPattern, fs.Header)
	nh.Authenticate(ephemeralKP.Pub[:])
	data, err := proto.Marshal(&waProto.HandshakeMessage{
		ClientHello: &waProto.HandshakeClientHello{
			Ephemeral: ephemeralKP.Pub[:],
		},
	})
	if err != nil {
		return fmt.Errorf("failed to marshal handshake message: %w", err)
	}
	err = fs.SendFrame(data)
	if err != nil {
		return fmt.Errorf("failed to send handshake message: %w", err)
	}
	var resp []byte
	select {
	case resp = <-fs.Frames:
	case <-time.After(NoiseHandshakeResponseTimeout):
		return fmt.Errorf("timed out waiting for handshake response")
	}
	var handshakeResponse waProto.HandshakeMessage
	err = proto.Unmarshal(resp, &handshakeResponse)
	if err != nil {
		return fmt.Errorf("failed to unmarshal handshake response: %w", err)
	}
	serverEphemeral := handshakeResponse.GetServerHello().GetEphemeral()
	serverStaticCiphertext := handshakeResponse.GetServerHello().GetStatic()
	certificateCiphertext := handshakeResponse.GetServerHello().GetPayload()
	if len(serverEphemeral) != 32 || serverStaticCiphertext == nil || certificateCiphertext == nil {
		return fmt.Errorf("missing parts of handshake response")
	}
	serverEphemeralArr := *(*[32]byte)(serverEphemeral)

	nh.Authenticate(serverEphemeral)
	err = nh.MixSharedSecretIntoKey(*ephemeralKP.Priv, serverEphemeralArr)
	if err != nil {
		return fmt.Errorf("failed to mix server ephemeral key in: %w", err)
	}

	staticDecrypted, err := nh.Decrypt(serverStaticCiphertext)
	if err != nil {
		return fmt.Errorf("failed to decrypt server static ciphertext: %w", err)
	} else if len(staticDecrypted) != 32 {
		return fmt.Errorf("unexpected length of server static plaintext %d (expected 32)", len(staticDecrypted))
	}
	err = nh.MixSharedSecretIntoKey(*ephemeralKP.Priv, *(*[32]byte)(staticDecrypted))
	if err != nil {
		return fmt.Errorf("failed to mix server static key in: %w", err)
	}

	certDecrypted, err := nh.Decrypt(certificateCiphertext)
	if err != nil {
		return fmt.Errorf("failed to decrypt noise certificate ciphertext: %w", err)
	}
	var cert waProto.NoiseCertificate
	err = proto.Unmarshal(certDecrypted, &cert)
	if err != nil {
		return fmt.Errorf("failed to unmarshal noise certificate: %w", err)
	}
	certDetailsRaw := cert.GetDetails()
	certSignature := cert.GetSignature()
	if certDetailsRaw == nil || certSignature == nil {
		return fmt.Errorf("missing parts of noise certificate")
	}
	var certDetails waProto.NoiseCertificate_Details
	err = proto.Unmarshal(certDetailsRaw, &certDetails)
	if err != nil {
		return fmt.Errorf("failed to unmarshal noise certificate details: %w", err)
	} else if !bytes.Equal(certDetails.GetKey(), staticDecrypted) {
		return fmt.Errorf("cert key doesn't match decrypted static")
	}

	encryptedPubkey := nh.Encrypt(cli.Store.NoiseKey.Pub[:])
	err = nh.MixSharedSecretIntoKey(*cli.Store.NoiseKey.Priv, serverEphemeralArr)
	if err != nil {
		return fmt.Errorf("failed to mix noise private key in: %w", err)
	}

	clientFinishPayloadBytes, err := proto.Marshal(cli.Store.GetClientPayload())
	if err != nil {
		return fmt.Errorf("failed to marshal client finish payload: %w", err)
	}
	encryptedClientFinishPayload := nh.Encrypt(clientFinishPayloadBytes)
	data, err = proto.Marshal(&waProto.HandshakeMessage{
		ClientFinish: &waProto.HandshakeClientFinish{
			Static:  encryptedPubkey,
			Payload: encryptedClientFinishPayload,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to marshal handshake finish message: %w", err)
	}
	err = fs.SendFrame(data)
	if err != nil {
		return fmt.Errorf("failed to send handshake finish message: %w", err)
	}

	ns, err := nh.Finish(fs, cli.handleFrame, cli.onDisconnect)
	if err != nil {
		return fmt.Errorf("failed to create noise socket: %w", err)
	}

	cli.socket = ns

	return nil
}
