// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package multidevice

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	mathRand "math/rand"

	"google.golang.org/protobuf/proto"

	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/crypto/curve25519"
	"go.mau.fi/whatsmeow/multidevice/socket"
)

func sliceToArray32(data []byte) (out [32]byte) {
	copy(out[:], data[:32])
	return
}

func (cli *Client) doHandshake(fs *socket.FrameSocket, ephemeralKP KeyPair) error {
	nh := socket.NewNoiseHandshake()
	nh.Start(socket.NoiseStartPattern, fs.Header)
	nh.Authenticate((*ephemeralKP.Pub)[:])
	data, err := proto.Marshal(&waProto.HandshakeMessage{
		ClientHello: &waProto.ClientHello{
			Ephemeral: (*ephemeralKP.Pub)[:],
		},
	})
	if err != nil {
		return fmt.Errorf("failed to marshal handshake message: %w", err)
	}
	resp, err := fs.SendAndReceiveFrame(context.Background(), data)
	if err != nil {
		return fmt.Errorf("failed to send handshake message: %w", err)
	}
	var handshakeResponse waProto.HandshakeMessage
	err = proto.Unmarshal(resp, &handshakeResponse)
	if err != nil {
		return fmt.Errorf("failed to unmarshal handshake response: %w", err)
	}
	serverEphemeral := handshakeResponse.GetServerHello().GetEphemeral()
	serverStaticCiphertext := handshakeResponse.GetServerHello().GetStatic()
	certificateCiphertext := handshakeResponse.GetServerHello().GetPayload()
	if serverEphemeral == nil || serverStaticCiphertext == nil || certificateCiphertext == nil {
		return fmt.Errorf("missing parts of handshake response")
	}

	nh.Authenticate(serverEphemeral)
	err = nh.MixIntoKey(curve25519.GenerateSharedSecret(*ephemeralKP.Priv, sliceToArray32(serverEphemeral)))
	if err != nil {
		return fmt.Errorf("failed to mix server ephemeral key in: %w", err)
	}

	staticDecrypted, err := nh.Decrypt(serverStaticCiphertext)
	if err != nil {
		return fmt.Errorf("failed to decrypt server static ciphertext: %w", err)
	}
	err = nh.MixIntoKey(curve25519.GenerateSharedSecret(*ephemeralKP.Priv, sliceToArray32(staticDecrypted)))
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
	var certDetails waProto.NoiseCertificateDetails
	err = proto.Unmarshal(certDetailsRaw, &certDetails)
	if err != nil {
		return fmt.Errorf("failed to unmarshal noise certificate details: %w", err)
	} else if !bytes.Equal(certDetails.GetKey(), staticDecrypted) {
		return fmt.Errorf("cert key doesn't match decrypted static")
	}

	if cli.Session.NoiseKey == nil {
		cli.Session.NoiseKey = &KeyPair{}
		cli.Session.NoiseKey.Priv, cli.Session.NoiseKey.Pub, err = curve25519.GenerateKey()
		if err != nil {
			return fmt.Errorf("failed to generate curve25519 keypair: %w", err)
		}
	}

	encryptedPubkey := nh.Encrypt((*cli.Session.NoiseKey.Pub)[:])
	err = nh.MixIntoKey(curve25519.GenerateSharedSecret(*cli.Session.NoiseKey.Priv, sliceToArray32(serverEphemeral)))
	if err != nil {
		return fmt.Errorf("failed to mix noise private key in: %w", err)
	}

	if cli.Session.SignedIdentityKey == nil {
		cli.Session.SignedIdentityKey = &KeyPair{}
		cli.Session.SignedIdentityKey.Priv, cli.Session.SignedIdentityKey.Pub, err = curve25519.GenerateKey()
		if err != nil {
			return fmt.Errorf("failed to generate curve25519 keypair: %w", err)
		}
	}
	if cli.Session.SignedPreKey == nil {
		cli.Session.SignedPreKey, err = cli.Session.SignedIdentityKey.CreateSignedPreKey(1)
		if err != nil {
			return fmt.Errorf("failed to generate signed prekey: %w", err)
		}
	}
	if cli.Session.RegistrationID == 0 {
		cli.Session.RegistrationID = mathRand.Uint32()
	}

	clientFinishPayloadBytes, err := proto.Marshal(cli.Session.getClientPayload())
	if err != nil {
		return fmt.Errorf("failed to marshal client finish payload: %w", err)
	}
	encryptedClientFinishPayload := nh.Encrypt(clientFinishPayloadBytes)
	data, err = proto.Marshal(&waProto.HandshakeMessage{
		ClientFinish: &waProto.ClientFinish{
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

	ns, err := nh.Finish(fs)
	if err != nil {
		return fmt.Errorf("failed to create noise socket: %w", err)
	}

	if cli.Session.AdvSecretKey == nil {
		cli.Session.AdvSecretKey = make([]byte, 32)
		_, err = rand.Read(cli.Session.AdvSecretKey)
		if err != nil {
			return fmt.Errorf("failed to generate adv secret key: %w", err)
		}
	}

	cli.socket = ns

	return nil
}
