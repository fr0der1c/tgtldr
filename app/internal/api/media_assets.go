package api

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/fr0der1c/tgtldr/app/internal/httpx"
	"github.com/jackc/pgx/v5"
)

// handleAssetContent 在登录保护下提供本地资源，并由 ServeContent 处理 Range 和 HEAD。
func (r *Router) handleAssetContent(w http.ResponseWriter, req *http.Request) {
	assetID, err := strconv.ParseInt(req.PathValue("assetID"), 10, 64)
	if err != nil || assetID <= 0 {
		httpx.Error(w, http.StatusBadRequest, "invalid asset id")
		return
	}
	asset, err := r.store.Assets.Get(req.Context(), assetID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusNotFound
		}
		httpx.Error(w, status, err.Error())
		return
	}
	if asset.Status != "succeeded" || asset.LocalPath == "" {
		httpx.Error(w, http.StatusConflict, "asset is not ready")
		return
	}
	path, err := safeMediaPath(r.mediaDir, asset.LocalPath)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	file, err := os.Open(path)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "asset file not found")
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", asset.MIMEType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", contentDisposition(asset.Kind, asset.FileName))
	w.Header().Set("ETag", fmt.Sprintf(`"asset-%d-%d"`, asset.ID, info.ModTime().Unix()))
	http.ServeContent(w, req, asset.FileName, info.ModTime(), file)
}

// handleAssetDownload 将失败或超限资源重新排队，真实下载由账号 worker 执行。
func (r *Router) handleAssetDownload(w http.ResponseWriter, req *http.Request) {
	assetID, err := strconv.ParseInt(req.PathValue("assetID"), 10, 64)
	if err != nil || assetID <= 0 {
		httpx.Error(w, http.StatusBadRequest, "invalid asset id")
		return
	}
	if err := r.store.Assets.RequestDownload(req.Context(), assetID); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, pgx.ErrNoRows) {
			status = http.StatusConflict
		}
		httpx.Error(w, status, err.Error())
		return
	}
	httpx.JSON(w, http.StatusAccepted, map[string]string{"status": "pending"})
}

// safeMediaPath 确保数据库中的相对路径无法逃逸媒体根目录。
func safeMediaPath(root string, relative string) (string, error) {
	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve media root: %w", err)
	}
	path, err := filepath.Abs(filepath.Join(cleanRoot, relative))
	if err != nil {
		return "", fmt.Errorf("resolve media path: %w", err)
	}
	if path != cleanRoot && !strings.HasPrefix(path, cleanRoot+string(os.PathSeparator)) {
		return "", fmt.Errorf("invalid media path")
	}
	return path, nil
}

// contentDisposition 只允许受支持媒体内联，其余文件强制作为附件下载。
func contentDisposition(kind string, fileName string) string {
	disposition := "attachment"
	if kind == "photo" || kind == "avatar" || kind == "video" || kind == "audio" || kind == "voice" {
		disposition = "inline"
	}
	name := strings.Map(func(r rune) rune {
		if r < 32 || r == '"' || r == '\\' {
			return '_'
		}
		return r
	}, fileName)
	return fmt.Sprintf(`%s; filename="%s"`, disposition, name)
}
