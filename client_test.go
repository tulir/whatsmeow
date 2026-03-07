// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package whatsmeow_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image/png"
	"log"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/qr"
	"github.com/hajimehoshi/go-mp3"
	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/song-xiang13/whatsmeow"
	"github.com/song-xiang13/whatsmeow/proto/waE2E"
	"github.com/song-xiang13/whatsmeow/proto/waHistorySync"
	"github.com/song-xiang13/whatsmeow/store/sqlstore"
	"github.com/song-xiang13/whatsmeow/types"
	"github.com/song-xiang13/whatsmeow/types/events"
	waLog "github.com/song-xiang13/whatsmeow/util/log"
	"go.mau.fi/util/dbutil"
	"google.golang.org/protobuf/proto"
)

const ignoreDir = "./zzz/"

func printStr(str1 any) string {
	str := fmt.Sprintf("%+v", str1)
	if len(str) <= 2000 {
		return str
	}

	return string([]rune(str)[:2000])
}

func WriteFile(name string, msg any) {
	name = ignoreDir + name + ".json"
	b, _ := json.Marshal(msg)
	err := os.WriteFile(name, b, os.ModePerm)
	if err != nil {
		log.Println("Error writing file:", name, err)
	}
}

func mergeContactInfo(existing, newInfo types.ContactInfo) types.ContactInfo {
	if existing.FirstName == "" {
		existing.FirstName = newInfo.FirstName
	}
	if existing.FullName == "" {
		existing.FullName = newInfo.FullName
	}
	if existing.PushName == "" {
		existing.PushName = newInfo.PushName
	}
	if existing.BusinessName == "" {
		existing.BusinessName = newInfo.BusinessName
	}
	if existing.RedactedPhone == "" {
		existing.RedactedPhone = newInfo.RedactedPhone
	}
	return existing
}

func dedupeContacts(ctx context.Context, client *whatsmeow.Client, contacts map[types.JID]types.ContactInfo) map[types.JID]types.ContactInfo {
	out := make(map[types.JID]types.ContactInfo)
	selfPN := client.Store.GetJID().ToNonAD()
	selfLID := client.Store.GetLID().ToNonAD()

	for jid, info := range contacts {
		//fmt.Printf("dedupeContacts -- %#v\n", jid)
		base := jid.ToNonAD()
		if base == selfPN || (!selfLID.IsEmpty() && base == selfLID) {
			// skip self (both PN and LID)
			continue
		}

		canonical := base
		if canonical.Server == types.DefaultUserServer {
			if lid, err := client.Store.LIDs.GetLIDForPN(ctx, base); err == nil && lid.User != "" {
				canonical = lid.ToNonAD()
			}
		}

		log.Printf("dedupeContacts -- %s\n", canonical)
		if existing, ok := out[canonical]; ok {
			out[canonical] = mergeContactInfo(existing, info)
			//log.Printf("mergeContactInfo -- %+v %+v\n", canonical, info)
		} else {
			out[canonical] = info
		}
	}
	return out
}

func makeEventHandler() func(evt interface{}) {
	return func(evt interface{}) {
		log.Printf("eventHandler -- %T %s\n", evt, printStr(evt))
		switch v := evt.(type) {
		case *events.HistorySync:
			//v.Data.GetSyncType() == waHistorySync.HistorySync_PUSH_NAME
			WriteFile("HistorySync_"+v.Data.GetSyncType().String(), v.Data)
			switch v.Data.GetSyncType() {
			case waHistorySync.HistorySync_INITIAL_BOOTSTRAP: // 首条
				v.Data.GetPhoneNumberToLidMappings() // 就是会话列表数
			case waHistorySync.HistorySync_PUSH_NAME: // 推送会话列表的 PushName集合
				log.Printf("HistorySync_PUSH_NAME -- PushNameCount:%d\n", len(v.Data.GetPushnames()))
				for _, p := range v.Data.GetPushnames() {
					log.Printf("HistorySync_PUSH_NAME -- PushName: ID:%s PushName:%s", p.GetID(), p.GetPushname())
				}
			case waHistorySync.HistorySync_RECENT:
				log.Printf("HistorySync_RECENT -- GetConversations:%d\n", len(v.Data.GetConversations()))
				for _, p := range v.Data.GetConversations() {
					log.Printf("HistorySync_RECENT -- Conversation ID:%s msg.len:%d", p.GetID(), len(p.Messages))
				}
			case waHistorySync.HistorySync_ON_DEMAND:

			}
			//err := writeConversationsCSV("conversations.csv", v.Data.GetConversations())
			//if err != nil {
			//	log.Println("Error writing conversations.csv:", err)
			//}
		case *events.Archive:
		case *events.Message: // 自己发的消息
			b, _ := json.Marshal(v.Message)
			b2, _ := json.Marshal(v.Info)
			//v.Message.GetConversation()
			//v.Info.RecipientAlt  -- 别人接收时有 8618587904107@s.whatsapp.net
			//v.Info.SenderAlt -- 别人发送时有
			//v.Info.Timestamp
			//v.Info.PushName
			//v.Info.Type
			//v.Info.ID -- 消息id
			WriteFile("v.Message.json", b)
			WriteFile("v.Info.json", b2)
			log.Println("Received a message!", v.Message.GetConversation())
		case *events.PushName:
			log.Printf("Received a PushName: %+v -- Msg:%+v\n", v, v.Message)
		case *events.Receipt: // 消息回执，可能会发重复的好几条
		//v.MessageIDs
		case *events.AppState: // 很多场景，如
			//v.GetTimestamp() ms
			//v.AgentAction 设置关联设备名称 {name:"dddw"  deviceID:9  isDeleted:false}
		}
	}
}

