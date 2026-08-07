package types

type StickerPack struct {
	StickerPackID    string             `json:"sticker-pack-id"`
	Name             string             `json:"name"`
	Publisher        string             `json:"publisher"`
	Description      string             `json:"description"`
	FileSize         string             `json:"file-size"`
	ImageDataHash    string             `json:"image-data-hash"`
	Stickers         []*StickerPackItem `json:"stickers"`
	Animated         int                `json:"animated"`
	Lottie           int                `json:"lottie"`
	PreviewImageIDs  []string           `json:"preview-image-ids"`
	TrayImageID      string             `json:"tray-image-id"`
	TrayImagePreview string             `json:"tray-image-preview"`
}

type StickerPackItem struct {
	MediaKey               []byte   `json:"media-key"`
	EncFileHash            []byte   `json:"enc-file-hash"`
	FileHash               []byte   `json:"file-hash"`
	DirectPath             string   `json:"direct-path"`
	URL                    string   `json:"url"`
	FileSize               int64    `json:"file-size"`
	MimeType               string   `json:"mimetype"`
	Height                 int      `json:"height"`
	Width                  int      `json:"width"`
	Emojis                 []string `json:"emojis"`
	AccessibilityText      string   `json:"accessibility-text"`
	Handle                 string   `json:"handle"`
	StickerHashWithoutMeta []byte   `json:"sticker-hash-without-meta"`
	PreviewWebpID          string   `json:"preview-webp-id"`
}

func (spi *StickerPackItem) GetDirectPath() string {
	return spi.DirectPath
}

func (spi *StickerPackItem) GetMediaKey() []byte {
	return spi.MediaKey
}

func (spi *StickerPackItem) GetFileSHA256() []byte {
	return spi.FileHash
}

func (spi *StickerPackItem) GetFileEncSHA256() []byte {
	return spi.EncFileHash
}

func (spi *StickerPackItem) GetFileSizeBytes() int64 {
	return spi.FileSize
}
