package main

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/RadicalApp/libsignal-protocol-go/ecc"
	"google.golang.org/protobuf/proto"
	log "maunium.net/go/maulogger/v2"

	waBinary "github.com/Rhymen/go-whatsapp/binary"
	waProto "github.com/Rhymen/go-whatsapp/binary/proto"
	"github.com/Rhymen/go-whatsapp/crypto/curve25519"
	"github.com/Rhymen/go-whatsapp/multidevice/socket"
)

func sliceToArray(data []byte) (out [32]byte) {
	copy(out[:], data[:32])
	return
}

func main() {
	log.DefaultLogger.PrintLevel = 0

	cli := Client{
		Log: log.DefaultLogger,
	}
	err := cli.Connect()
	if err != nil {
		log.Fatalln("Failed to connect:", err)
		return
	}

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
}

type WADeviceType int

const (
	WADeviceTypeJID WADeviceType = 0
	WADeviceTypeAD  WADeviceType = 0
)

type WADeviceID struct {
	Type   WADeviceType
	User   uint64
	Device int
	Agent  int
	Server string
}

type Session struct {
	NoiseKey          *KeyPair
	SignedIdentityKey *KeyPair
	SignedPreKey      *SignedKeyPair
	RegistrationID    uint32
	AdvSecretKey      []byte
	ID                *WADeviceID
}

type KeyPair struct {
	Pub  *[32]byte
	Priv *[32]byte
}

func NewKeyPair() (*KeyPair, error) {
	var kp KeyPair
	var err error
	kp.Priv, kp.Pub, err = curve25519.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate curve25519 keypair: %w", err)
	}
	return &kp, nil
}

func (kp *KeyPair) CreateSignedPreKey(keyID int) (*SignedKeyPair, error) {
	if keyID <= 0 {
		return nil, fmt.Errorf("invalid prekey ID %d", keyID)
	}
	keyPair, err := NewKeyPair()
	if err != nil {
		return nil, err
	}
	pubKeyForSignature := make([]byte, 33)
	pubKeyForSignature[0] = 5
	copy(pubKeyForSignature[1:], (*keyPair.Pub)[:])

	signature := ecc.CalculateSignature(ecc.NewDjbECPrivateKey(*keyPair.Priv), pubKeyForSignature)
	return &SignedKeyPair{
		KeyPair:   *keyPair,
		KeyID:     keyID,
		Signature: signature[:],
	}, nil
}

type SignedKeyPair struct {
	KeyPair
	KeyID     int
	Signature []byte
}

type Client struct {
	Session Session
	Log     log.Logger
	socket  *socket.NoiseSocket
}

// waVersion is the WhatsApp web client version
var waVersion = []int{2, 2138, 10}

// waVersionHashEncoded is the base64-encoded md5 hash of a dot-separated waVersion
var waVersionHashEncoded string

func init() {
	waVersionParts := make([]string, len(waVersion))
	for i, part := range waVersion {
		waVersionParts[i] = strconv.Itoa(part)
	}
	waVersionString := strings.Join(waVersionParts, ".")
	waVersionHash := md5.Sum([]byte(waVersionString))
	waVersionHashEncoded = base64.StdEncoding.EncodeToString(waVersionHash[:])
}

var BaseClientPayload = &waProto.ClientPayload{
	UserAgent: &waProto.UserAgent{
		Platform: waProto.UserAgent_WEB.Enum(),
		AppVersion: &waProto.AppVersion{
			Primary:   proto.Uint32(uint32(waVersion[0])),
			Secondary: proto.Uint32(uint32(waVersion[1])),
			Tertiary:  proto.Uint32(uint32(waVersion[2])),
		},
		Mcc:                         proto.String("000"),
		Mnc:                         proto.String("000"),
		OsVersion:                   proto.String("0.1"),
		Manufacturer:                proto.String(""),
		Device:                      proto.String("Desktop"),
		OsBuildNumber:               proto.String("0.1"),
		LocaleLanguageIso6391:       proto.String("en"),
		LocaleCountryIso31661Alpha2: proto.String("en"),
	},
	WebInfo: &waProto.WebInfo{
		WebSubPlatform: waProto.WebInfo_WEB_BROWSER.Enum(),
	},
	ConnectType:   waProto.ClientPayload_WIFI_UNKNOWN.Enum(),
	ConnectReason: waProto.ClientPayload_USER_ACTIVATED.Enum(),
}

