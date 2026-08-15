package handler

import (
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v2"

	"filebox/internal/config"
	"filebox/internal/service"
	"filebox/internal/utils"
)

// ItemHandler exposes the unified item (file + folder) endpoints.
type ItemHandler struct {
	BaseHandler
	svc *service.ItemService
	cfg *config.Config
}

// NewItemHandler creates an ItemHandler.
func NewItemHandler(svc *service.ItemService, cfg *config.Config) *ItemHandler {
	return &ItemHandler{svc: svc, cfg: cfg}
}

// ItemListRequest is the query for listing folder contents.
type ItemListRequest struct {
	Directory string `json:"directory" query:"directory"`
}

// ItemFolderRequest is the body for creating a folder.
type ItemFolderRequest struct {
	Directory string `json:"directory"`
}

// ItemUploadByURLRequest is the body for importing a file from a public URL.
type ItemUploadByURLRequest struct {
	Directory string   `json:"directory"`
	URL       string   `json:"url"`
	Favorite  bool     `json:"favorite"`
	ShareWith []string `json:"share_with"`
	Password  string   `json:"password"`
}

// ItemUpdateRequest is the body for updating an item.
type ItemUpdateRequest struct {
	Directory  string   `json:"directory"`
	Name       string   `json:"name,omitempty"`
	IsFavorite *bool    `json:"is_favorite,omitempty"`
	Shares     []string `json:"shares,omitempty"`
	Password   string   `json:"password,omitempty"`
}

// ItemDeleteRequest is the query for deleting an item.
type ItemDeleteRequest struct {
	Directory string `json:"directory" query:"directory"`
}

// ItemDetailRequest is the query for fetching item details.
type ItemDetailRequest struct {
	Directory string `json:"directory" query:"directory"`
}

// ItemDownloadRequest is the query for downloading a file.
type ItemDownloadRequest struct {
	Directory string `json:"directory" query:"directory"`
	Password  string `json:"password" query:"password"`
}

// ItemSearchRequest is the query for searching items.
type ItemSearchRequest struct {
	Q string `json:"q" query:"q"`
}

// List godoc
// @Summary      List items
// @Description  List the contents of a folder (files + subfolders). Empty directory means root.
// @Tags         items
// @Produce      json
// @Param        directory  query  string  false  "Folder path"
// @Success      200  {object}  utils.SuccessResponse{data=[]entity.Item}
// @Failure      404  {object}  utils.ErrorResponse
// @Security     BearerAuth
// @Router       /item/list [get]
func (h *ItemHandler) List(c *fiber.Ctx) error {
	userID, ok := h.UserID(c)
	if !ok {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}
	var req ItemListRequest
	_ = c.QueryParser(&req)
	items, err := h.svc.List(c.Context(), userID, req.Directory)
	if err != nil {
		return respondError(c, err)
	}
	return utils.JSON(c, "items fetched", items)
}

// Detail godoc
// @Summary      Item detail
// @Description  Get metadata for a file or folder. Files include their share list.
// @Tags         items
// @Produce      json
// @Param        directory  query  string  true  "Item path"
// @Success      200  {object}  utils.SuccessResponse{data=service.ItemDetail}
// @Failure      404  {object}  utils.ErrorResponse
// @Security     BearerAuth
// @Router       /item/detail [get]
func (h *ItemHandler) Detail(c *fiber.Ctx) error {
	userID, ok := h.UserID(c)
	if !ok {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}
	var req ItemDetailRequest
	if err := c.QueryParser(&req); err != nil || req.Directory == "" {
		return utils.ValidationError(c, "directory is required", "")
	}
	detail, err := h.svc.Detail(c.Context(), userID, req.Directory)
	if err != nil {
		return respondError(c, err)
	}
	return utils.JSON(c, "item detail fetched", detail)
}

// CreateFolder godoc
// @Summary      Create folder
// @Description  Create a new folder at the given path. Parent folders must already exist.
// @Tags         items
// @Accept       json
// @Produce      json
// @Param        body  body  ItemFolderRequest  true  "Folder path"
// @Success      201  {object}  utils.SuccessResponse{data=entity.Item}
// @Failure      400  {object}  utils.ErrorResponse
// @Security     BearerAuth
// @Router       /item/folder [post]
func (h *ItemHandler) CreateFolder(c *fiber.Ctx) error {
	userID, ok := h.UserID(c)
	if !ok {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}
	var req ItemFolderRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.ValidationError(c, "invalid request body", err.Error())
	}
	folder, err := h.svc.CreateFolder(c.Context(), userID, req.Directory)
	if err != nil {
		return respondError(c, err)
	}
	return utils.Created(c, "folder created", folder)
}

