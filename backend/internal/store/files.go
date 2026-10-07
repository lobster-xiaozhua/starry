package store

import (
	"bytes"
	"errors"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/image/draw"
)

const (
	MaxAttachmentBytes = 5 << 20
	ThumbWidth         = 320
)

var allowedMIME = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

type FileStore struct {
	uploadDir string
}

func NewFileStore(uploadDir string) *FileStore {
	return &FileStore{uploadDir: uploadDir}
}

func (f *FileStore) Init() error {
	if err := os.MkdirAll(filepath.Join(f.uploadDir, "thumbs"), 0o755); err != nil {
		return err
	}
	return nil
}

// Save 校验 MIME/大小并落盘，返回公开 URL 与缩略图 URL。
func (f *FileStore) Save(mime string, r io.Reader) (url, thumbURL string, size int64, err error) {
	ext, ok := allowedMIME[mime]
	if !ok {
		return "", "", 0, errors.New("unsupported image type")
	}
	data, err := io.ReadAll(io.LimitReader(r, MaxAttachmentBytes+1))
	if err != nil {
		return "", "", 0, err
	}
	if int64(len(data)) > MaxAttachmentBytes {
		return "", "", 0, errors.New("file exceeds 5MB limit")
	}
	name := uuid.NewString() + ext
	dst := filepath.Join(f.uploadDir, name)
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return "", "", 0, err
	}
	thumb, _ := f.generateThumb(name, data)
	return "/uploads/" + name, thumb, int64(len(data)), nil
}

func (f *FileStore) generateThumb(name string, data []byte) (string, error) {
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	sb := src.Bounds()
	scale := float64(ThumbWidth) / float64(sb.Dx())
	if scale > 1 {
		scale = 1
	}
	newW := ThumbWidth
	newH := int(float64(sb.Dy()) * scale)
	if newH < 1 {
		newH = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	draw.BiLinear.Scale(dst, dst.Bounds(), src, sb, draw.Over, nil)

	out := filepath.Join(f.uploadDir, "thumbs", name+".jpg")
	fh, err := os.Create(out)
	if err != nil {
		return "", err
	}
	defer fh.Close()
	if err := jpeg.Encode(fh, dst, &jpeg.Options{Quality: 85}); err != nil {
		return "", err
	}
	return "/uploads/thumbs/" + name + ".jpg", nil
}

// PathForURL 将公开 URL 转换为磁盘路径。
func (f *FileStore) PathForURL(url string) string {
	rel := strings.TrimPrefix(url, "/uploads/")
	return filepath.Join(f.uploadDir, rel)
}

// DeleteURLs 删除附件磁盘文件（忽略不存在）。
func (f *FileStore) DeleteURLs(urls []string) {
	for _, u := range urls {
		if u == "" {
			continue
		}
		os.Remove(f.PathForURL(u))
		os.Remove(f.PathForURL(u + ".jpg"))
	}
}
