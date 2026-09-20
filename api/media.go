package api

import (
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/gen2brain/beeep"
	"github.com/lugvitc/whats4linux/internal/store"
	mtypes "github.com/lugvitc/whats4linux/internal/types"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

// LinkPreviewResult is the link preview surfaced to the frontend.
type LinkPreviewResult struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Thumbnail   string `json:"thumbnail"` // data URL, or empty
}

// GetLinkPreview returns the stored preview card for a message's URL, or nil.
func (a *Api) GetLinkPreview(messageID string) *LinkPreviewResult {
	if a.messageStore == nil {
		return nil
	}
	lp := a.messageStore.GetLinkPreview(messageID)
	if lp == nil {
		return nil
	}
	res := &LinkPreviewResult{URL: lp.URL, Title: lp.Title, Description: lp.Description}
	if len(lp.Thumbnail) > 0 {
		res.Thumbnail = "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(lp.Thumbnail)
	}
	return res
}

// GetLinkPreviewImage returns the preview poster as a data URL, downloading and
// caching it on first request (WhatsApp ships the poster as an encrypted
// reference rather than embedded). Empty string if there's nothing to fetch.
func (a *Api) GetLinkPreviewImage(messageID string) string {
	if a.messageStore == nil {
		return ""
	}
	m := a.messageStore.GetLinkPreviewMedia(messageID)
	if m == nil {
		return ""
	}
	if len(m.Thumbnail) > 0 {
		return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(m.Thumbnail)
	}
	if m.DirectPath == "" || len(m.MediaKey) == 0 {
		return ""
	}
	if a.waClient == nil {
		return ""
	}
	data, err := a.waClient.DownloadMediaWithPath(
		a.ctx, m.DirectPath, m.FileEncSHA256, m.FileSHA256, m.MediaKey,
		whatsmeow.MediaLinkThumbnail, "thumbnail-link", true,
	)
	if err != nil || len(data) == 0 {
		return ""
	}
	if err := a.messageStore.CacheLinkPreviewThumbnail(messageID, data); err != nil {
		log.Printf("[GetLinkPreviewImage] failed to cache poster for %s: %v", messageID, err)
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(data)
}

// GetVideoThumbnail returns the message's embedded preview image as a data URL,
// or an empty string if none was stored. Lets the UI show a video preview + play
// button without downloading the full video.
func (a *Api) GetVideoThumbnail(messageID string) string {
	if a.messageStore == nil {
		return ""
	}
	thumb := a.messageStore.GetThumbnail(messageID)
	if len(thumb) == 0 {
		return ""
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(thumb)
}

func (a *Api) GetImageThumbnail(messageID string) string {
	if a.messageStore == nil {
		return ""
	}
	thumb := a.messageStore.GetThumbnail(messageID)
	if isUnavailableThumbnail(thumb) {
		return ""
	}
	if len(thumb) > 0 {
		return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(thumb)
	}
	if a.imageCache == nil {
		return ""
	}
	data, mime, err := a.imageCache.ReadImageByMessageID(messageID)
	if err != nil || len(data) == 0 {
		return ""
	}
	thumbData := generateThumbnail(data, mime)
	if len(thumbData) == 0 {
		if err := a.messageStore.CacheThumbnail(messageID, thumbnailUnavailable); err != nil {
			log.Printf("[GetImageThumbnail] failed to store negative thumbnail for %s: %v", messageID, err)
		}
		return ""
	}
	if err := a.messageStore.CacheThumbnail(messageID, thumbData); err != nil {
		log.Printf("[GetImageThumbnail] failed to cache thumbnail for %s: %v", messageID, err)
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(thumbData)
}

func (a *Api) DownloadMedia(chatJID string, messageID string) (string, error) {
	if a.messageStore == nil {
		return "", fmt.Errorf("message store is not ready")
	}
	msg, err := a.messageStore.GetMessageWithMedia(chatJID, messageID)
	if err != nil || msg == nil {
		return "", fmt.Errorf("message not found")
	}
	if msg.Media == nil {
		return "", fmt.Errorf("message %s has no downloadable media", messageID)
	}
	if a.waClient == nil {
		return "", fmt.Errorf("WhatsApp client is not ready")
	}

	mime := msg.Media.GetMimetype()
	width, height := msg.Media.GetDimensions()

	mediaType := msg.Media.GetMediaType()
	if mime == "" {
		// A correct MIME is required or <video>/<audio> won't play the data URL.
		switch mediaType {
		case whatsmeow.MediaImage:
			mime = "image/jpeg"
		case whatsmeow.MediaVideo:
			mime = "video/mp4"
		case whatsmeow.MediaAudio:
			mime = "audio/ogg"
		default:
			mime = "application/octet-stream"
		}
	}
	data, err := a.waClient.Download(a.ctx, msg.Media)
	if err != nil {
		return "", fmt.Errorf("failed to download media: %v", err)
	}

	// Save to cache for images and stickers
	if mediaType == whatsmeow.MediaImage {
		_, err = a.imageCache.SaveImage(messageID, data, mime, width, height)
		if err != nil {
			// Log error but continue
		}
	}

	// Return a ready-to-use data URL with the correct MIME.
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

// downloadMedia downloads media from a message and returns data, mime, width, height
func (a *Api) downloadMedia(msg *store.ExtendedMessage) ([]byte, string, int, int, error) {
	if msg == nil || msg.Media == nil {
		return nil, "", 0, 0, fmt.Errorf("message has no downloadable media")
	}
	if a.waClient == nil {
		return nil, "", 0, 0, fmt.Errorf("WhatsApp client is not ready")
	}
	data, err := a.waClient.Download(a.ctx, msg.Media)
	mime := msg.Media.GetMimetype()

	if mime == "" && msg.Media.GetMediaType() == whatsmeow.MediaImage {
		mime = "image/jpeg"
	}
	width, height := msg.Media.GetDimensions()

	return data, mime, width, height, err
}

func (a *Api) GetCachedImage(messageID string) (string, error) {
	data, mime, err := a.fetchImageBytes(messageID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(data)), nil
}

const (
	maxCachedImageBatchItems = 8
	maxCachedImageBatchBytes = 2 << 20
)

// GetCachedImages retrieves multiple cached images by message IDs (batch operation).
// Only already-cached image/sticker bytes are returned; the batch is capped so a
// single IPC string cannot evict on-screen images or freeze the UI.
func (a *Api) GetCachedImages(messageIDs []string) (map[string]string, error) {
	result := make(map[string]string)
	if a.imageCache == nil {
		return result, fmt.Errorf("image cache is not ready")
	}
	if len(messageIDs) > maxCachedImageBatchItems {
		messageIDs = messageIDs[:maxCachedImageBatchItems]
	}
	metas, err := a.imageCache.GetImagesByMessageIDs(messageIDs)
	if err != nil {
		return nil, err
	}

	remaining := maxCachedImageBatchBytes
	for _, msgID := range messageIDs {
		meta := metas[msgID]
		if meta == nil || !isImageMime(meta.Mime) {
			continue
		}
		data, mime, err := a.imageCache.ReadImageByMessageID(msgID)
		if err != nil || len(data) == 0 || !isImageMime(mime) {
			continue
		}
		encodedLen := base64.StdEncoding.EncodedLen(len(data))
		if encodedLen > remaining {
			continue
		}
		result[msgID] = fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(data))
		remaining -= encodedLen
		if remaining <= 0 {
			break
		}
	}

	return result, nil
}

func isImageMime(mime string) bool {
	mime = strings.ToLower(strings.TrimSpace(mime))
	switch mime {
	case "image/jpeg", "image/jpg", "image/png", "image/gif", "image/webp":
		return true
	}
	return strings.HasPrefix(mime, "image/") && !strings.HasPrefix(mime, "image/svg")
}

func isImageOrSticker(msg *store.ExtendedMessage) bool {
	if msg == nil || msg.Media == nil {
		return false
	}
	switch msg.Media.GetMediaGeneralType() {
	case mtypes.MediaTypeImage, mtypes.MediaTypeSticker:
		return true
	default:
		return false
	}
}

// fetchImageBytes returns the decoded image bytes and MIME for a message.
// It reads the on-disk image cache first and only downloads image/sticker media.
func (a *Api) fetchImageBytes(messageID string) (data []byte, mime string, err error) {
	if a.imageCache == nil {
		return nil, "", fmt.Errorf("image cache is not ready")
	}
	if data, mime, err = a.imageCache.ReadImageByMessageID(messageID); err == nil && len(data) > 0 {
		if !isImageMime(mime) {
			return nil, "", fmt.Errorf("cached media for %s is not an image", messageID)
		}
		return data, mime, nil
	}

	if a.messageStore == nil {
		return nil, "", fmt.Errorf("message store is not ready")
	}
	msg, err := a.messageStore.GetMessageWithMediaByID(messageID)
	if err != nil || msg == nil {
		return nil, "", fmt.Errorf("message not found")
	}
	if msg.Media == nil || !isImageOrSticker(msg) {
		return nil, "", fmt.Errorf("message %s has no downloadable image", messageID)
	}
	if storedMime := msg.Media.GetMimetype(); storedMime != "" && !isImageMime(storedMime) {
		return nil, "", fmt.Errorf("message %s has no downloadable image", messageID)
	}

	data, mime, width, height, err := a.downloadMedia(msg)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download image: %w", err)
	}
	if mime != "" && !isImageMime(mime) {
		return nil, "", fmt.Errorf("message %s has no downloadable image", messageID)
	}
	if mime == "" {
		mime = "image/jpeg"
	}
	if _, saveErr := a.imageCache.SaveImage(messageID, data, mime, width, height); saveErr != nil {
		log.Printf("[fetchImageBytes] failed to cache image for %s: %v", messageID, saveErr)
	}
	return data, mime, nil
}

// GetCachedAvatar retrieves or downloads and caches an avatar for a JID
func (a *Api) GetCachedAvatar(jid string, recache bool) (string, error) {
	if a.imageCache == nil {
		return "", fmt.Errorf("image cache is not ready")
	}

	// Try to get cached avatar data first
	data, mime, err := a.imageCache.ReadAvatarByJID(jid)

	if err == nil && !recache {
		avatarDataURL := fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(data))
		return avatarDataURL, nil
	}

	// Avatar not in cache, download and cache it
	if a.waClient == nil || a.waClient.Store == nil {
		return "", fmt.Errorf("WhatsApp client is not ready")
	}
	jidParsed, err := types.ParseJID(jid)
	if err != nil {
		return "", fmt.Errorf("invalid JID: %w", err)
	}

	// Get profile picture info. Community parent groups need IsCommunity: true.
	pic, err := a.waClient.GetProfilePictureInfo(a.ctx, jidParsed, &whatsmeow.GetProfilePictureParams{
		Preview: false, // Get full resolution
	})
	if (err != nil || pic == nil) && jidParsed.Server == types.GroupServer {
		pic, err = a.waClient.GetProfilePictureInfo(a.ctx, jidParsed, &whatsmeow.GetProfilePictureParams{
			Preview:     true,
			IsCommunity: true,
		})
	}
	if err != nil || pic == nil {
		if recache {
			a.startBackground(func() { _ = a.imageCache.DeleteAvatar(jid) })
		}
		return "", nil // No avatar available
	}

	return a.downloadAvatarFromURL(jid, pic.URL)
}

// downloadAvatarFromURL fetches an avatar image from URL, caches it, and
// returns a data URL. Used by both regular and community avatar paths.
func (a *Api) downloadAvatarFromURL(jid, url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to download avatar: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download avatar: status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read avatar data: %w", err)
	}

	mime := resp.Header.Get("Content-Type")
	if mime == "" {
		mime = "image/jpeg"
		if len(data) > 3 {
			switch {
			case data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47:
				mime = "image/png"
			case data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46:
				mime = "image/gif"
			case data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46:
				mime = "image/webp"
			}
		}
	}

	_, err = a.imageCache.SaveAvatar(jid, data, mime)
	if err != nil {
		log.Printf("[downloadAvatarFromURL] Failed to cache avatar for %s: %v", jid, err)
		return "", fmt.Errorf("failed to cache avatar: %w", err)
	}

	return fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(data)), nil
}

