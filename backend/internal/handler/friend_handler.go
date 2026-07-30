package handler

import (
	"github.com/gofiber/fiber/v2"

	"filebox/internal/service"
	"filebox/internal/utils"
)

// FriendHandler exposes friend/contact endpoints.
type FriendHandler struct {
	BaseHandler
	svc *service.FriendService
}

// NewFriendHandler creates a FriendHandler.
func NewFriendHandler(svc *service.FriendService) *FriendHandler {
	return &FriendHandler{svc: svc}
}

type addFriendRequest struct {
	Email string `json:"email"`
}

// Create godoc
// @Summary      Send a friend request
// @Description  Sends a friend request to a user by email.
// @Tags         friends
// @Accept       json
// @Produce      json
// @Param        body  body      addFriendRequest  true  "Friend email"
// @Success      201   {object}  utils.SuccessResponse{data=entity.FriendRequest}
// @Failure      400   {object}  utils.ErrorResponse
// @Failure      401   {object}  utils.ErrorResponse
// @Security     BearerAuth
// @Router       /friends [post]
func (h *FriendHandler) Create(c *fiber.Ctx) error {
	userID, ok := h.UserID(c)
	if !ok {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	var req addFriendRequest
	if err := c.BodyParser(&req); err != nil {
		return utils.ValidationError(c, "invalid request body", err.Error())
	}

	friendReq, err := h.svc.Add(c.Context(), userID, req.Email)
	if err != nil {
		return respondError(c, err)
	}
	return utils.Created(c, "friend request sent", friendReq)
}

// List godoc
// @Summary      List friends
// @Description  Returns the current user's accepted friend list.
// @Tags         friends
// @Produce      json
// @Success      200  {object}  utils.SuccessResponse{data=[]entity.Friend}
// @Failure      401  {object}  utils.ErrorResponse
// @Security     BearerAuth
// @Router       /friends [get]
func (h *FriendHandler) List(c *fiber.Ctx) error {
	userID, ok := h.UserID(c)
	if !ok {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	friends, err := h.svc.List(c.Context(), userID)
	if err != nil {
		return respondError(c, err)
	}
	return utils.JSON(c, "friends fetched", friends)
}

// ListRequests godoc
// @Summary      List friend requests
// @Description  Returns pending friend requests for the current user.
// @Tags         friends
// @Produce      json
// @Param        direction  query  string  false  "Filter by direction: incoming, outgoing"
// @Success      200  {object}  utils.SuccessResponse{data=[]entity.FriendRequest}
// @Failure      401  {object}  utils.ErrorResponse
// @Security     BearerAuth
// @Router       /friends/requests [get]
func (h *FriendHandler) ListRequests(c *fiber.Ctx) error {
	userID, ok := h.UserID(c)
	if !ok {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	direction := c.Query("direction", "")
	requests, err := h.svc.ListRequests(c.Context(), userID, direction)
	if err != nil {
		return respondError(c, err)
	}
	return utils.JSON(c, "friend requests fetched", requests)
}

// AcceptRequest godoc
// @Summary      Accept a friend request
// @Description  Accepts an incoming friend request and creates a mutual friendship.
// @Tags         friends
// @Produce      json
// @Param        id  path  uint64  true  "Request ID"
// @Success      200  {object}  utils.SuccessResponse{data=entity.FriendRequest}
// @Failure      400  {object}  utils.ErrorResponse
// @Failure      401  {object}  utils.ErrorResponse
// @Security     BearerAuth
// @Router       /friends/requests/{id}/accept [post]
func (h *FriendHandler) AcceptRequest(c *fiber.Ctx) error {
	userID, ok := h.UserID(c)
	if !ok {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	id, err := h.ParseUintParam(c, "id")
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid request id")
	}

	req, err := h.svc.AcceptRequest(c.Context(), userID, id)
	if err != nil {
		return respondError(c, err)
	}
	return utils.JSON(c, "friend request accepted", req)
}

// RejectRequest godoc
// @Summary      Reject a friend request
// @Description  Rejects an incoming friend request.
// @Tags         friends
// @Produce      json
// @Param        id  path  uint64  true  "Request ID"
// @Success      200  {object}  utils.SuccessResponse{data=entity.FriendRequest}
// @Failure      400  {object}  utils.ErrorResponse
// @Failure      401  {object}  utils.ErrorResponse
// @Security     BearerAuth
// @Router       /friends/requests/{id}/reject [post]
func (h *FriendHandler) RejectRequest(c *fiber.Ctx) error {
	userID, ok := h.UserID(c)
	if !ok {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	id, err := h.ParseUintParam(c, "id")
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid request id")
	}

	req, err := h.svc.RejectRequest(c.Context(), userID, id)
	if err != nil {
		return respondError(c, err)
	}
	return utils.JSON(c, "friend request rejected", req)
}

// CancelRequest godoc
// @Summary      Cancel a friend request
// @Description  Cancels an outgoing friend request.
// @Tags         friends
// @Produce      json
// @Param        id  path  uint64  true  "Request ID"
// @Success      200  {object}  utils.SuccessResponse
// @Failure      400  {object}  utils.ErrorResponse
// @Failure      401  {object}  utils.ErrorResponse
// @Security     BearerAuth
// @Router       /friends/requests/{id} [delete]
func (h *FriendHandler) CancelRequest(c *fiber.Ctx) error {
	userID, ok := h.UserID(c)
	if !ok {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	id, err := h.ParseUintParam(c, "id")
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid request id")
	}

	if err := h.svc.CancelRequest(c.Context(), userID, id); err != nil {
		return respondError(c, err)
	}
	return utils.JSON(c, "friend request cancelled", nil)
}

// Delete godoc
// @Summary      Remove friend
// @Description  Removes a mutual friend relationship.
// @Tags         friends
// @Produce      json
// @Param        id  path  uint64  true  "Friend ID"
// @Success      200  {object}  utils.SuccessResponse
// @Failure      401  {object}  utils.ErrorResponse
// @Failure      404  {object}  utils.ErrorResponse
// @Security     BearerAuth
// @Router       /friends/{id} [delete]
func (h *FriendHandler) Delete(c *fiber.Ctx) error {
	userID, ok := h.UserID(c)
	if !ok {
		return utils.Error(c, fiber.StatusUnauthorized, "unauthorized")
	}

	id, err := h.ParseUintParam(c, "id")
	if err != nil {
		return utils.Error(c, fiber.StatusBadRequest, "invalid friend id")
	}

	if err := h.svc.Delete(c.Context(), userID, id); err != nil {
		return respondError(c, err)
	}
	return utils.JSON(c, "friend removed", nil)
}