func writeQRPNG(path, content string) error {
	code, err := qr.Encode(content, qr.M, qr.Auto)
	if err != nil {
		return err
	}
	scaled, err := barcode.Scale(code, 256, 256)
	if err != nil {
		return err
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return png.Encode(f, scaled)
}

func sendImgTextMsg(client *whatsmeow.Client, to string, img []byte, t string) error {
	resp, err := client.Upload(context.Background(), img, whatsmeow.MediaImage)
	if err != nil {
		return err
	}

	// 创建一个消息对象并发送
	msg := &waE2E.Message{
		ImageMessage: &waE2E.ImageMessage{
			URL:           &resp.URL,
			Mimetype:      &t,
			Caption:       dbutil.StrPtr("哈哈哈"),
			FileSHA256:    resp.FileSHA256,
			FileLength:    &resp.FileLength,
			MediaKey:      resp.MediaKey,
			FileEncSHA256: resp.FileEncSHA256,
			DirectPath:    &resp.DirectPath,
		},
	}

	chat := types.NewJID(to, "s.whatsapp.net")

	_, err = client.SendMessage(context.Background(), chat, msg)
	return err
}

func sendAudioMsg(client *whatsmeow.Client, to string, file []byte, t string) error {
	resp, err := client.Upload(context.Background(), file, whatsmeow.MediaAudio)
	if err != nil {
		return err
	}

	d, _ := mp3.NewDecoder(bytes.NewReader(file))
	sec := float64(d.Length()) / float64(d.SampleRate()*4)
	println(1111, int(sec))
	// 创建一个消息对象并发送
	msg := &waE2E.Message{
		AudioMessage: &waE2E.AudioMessage{
			URL:           &resp.URL,
			Mimetype:      &t,
			Seconds:       proto.Uint32(uint32(float64(d.Length()) / float64(d.SampleRate()*4))),
			FileSHA256:    resp.FileSHA256,
			FileLength:    &resp.FileLength,
			MediaKey:      resp.MediaKey,
			FileEncSHA256: resp.FileEncSHA256,
			DirectPath:    &resp.DirectPath,
			PTT:           proto.Bool(true),
			//Waveform:
		},
	}

	chat := types.NewJID(to, "s.whatsapp.net")

	_, err = client.SendMessage(context.Background(), chat, msg)
	return err
}

func TestExample(t *testing.T) {
	// |------------------------------------------------------------------------------------------------------|
	// | NOTE: You must also import the appropriate DB connector, e.g. github.com/mattn/go-sqlite3 for SQLite |
	// |------------------------------------------------------------------------------------------------------|

	dbLog := waLog.Stdout("Database", "DEBUG", true)
	ctx, _ := context.WithTimeout(context.Background(), 20*time.Second)
	container, err := sqlstore.New(ctx, "sqlite3", fmt.Sprintf("file:%sexamplestore.db?_foreign_keys=on", ignoreDir), dbLog)
	if err != nil {
		panic(err)
	}
	// If you want multiple sessions, remember their JIDs and use .GetDevice(jid) or .GetAllDevices() instead.
	deviceStore, err := container.GetFirstDevice(ctx)
	if err != nil {
		panic(err)
	}
	clientLog := waLog.Stdout("Client", "DEBUG", true)
	client := whatsmeow.NewClient(deviceStore, clientLog)
	client.CustomStoredMsgMaxNum = 11
	client.AddEventHandler(makeEventHandler())

	if client.Store.ID == nil {
		// No ID stored, new login
		qrChan, _ := client.GetQRChannel(context.Background())
		err = client.Connect()
		if err != nil {
			panic(err)
		}
		for evt := range qrChan {
			if evt.Event == "code" {
				// 生成 PNG 登录二维码，方便手机扫码
				const qrPath = "./zzz/login-qr.png"
				if err := writeQRPNG(qrPath, evt.Code); err != nil {
					log.Println("生成二维码失败:", err, "请手动扫描文本:", evt.Code)
				} else {
					log.Println("二维码已生成:", qrPath, "若无法打开，请手动扫描文本:", evt.Code)
				}
			} else {
				log.Println("Login event:", evt.Event)
			}
		}
	} else {
		// Already logged in, just connect
		err = client.Connect()
		if err != nil {
			panic(err)
		}

		rawList, err := client.Store.Contacts.GetAllContacts(context.Background(), true)
		if err != nil {
			panic(err)
		}
		contacts := dedupeContacts(context.Background(), client, rawList)
		//for jid, c := range contacts {
		//	log.Printf("Contact %+v -- Contact:%+v", jid, c)
		//}
		log.Printf("Contact Count:%d rawList:%d", len(contacts), len(rawList))
		//
		//log.Printf("GetLID -- %+v\n", client.Store.GetLID())
		// 获取头像
		//infomap, _ := client.GetUserInfo(ctx, []types.JID{client.Store.GetLID()})
		//for jid, v := range infomap {
		//	//v.Status 是用户文字状态，一般非空
		//	//v.PictureID 用户头像ID，为空表示无头像
		//	//v.Devices 账户登录的设备JIDs，含当前
		//	//v.VerifiedName
		//	log.Printf("GetUserInfo jid=%s -%s|%s- %+v\n", jid, jid, client.Store.GetLID(), v)
		//}

		//glist, _ := client.GetJoinedGroups(ctx)
		//for _, g := range glist {
		//	log.Printf("GetJoinedGroups -- %+v\n", g.Name)
		//	WriteFile("group_msg_"+g.Name, g)
		//}
		//// 隐私
		//s := client.GetPrivacySettings(ctx)
		//log.Printf("GetPrivacySettings -- %+v\n", s)
		//// 查询会话数量
		imap, err := client.Store.MsgSecrets.GetMessageSessionNumGroupByPeer(ctx)
		if err != nil {
			log.Println("GetMessageSessionNumGroupByPeer failed --", err)
		} else {
			for _, v := range imap {
				log.Printf("GetMessageSessionNumGroupByPeer OK -- %+v\n", v)
			}
		}
		//
		//clist, err := client.Store.ChatSettings.GetAllChatSettings(ctx)
		//if err != nil {
		//	log.Println("GetAllChatSettings --", err)
		//} else {
		//	for _, v := range clist {
		//		log.Printf("GetAllChatSettings -- %+v\n", v)
		//	}
		//}
		//
		//blist, err := client.GetBlocklist(ctx)
		//if err != nil {
		//	log.Println("GetBlocklist --", err)
		//} else {
		//	log.Printf("GetBlocklist -- %+v\n", blist)
		//}
		//
		//jid := client.Store.GetJID()
		////jid.User = "919314613946@s.whatsapp.net"
		//b, err := client.GetBusinessProfile(context.Background(), jid)
		//if err != nil {
		//	log.Println("GetBusinessProfile --", err)
		//} else {
		//	log.Printf("GetBusinessProfile -- %+v -- Jid.Empty=%t\n", b, b.JID.IsEmpty())
		//}

		// 发送图片+文字消息
		//img, err := os.ReadFile("./zzz/car.jpg")
		//if err != nil {
		//	panic(err)
		//}
		//err = sendImgTextMsg(client, "201202883785", img, "image/jpeg")
		//if err != nil {
		//	log.Fatalf("sendImgTextMsg error: %v", err)
		//}

		// 发送语音消息
		//audio, err := os.ReadFile("./zzz/sound.mp3")
		//if err != nil {
		//	panic(err)
		//}
		//
		//err = sendAudioMsg(client, "201202883785", audio, "audio/ogg; codecs=opus")
		//if err != nil {
		//	log.Fatalf("sendAudioMsg error: %v", err)
		//}
	}

	// Listen to Ctrl+C (you can also do something else that prevents the program from exiting)
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	client.Disconnect()
}