// Upload godoc
// @Summary      Upload a file
// @Description  Upload a file directly to the given directory. For large files, use chunked upload.
// @Tags         items
// @Accept       multipart/form-data
// @Produce      json
// @Param        file        formData  file    true  "File bytes"
// @Param        directory   formData  string  false  "Destination folder path"
// @Param        favorite    formData  bool    false  "Add to favorites"
// @Param        password    formData  string  false  "File password"
// @Param        share_with  formData  []string false  "Emails to share with"
// @Success      201  {object}  utils.SuccessResponse{data=entity.Item}
// @Failure      400  {object}  utils.ErrorResponse
// @Security     BearerAuth
// @Router       /item/upload [post]
func (h *ItemHandler) Upload(c *fiber.Ctx) error {
	userID, ok := h.UserID(c)
	if !ok {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	fh, err := c.FormFile("file")
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "file is required")
	}
	if fh.Filename == "" {
		return utils.Error(c, fiber.StatusBadRequest, "file name is required")
	}
	if fh.Size == 0 {
		return utils.Error(c, fiber.StatusBadRequest, "file is empty")
	}

	fav := false
	if c.FormValue("favorite") == "true" || c.FormValue("favorite") == "1" {
		fav = true
	}
	var shareRecipients []string
	if raw := c.FormValue("share_with"); raw != "" {
		shareRecipients = strings.Split(raw, ",")
	}

	src, err := fh.Open()
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "cannot read file")
	}
	defer src.Close()

	file, err := h.svc.Upload(c.Context(), service.ItemUploadInput{
		UserID:          userID,
		Directory:       c.FormValue("directory"),
		Name:            fh.Filename,
		MIME:            fh.Header.Get("Content-Type"),
		Content:         src,
		DeclaredSize:    fh.Size,
		Favorite:        fav,
		ShareRecipients: shareRecipients,
		Password:        c.FormValue("password"),
	})
	if err != nil {
		return respondError(c, err)
	}
	return utils.Created(c, "file uploaded", file)
}

// UploadByURL godoc
// @Summary      Upload a file from a URL
// @Description  Download a file from a public URL and store it like a normal upload. Only http/https URLs are accepted; private/localhost hosts are rejected.
// @Tags         items
// @Accept       json
// @Produce      json
// @Param        body  body  ItemUploadByURLRequest  true  "URL upload payload"
// @Success      201  {object}  utils.SuccessResponse{data=entity.Item}
// @Failure      400  {object}  utils.ErrorResponse
// @Security     BearerAuth
// @Router       /item/upload-by-url [post]
func (h *ItemHandler) UploadByURL(c *fiber.Ctx) error {
	userID, ok := h.UserID(c)
	if !ok {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}
	var req ItemUploadByURLRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.ValidationError(c, "invalid request body", err.Error())
	}
	if req.URL == "" {
		return utils.ValidationError(c, "url is required", "")
	}

	file, err := h.svc.UploadByURL(c.Context(), service.ItemUploadByURLInput{
		UserID:          userID,
		Directory:       req.Directory,
		URL:             req.URL,
		Favorite:        req.Favorite,
		ShareRecipients: req.ShareWith,
		Password:        req.Password,
	})
	if err != nil {
		return respondError(c, err)
	}
	return utils.Created(c, "file uploaded from URL", file)
}

// Update godoc
// @Summary      Update an item
// @Description  Rename a file/folder, set file favorite, replace the entire file share list, or set/clear file password. Folders only support rename. The shares array overwrites the current share list when provided.
// @Tags         items
// @Accept       json
// @Produce      json
// @Param        body  body  ItemUpdateRequest  true  "Update payload"
// @Success      200  {object}  utils.SuccessResponse{data=entity.Item}
// @Failure      400  {object}  utils.ErrorResponse
// @Failure      404  {object}  utils.ErrorResponse
// @Security     BearerAuth
// @Router       /item/update [put]
func (h *ItemHandler) Update(c *fiber.Ctx) error {
	userID, ok := h.UserID(c)
	if !ok {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}
	var req ItemUpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.ValidationError(c, "invalid request body", err.Error())
	}
	if req.Directory == "" {
		return utils.ValidationError(c, "directory is required", "")
	}
	updated, err := h.svc.Update(c.Context(), userID, req.Directory, req.Name, req.IsFavorite, req.Shares, req.Password)
	if err != nil {
		return respondError(c, err)
	}
	return utils.JSON(c, "item updated", updated)
}

// Delete godoc
// @Summary      Delete an item
// @Description  Permanently delete a file or folder (folders are deleted recursively).
// @Tags         items
// @Produce      json
// @Param        directory  query  string  true  "Item path"
// @Success      200  {object}  utils.SuccessResponse
// @Failure      404  {object}  utils.ErrorResponse
// @Security     BearerAuth
// @Router       /item/delete [delete]
func (h *ItemHandler) Delete(c *fiber.Ctx) error {
	userID, ok := h.UserID(c)
	if !ok {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}
	var req ItemDeleteRequest
	if err := c.QueryParser(&req); err != nil || req.Directory == "" {
		return utils.ValidationError(c, "directory is required", "")
	}
	if err := h.svc.Delete(c.Context(), userID, req.Directory); err != nil {
		return respondError(c, err)
	}
	return utils.JSON(c, "item deleted", nil)
}

