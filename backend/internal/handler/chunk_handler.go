package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"filebox/internal/config"
	"filebox/internal/service"
	"filebox/internal/utils"
)

// ChunkHandler exposes chunked upload endpoints.
type ChunkHandler struct {
	BaseHandler
	svc *service.ChunkUploadService
	cfg *config.Config
}

// NewChunkHandler creates a ChunkHandler.
func NewChunkHandler(svc *service.ChunkUploadService, cfg *config.Config) *ChunkHandler {
	return &ChunkHandler{svc: svc, cfg: cfg}
}

type uploadConfigResponse struct {
	UploadMaxDirect int64 `json:"upload_max_direct"`
	ChunkSize       int64 `json:"chunk_size"`
}

// UploadConfig returns chunk size and direct upload limit for the client.
// This endpoint is intentionally omitted from Swagger to keep the public API
// surface small; the frontend still calls it internally.
func (h *ChunkHandler) UploadConfig(c *fiber.Ctx) error {
	return utils.JSON(c, "upload config fetched", map[string]int64{
		"upload_max_direct": h.cfg.UploadMaxDirect,
		"chunk_size":        h.cfg.ChunkSize,
	})
}

// InitChunk initializes a chunked upload session. This endpoint is intentionally
// omitted from Swagger to keep the upload API surface simple; see /upload.
func (h *ChunkHandler) InitChunk(c *fiber.Ctx) error {
	userID, ok := h.UserID(c)
	if !ok {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	var req initChunkRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.ValidationError(c, "invalid request body", err.Error())
	}

	uploadID, err := h.svc.Init(c.Context(), userID, req.ChunkSize, req.TotalChunks, req.TotalSize, service.ChunkFileMeta{
		Name:      req.Name,
		Ext:       req.Ext,
		MIME:      req.MIME,
		Directory: req.Directory,
		Password:  req.Password,
	})
	if err != nil {
		return respondError(c, err)
	}
	return utils.Created(c, "chunk upload initialized", initChunkResponse{UploadID: uploadID, ChunkSize: req.ChunkSize})
}

type initChunkRequest struct {
	Name        string `json:"name"`
	Ext         string `json:"ext"`
	MIME        string `json:"mime"`
	Directory   string `json:"directory,omitempty"`
	Password    string `json:"password,omitempty"`
	ChunkSize   int    `json:"chunk_size"`
	TotalChunks int    `json:"total_chunks"`
	TotalSize   int64  `json:"total_size"`
}

type initChunkResponse struct {
	UploadID  string `json:"upload_id"`
	ChunkSize int    `json:"chunk_size"`
}

// UploadChunk receives one chunk for an existing chunked upload session.
func (h *ChunkHandler) UploadChunk(c *fiber.Ctx) error {
	userID, ok := h.UserID(c)
	if !ok {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	uploadID := c.Params("id")
	if uploadID == "" {
		return utils.Error(c, fiber.StatusBadRequest, "upload id is required")
	}

	index, err := strconv.Atoi(c.FormValue("index"))
	if err != nil || index < 0 {
		return utils.Error(c, fiber.StatusBadRequest, "invalid chunk index")
	}

	fh, err := c.FormFile("chunk")
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "chunk field is required")
	}

	src, err := fh.Open()
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "cannot read chunk")
	}
	defer src.Close()

	if err := h.svc.UploadChunk(c.Context(), userID, uploadID, index, src); err != nil {
		return respondError(c, err)
	}
	return utils.JSON(c, "chunk received", chunkResponse{Received: index})
}

// ChunkStatus returns which chunks have been received for a session.
func (h *ChunkHandler) ChunkStatus(c *fiber.Ctx) error {
	userID, ok := h.UserID(c)
	if !ok {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	uploadID := c.Params("id")
	if uploadID == "" {
		return utils.Error(c, fiber.StatusBadRequest, "upload id is required")
	}

	total, received, err := h.svc.Status(c.Context(), userID, uploadID)
	if err != nil {
		return respondError(c, err)
	}
	return utils.JSON(c, "chunk status fetched", statusResponse{TotalChunks: total, Received: received})
}

// CompleteChunk assembles all chunks and creates the file record.
func (h *ChunkHandler) CompleteChunk(c *fiber.Ctx) error {
	userID, ok := h.UserID(c)
	if !ok {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	uploadID := c.Params("id")
	if uploadID == "" {
		return utils.Error(c, fiber.StatusBadRequest, "upload id is required")
	}

	file, err := h.svc.Complete(c.Context(), userID, uploadID)
	if err != nil {
		return respondError(c, err)
	}
	return utils.Created(c, "chunk upload completed", file)
}

type chunkResponse struct {
	Received int `json:"received"`
}

type statusResponse struct {
	TotalChunks int   `json:"total_chunks"`
	Received    []int `json:"received"`
}
