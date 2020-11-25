package whatsapp

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/Rhymen/go-whatsapp/crypto/cbc"
	"github.com/Rhymen/go-whatsapp/crypto/hkdf"
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
		return nil, fmt.Errorf("file length does not match. Expected: %v, got: %v", fileLength, len(data))
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
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("download failed with status code %d", resp.StatusCode)
	}
	if resp.ContentLength <= 10 {
		return nil, nil, fmt.Errorf("file to short")
	}
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	n := len(data)
	return data[:n-10], data[n-10 : n], nil
}

type MediaConn struct {
	Status    int `json:"status"`
	MediaConn struct {
		Auth  string `json:"auth"`
		TTL   int    `json:"ttl"`
		Hosts []struct {
			Hostname string `json:"hostname"`
			IPs      []struct {
				IP4 net.IP `json:"ip4"`
				IP6 net.IP `json:"ip6"`
			} `json:"ips"`
		} `json:"hosts"`
	} `json:"media_conn"`
}

func (wac *Conn) queryMediaConn() (hostname, auth string, ttl int, err error) {
	queryReq := []interface{}{"query", "mediaConn"}
	ch, err := wac.writeJson(queryReq)
	if err != nil {
		return "", "", 0, err
	}

	var resp MediaConn
	select {
	case r := <-ch:
		if err = json.Unmarshal([]byte(r), &resp); err != nil {
			return "", "", 0, fmt.Errorf("error decoding query media conn response: %v", err)
		}
	case <-time.After(wac.msgTimeout):
		return "", "", 0, fmt.Errorf("query media conn timed out")
	}

	if resp.Status != http.StatusOK {
		return "", "", 0, fmt.Errorf("query media conn responded with %d", resp.Status)
	}

	for _, h := range resp.MediaConn.Hosts {
		if h.Hostname != "" {
			return h.Hostname, resp.MediaConn.Auth, resp.MediaConn.TTL, nil
		}
	}

	return "", "", 0, fmt.Errorf("query media conn responded with no host")
}

var mediaTypeMap = map[MediaType]string{
	MediaImage:    "/mms/image",
	MediaVideo:    "/mms/video",
	MediaDocument: "/mms/document",
	MediaAudio:    "/mms/audio",
}

func (wac *Conn) Upload(reader io.Reader, appInfo MediaType) (downloadURL string, mediaKey []byte, fileEncSha256 []byte, fileSha256 []byte, fileLength uint64, err error) {
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

	hostname, auth, _, err := wac.queryMediaConn()
	if err != nil {
		return "", nil, nil, nil, 0, err
	}

	token := base64.URLEncoding.EncodeToString(fileEncSha256)
	q := url.Values{
		"auth":  []string{auth},
		"token": []string{token},
	}
	path := mediaTypeMap[appInfo]
	uploadURL := url.URL{
		Scheme:   "https",
		Host:     hostname,
		Path:     fmt.Sprintf("%s/%s", path, token),
		RawQuery: q.Encode(),
	}

	body := bytes.NewReader(append(enc, mac...))

	req, err := http.NewRequest(http.MethodPost, uploadURL.String(), body)
	if err != nil {
		return "", nil, nil, nil, 0, err
	}

	req.Header.Set("Origin", "https://web.whatsapp.com")
	req.Header.Set("Referer", "https://web.whatsapp.com/")

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
	if err := json.NewDecoder(res.Body).Decode(&jsonRes); err != nil {
		return "", nil, nil, nil, 0, err
	}

	return jsonRes["url"], mediaKey, fileEncSha256, fileSha256, fileLength, nil
}