// Download godoc
// @Summary      Download a file
// @Description  Download a file by path. If the file is password-protected, supply the password.
// @Tags         items
// @Produce      octet-stream
// @Param        directory  query  string  true  "File path"
// @Param        password   query  string  false  "File password"
// @Success      200
// @Failure      403  {object}  utils.ErrorResponse
// @Failure      404  {object}  utils.ErrorResponse
// @Security     BearerAuth
// @Router       /item/download [get]
func (h *ItemHandler) Download(c *fiber.Ctx) error {
	userID, ok := h.UserID(c)
	if !ok {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}
	var req ItemDownloadRequest
	if err := c.QueryParser(&req); err != nil || req.Directory == "" {
		return utils.ValidationError(c, "directory is required", "")
	}
	item, err := h.svc.GetByPathOrShared(c.Context(), userID, req.Directory)
	if err != nil {
		return respondError(c, err)
	}
	if err := h.svc.VerifyPassword(item, req.Password); err != nil {
		return respondError(c, err)
	}
	tempPath, err := h.svc.DecryptToTempPath(c.Context(), item)
	if err != nil {
		utils.Log.Error().Err(err).Uint64("item_id", item.ID).Str("path", item.Path).Msg("failed to decrypt file for download")
		return respondError(c, err)
	}
	defer func() { _ = removeFile(tempPath) }()

	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, item.Name))
	if item.MIME != nil && *item.MIME != "" {
		c.Set(fiber.HeaderContentType, *item.MIME)
	}
	return c.SendFile(tempPath)
}

// Preview godoc
// @Summary      Preview a file
// @Description  Preview a file inline by path. If the file is password-protected, supply the password.
// @Tags         items
// @Produce      octet-stream
// @Param        directory  query  string  true  "File path"
// @Param        password   query  string  false  "File password"
// @Success      200
// @Failure      403  {object}  utils.ErrorResponse
// @Failure      404  {object}  utils.ErrorResponse
// @Security     BearerAuth
// @Router       /item/preview [get]
func (h *ItemHandler) Preview(c *fiber.Ctx) error {
	userID, ok := h.UserID(c)
	if !ok {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}
	var req ItemDownloadRequest
	if err := c.QueryParser(&req); err != nil || req.Directory == "" {
		return utils.ValidationError(c, "directory is required", "")
	}
	item, err := h.svc.GetByPathOrShared(c.Context(), userID, req.Directory)
	if err != nil {
		return respondError(c, err)
	}
	if err := h.svc.VerifyPassword(item, req.Password); err != nil {
		return respondError(c, err)
	}
	tempPath, err := h.svc.DecryptToTempPath(c.Context(), item)
	if err != nil {
		utils.Log.Error().Err(err).Uint64("item_id", item.ID).Str("path", item.Path).Msg("failed to decrypt file for preview")
		return respondError(c, err)
	}
	defer func() { _ = removeFile(tempPath) }()

	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`inline; filename="%s"`, item.Name))
	contentType := ""
	if item.MIME != nil {
		contentType = strings.TrimSpace(*item.MIME)
	}
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = mime.TypeByExtension(filepath.Ext(item.FullName()))
	}
	if contentType != "" {
		c.Set(fiber.HeaderContentType, contentType)
	}
	return c.SendFile(tempPath)
}

// Search godoc
// @Summary      Search items
// @Description  Search files and folders by name.
// @Tags         items
// @Produce      json
// @Param        q  query  string  true  "Search query"
// @Success      200  {object}  utils.SuccessResponse{data=[]entity.Item}
// @Security     BearerAuth
// @Router       /item/search [get]
func (h *ItemHandler) Search(c *fiber.Ctx) error {
	userID, ok := h.UserID(c)
	if !ok {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		return utils.ValidationError(c, "query is required", "")
	}
	items, err := h.svc.Search(c.Context(), userID, q)
	if err != nil {
		return respondError(c, err)
	}
	return utils.JSON(c, "search results", items)
}

// Shared godoc
// @Summary      Shared with me
// @Description  List files that other users have shared with the current user.
// @Tags         items
// @Produce      json
// @Success      200  {object}  utils.SuccessResponse{data=[]entity.SharedItem}
// @Security     BearerAuth
// @Router       /item/shared [get]
func (h *ItemHandler) Shared(c *fiber.Ctx) error {
	userID, ok := h.UserID(c)
	if !ok {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}
	items, err := h.svc.ListShared(c.Context(), userID)
	if err != nil {
		return respondError(c, err)
	}
	return utils.JSON(c, "shared items fetched", items)
}

// Favorites godoc
// @Summary      Favorites
// @Description  List the user's favorite files.
// @Tags         items
// @Produce      json
// @Success      200  {object}  utils.SuccessResponse{data=[]entity.Item}
// @Security     BearerAuth
// @Router       /item/favorites [get]
func (h *ItemHandler) Favorites(c *fiber.Ctx) error {
	userID, ok := h.UserID(c)
	if !ok {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}
	items, err := h.svc.ListFavorites(c.Context(), userID)
	if err != nil {
		return respondError(c, err)
	}
	return utils.JSON(c, "favorites fetched", items)
}

func removeFile(path string) error {
	if path == "" {
		return nil
	}
	return os.Remove(path)
}
