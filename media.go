package whatsapp

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/Rhymen/go-whatsapp/crypto/cbc"
	"github.com/Rhymen/go-whatsapp/crypto/hkdf"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"
)

func Download(url string, mediaKey []byte, appInfo MediaType, fileLength int) ([]byte, error) {
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

func getMediaKeys(mediaKey []byte, appInfo MediaType) (iv, cipherKey, macKey, refKey []byte, err error) {
	mediaKeyExpanded, err := hkdf.Expand(mediaKey, 112, string(appInfo))
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

func (wac *Conn) Upload(reader io.Reader, appInfo MediaType) (url string, mediaKey []byte, fileEncSha256 []byte, fileSha256 []byte, fileLength uint64, err error) {
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return "", nil, nil, nil, 0, err
	}

	mediaKey = make([]byte, 32)
	rand.Read(mediaKey)

	iv, cipherKey, macKey, _, err := getMediaKeys(mediaKey, appInfo)
	if err != nil {
		return "", nil, nil, nil, 0, err
	}

	enc, err := cbc.Encrypt(cipherKey, iv, data)
	if err != nil {
		return "", nil, nil, nil, 0, err
	}

	fileLength = uint64(len(data))

	h := hmac.New(sha256.New, macKey)
	h.Write(append(iv, enc...))
	mac := h.Sum(nil)[:10]

	sha := sha256.New()
	sha.Write(data)
	fileSha256 = sha.Sum(nil)

	sha.Reset()
	sha.Write(append(enc, mac...))
	fileEncSha256 = sha.Sum(nil)

	var filetype string
	switch appInfo {
	case MediaImage:
		filetype = "image"
	case MediaAudio:
		filetype = "audio"
	case MediaDocument:
		filetype = "document"
	case MediaVideo:
		filetype = "video"
	}

	uploadReq := []interface{}{"action", "encr_upload", filetype, base64.StdEncoding.EncodeToString(fileEncSha256)}
	ch, err := wac.write(uploadReq)
	if err != nil {
		return "", nil, nil, nil, 0, err
	}

	var resp map[string]interface{}
	select {
	case r := <-ch:
		if err = json.Unmarshal([]byte(r), &resp); err != nil {
			return "", nil, nil, nil, 0, fmt.Errorf("error decoding upload response: %v\n", err)
		}
	case <-time.After(wac.msgTimeout):
		return "", nil, nil, nil, 0, fmt.Errorf("restore session init timed out")
	}

	if int(resp["status"].(float64)) != 200 {
		return "", nil, nil, nil, 0, fmt.Errorf("upload responsed with %d", resp["status"])
	}

	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	hashWriter, err := w.CreateFormField("hash")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
	}
	io.Copy(hashWriter, strings.NewReader(base64.StdEncoding.EncodeToString(fileEncSha256)))

	fileWriter, err := w.CreateFormFile("file", "blob")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
	}
	io.Copy(fileWriter, bytes.NewReader(append(enc, mac...)))
	err = w.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
	}

	req, err := http.NewRequest("POST", resp["url"].(string), &b)
	if err != nil {
		return "", nil, nil, nil, 0, err
	}

	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Origin", "https://web.whatsapp.com")
	req.Header.Set("Referer", "https://web.whatsapp.com/")

	req.URL.Query().Set("f", "j")

	client := &http.Client{}
	// Submit the request
	res, err := client.Do(req)
	if err != nil {
		return "", nil, nil, nil, 0, err
	}

	if res.StatusCode != http.StatusOK {
		return "", nil, nil, nil, 0, fmt.Errorf("upload failed with status code %d", res.StatusCode)
	}

	var jsonRes map[string]string
	json.NewDecoder(res.Body).Decode(&jsonRes)

	return jsonRes["url"], mediaKey, fileEncSha256, fileSha256, fileLength, nil
}