// GetSelfAvatar retrieves the avatar of the logged-in user
//
// We need to check canonical JID as if we check store's ID, it
// contains the device ID like so:
// XXXX:45@s.whatsapp.net instead of XXXX:@s.whatsapp.net
func (a *Api) GetSelfAvatar(recache bool) (string, error) {
	if a.waClient == nil || a.waClient.Store == nil || a.waClient.Store.ID == nil {
		return "", fmt.Errorf("not logged in")
	}
	jid := canonicalUserJID(a.ctx, a.waClient, *a.waClient.Store.ID)
	selfJID := jid.String()

	avatar, err := a.GetCachedAvatar(selfJID, recache)
	if err != nil {
		log.Printf("[SelfAvatar] GetCachedAvatar failed: %v", err)
		return "", err
	}

	if avatar == "" {
		log.Printf("[SelfAvatar] WhatsApp returned no avatar for self")
		return "", nil
	}

	return avatar, nil
}

func safeDownloadName(messageID, mime, storedName string) string {
	ext := getFileExtension(mime)
	if storedName = strings.TrimSpace(storedName); storedName != "" {
		base := filepath.Base(storedName)
		if base != "" && base != "." && base != ".." && !strings.Contains(base, string(filepath.Separator)) {
			return base
		}
	}
	id := filepath.Base(strings.TrimSpace(messageID))
	id = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 || unicode.IsControl(r) {
			return '_'
		}
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			return '_'
		}
		return r
	}, id)
	id = strings.Trim(id, " .")
	if id == "" || id == "." || id == ".." {
		id = "image"
	}
	if !strings.HasSuffix(strings.ToLower(id), ext) {
		id += ext
	}
	return id
}

