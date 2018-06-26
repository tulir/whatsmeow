package whatsapp

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"github.com/Rhymen/go-whatsapp/crypto/cbc"
	"github.com/Rhymen/go-whatsapp/crypto/hkdf"
	"github.com/Rhymen/go-whatsapp/whatsapp/binary"
	"io/ioutil"
	"net/http"
)

func (m *ImageMessage) Download() ([]byte, error) {
	fmt.Printf("A:%v\nD:%v\n", m.fileEncSha256, m.fileSha256)
	return download(m.url, m.mediaKey, binary.IMAGE, m.fileLength)
}
func (m *VideoMessage) Download() ([]byte, error) {
	return download(m.url, m.mediaKey, binary.VIDEO, m.fileLength)
}

func download(url string, mediaKey []byte, appInfo binary.AppInfo, fileLength int) ([]byte, error) {
	if url == "" {
		return nil, fmt.Errorf("no url present")
	}
	file, mac, err := downloadMedia(url)
	if err != nil {
		return nil, err
	}
	iv, cipherKey, macKey, _, err := getMediaKeys(mediaKey, appInfo)
	if err != nil {
		return nil, err
	}
	if err = validateMedia(iv, file, macKey, mac); err != nil {
		return nil, err
	}
	data, err := cbc.Decrypt(cipherKey, iv, file)
	if err != nil {
		return nil, err
	}
	if len(data) != fileLength {
		return nil, fmt.Errorf("file length does not match")
	}
	return data, nil
}

func validateMedia(iv []byte, file []byte, macKey []byte, mac []byte) error {
	h := hmac.New(sha256.New, macKey)
	n, err := h.Write(append(iv, file...))
	if err != nil {
		return err
	}
	if n < 10 {
		return fmt.Errorf("hash to short")
	}
	if !hmac.Equal(h.Sum(nil)[:10], mac) {
		return fmt.Errorf("invalid media hmac")
	}
	return nil
}

func getMediaKeys(mediaKey []byte, appInfo binary.AppInfo) (iv, cipherKey, macKey, refKey []byte, err error) {
	mediaKeyExpanded, err := hkdf.Expand(mediaKey, 112, appInfo)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return mediaKeyExpanded[:16], mediaKeyExpanded[16:48], mediaKeyExpanded[48:80], mediaKeyExpanded[80:], nil
}

func downloadMedia(url string) (file []byte, mac []byte, err error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode != 200 {
		return nil, nil, fmt.Errorf("download failed")
	}
	defer resp.Body.Close()
	if resp.ContentLength <= 10 {
		return nil, nil, fmt.Errorf("file to short")
	}
	data, err := ioutil.ReadAll(resp.Body)
	n := len(data)
	if err != nil {
		return nil, nil, err
	}
	return data[:n-10], data[n-10 : n], nil
}
