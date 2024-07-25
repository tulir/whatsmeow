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

	"go.mau.fi/libsignal/ecc"
	"google.golang.org/protobuf/proto"

	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/socket"
	"go.mau.fi/whatsmeow/util/keys"
)

const NoiseHandshakeResponseTimeout = 20 * time.Second
const WACertIssuerSerial = 0

var WACertPubKey = [...]byte{0x14, 0x23, 0x75, 0x57, 0x4d, 0xa, 0x58, 0x71, 0x66, 0xaa, 0xe7, 0x1e, 0xbe, 0x51, 0x64, 0x37, 0xc4, 0xa2, 0x8b, 0x73, 0xe3, 0x69, 0x5c, 0x6c, 0xe1, 0xf7, 0xf9, 0x54, 0x5d, 0xa8, 0xee, 0x6b}

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
	} else if err = verifyServerCert(certDecrypted, staticDecrypted); err != nil {
		return fmt.Errorf("failed to verify server cert: %w", err)
	}

	encryptedPubkey := nh.Encrypt(cli.Store.NoiseKey.Pub[:])
	err = nh.MixSharedSecretIntoKey(*cli.Store.NoiseKey.Priv, serverEphemeralArr)
	if err != nil {
		return fmt.Errorf("failed to mix noise private key in: %w", err)
	}

	var clientPayload *waProto.ClientPayload
	if cli.GetClientPayload != nil {
		clientPayload = cli.GetClientPayload()
	} else {
		clientPayload = cli.Store.GetClientPayload()
	}

	clientFinishPayloadBytes, err := proto.Marshal(clientPayload)
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

func verifyServerCert(certDecrypted, staticDecrypted []byte) error {
	var certChain waProto.CertChain
	err := proto.Unmarshal(certDecrypted, &certChain)
	if err != nil {
		return fmt.Errorf("failed to unmarshal noise certificate: %w", err)
	}
	var intermediateCertDetails, leafCertDetails waProto.CertChain_NoiseCertificate_Details
	intermediateCertDetailsRaw := certChain.GetIntermediate().GetDetails()
	intermediateCertSignature := certChain.GetIntermediate().GetSignature()
	leafCertDetailsRaw := certChain.GetLeaf().GetDetails()
	leafCertSignature := certChain.GetLeaf().GetSignature()
	if intermediateCertDetailsRaw == nil || intermediateCertSignature == nil || leafCertDetailsRaw == nil || leafCertSignature == nil {
		return fmt.Errorf("missing parts of noise certificate")
	} else if len(intermediateCertSignature) != 64 {
		return fmt.Errorf("unexpected length of intermediate cert signature %d (expected 64)", len(intermediateCertSignature))
	} else if len(leafCertSignature) != 64 {
		return fmt.Errorf("unexpected length of leaf cert signature %d (expected 64)", len(leafCertSignature))
	} else if !ecc.VerifySignature(ecc.NewDjbECPublicKey(WACertPubKey), intermediateCertDetailsRaw, [64]byte(intermediateCertSignature)) {
		return fmt.Errorf("failed to verify intermediate cert signature")
	} else if err = proto.Unmarshal(intermediateCertDetailsRaw, &intermediateCertDetails); err != nil {
		return fmt.Errorf("failed to unmarshal noise certificate details: %w", err)
	} else if intermediateCertDetails.GetIssuerSerial() != WACertIssuerSerial {
		return fmt.Errorf("unexpected intermediate issuer serial %d (expected %d)", intermediateCertDetails.GetIssuerSerial(), WACertIssuerSerial)
	} else if len(intermediateCertDetails.GetKey()) != 32 {
		return fmt.Errorf("unexpected length of intermediate cert key %d (expected 32)", len(intermediateCertDetails.GetKey()))
	} else if !ecc.VerifySignature(ecc.NewDjbECPublicKey([32]byte(intermediateCertDetails.GetKey())), leafCertDetailsRaw, [64]byte(leafCertSignature)) {
		return fmt.Errorf("failed to verify intermediate cert signature")
	} else if err = proto.Unmarshal(leafCertDetailsRaw, &leafCertDetails); err != nil {
		return fmt.Errorf("failed to unmarshal noise certificate details: %w", err)
	} else if leafCertDetails.GetIssuerSerial() != intermediateCertDetails.GetSerial() {
		return fmt.Errorf("unexpected leaf issuer serial %d (expected %d)", leafCertDetails.GetIssuerSerial(), intermediateCertDetails.GetSerial())
	} else if !bytes.Equal(leafCertDetails.GetKey(), staticDecrypted) {
		return fmt.Errorf("cert key doesn't match decrypted static")
	}
	return nil
}