// getFileExtension returns file extension for mime type
func getFileExtension(mime string) string {
	switch mime {
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/jpeg", "image/jpg":
		return ".jpg"
	default:
		return ".jpg"
	}
}

// DownloadImageToFile writes a cached (or freshly downloaded) image to ~/Downloads.
func (a *Api) DownloadImageToFile(messageID string) error {
	data, mime, err := a.fetchImageBytes(messageID)
	if err != nil {
		return fmt.Errorf("image not available: %w", err)
	}
	if len(data) == 0 {
		return fmt.Errorf("image not available")
	}

	storedName := ""
	if a.messageStore != nil {
		if msg, msgErr := a.messageStore.GetMessageWithMediaByID(messageID); msgErr == nil && msg != nil && msg.Media != nil {
			storedName = msg.Media.GetFileName()
		}
	}

	homeDir, _ := os.UserHomeDir()
	downloadsDir := filepath.Join(homeDir, "Downloads")
	fileName := safeDownloadName(messageID, mime, storedName)
	filePath := filepath.Join(downloadsDir, fileName)

	// Check if file exists and prompt for new path
	if _, err := os.Stat(filePath); err == nil {
		if filePath, err = runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
			DefaultDirectory: downloadsDir,
			DefaultFilename:  fileName,
			Title:            "File already exists. Save as...",
			Filters:          []runtime.FileFilter{{DisplayName: "Image Files", Pattern: "*" + getFileExtension(mime)}},
		}); err != nil || filePath == "" {
			return err
		}
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return err
	}

	beeep.Notify("whats4linux", "Downloaded: "+filePath, "")
	go func() {
		if _, err := exec.LookPath("mpg123"); err == nil {
			exec.Command("mpg123", "./beep.mp3").Run()
		}
	}()
	return nil
}