var CompanionProps = &waProto.CompanionProps{
	Os:              nil,
	Version:         nil,
	PlatformType:    nil,
	RequireFullSync: nil,
}

func (sess *Session) getRegistrationPayload() *waProto.ClientPayload {
	payload := proto.Clone(BaseClientPayload).(*waProto.ClientPayload)
	regID := make([]byte, 4)
	binary.BigEndian.PutUint32(regID, sess.RegistrationID)
	preKeyID := make([]byte, 4)
	binary.BigEndian.PutUint32(preKeyID, uint32(sess.SignedPreKey.KeyID))
	companionProps, _ := proto.Marshal(CompanionProps)
	payload.RegData = &waProto.CompanionRegData{
		ERegid:         regID,
		EKeytype:       []byte{ecc.DjbType},
		EIdent:         (*sess.NoiseKey.Pub)[:],
		ESkeyId:        preKeyID[1:],
		ESkeyVal:       (*sess.SignedPreKey.Pub)[:],
		ESkeySig:       sess.SignedPreKey.Signature,
		BuildHash:      []byte(waVersionHashEncoded),
		CompanionProps: companionProps,
	}
	payload.Passive = proto.Bool(false)
	return payload
}

func (sess *Session) getLoginPayload() *waProto.ClientPayload {
	payload := proto.Clone(BaseClientPayload).(*waProto.ClientPayload)
	payload.Username = proto.Uint64(sess.ID.User)
	payload.Device = proto.Uint32(uint32(sess.ID.Device))
	payload.Passive = proto.Bool(true)
	return payload
}

func (sess *Session) getClientPayload() *waProto.ClientPayload {
	if sess.ID != nil {
		return sess.getLoginPayload()
	} else {
		return sess.getRegistrationPayload()
	}
}

func (cli *Client) Connect() error {
	fs := socket.NewFrameSocket(cli.Log.Sub("Socket"), socket.WAConnHeader)
	if ephemeralKP, err := NewKeyPair(); err != nil {
		return fmt.Errorf("failed to generate ephemeral keypair: %w", err)
	} else if err = fs.Connect(); err != nil {
		fs.Close()
		return err
	} else if err = cli.doHandshake(fs, *ephemeralKP); err != nil {
		fs.Close()
		return fmt.Errorf("noise handshake failed: %w", err)
	}
	cli.socket.OnFrame = cli.handleFrame
	return nil
}

func (cli *Client) handleFrame(data []byte) {
	decompressed, err := waBinary.Unpack(data)
	if err != nil {
		cli.Log.Warnln("Failed to decompress frame:", err)
		fmt.Println(hex.EncodeToString(data))
		return
	}
	decoder := waBinary.NewDecoder(decompressed, true)
	node, err := decoder.ReadNode()
	if err != nil {
		cli.Log.Warnln("Failed to decode node in frame:", err)
		fmt.Println(hex.EncodeToString(decompressed))
		return
	}
	fmt.Println(node.XMLString())
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
	//fmt.Println("Server ephemeral:", base64.StdEncoding.EncodeToString(serverEphemeral))
	//fmt.Println("Server static:", base64.StdEncoding.EncodeToString(serverStaticCiphertext))
	//fmt.Println("Certificate:", base64.StdEncoding.EncodeToString(certificateCiphertext))

	nh.Authenticate(serverEphemeral)
	err = nh.MixIntoKey(curve25519.GenerateSharedSecret(*ephemeralKP.Priv, sliceToArray(serverEphemeral)))
	if err != nil {
		return fmt.Errorf("failed to mix server ephemeral key in: %w", err)
	}

	staticDecrypted, err := nh.Decrypt(serverStaticCiphertext)
	if err != nil {
		return fmt.Errorf("failed to decrypt server static ciphertext: %w", err)
	}
	err = nh.MixIntoKey(curve25519.GenerateSharedSecret(*ephemeralKP.Priv, sliceToArray(staticDecrypted)))
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
	err = nh.MixIntoKey(curve25519.GenerateSharedSecret(*cli.Session.NoiseKey.Priv, sliceToArray(serverEphemeral)))
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
		regID, err := rand.Int(rand.Reader, big.NewInt(2<<30-1))
		if err != nil {
			return fmt.Errorf("failed to generate registration ID: %w", err)
		}
		cli.Session.RegistrationID = uint32(regID.Int64())
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
