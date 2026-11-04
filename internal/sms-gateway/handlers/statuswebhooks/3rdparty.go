package statuswebhooks

import (
	"fmt"

	"github.com/android-sms-gateway/server/internal/sms-gateway/handlers/base"
	"github.com/android-sms-gateway/server/internal/sms-gateway/handlers/middlewares/permissions"
	"github.com/android-sms-gateway/server/internal/sms-gateway/handlers/middlewares/userauth"
	"github.com/android-sms-gateway/server/internal/sms-gateway/modules/statuswebhooks"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type thirdPartyControllerParams struct {
	fx.In

	StatusWebhooksSvc *statuswebhooks.Service

	Validator *validator.Validate
	Logger    *zap.Logger
}

type ThirdPartyController struct {
	base.Handler

	statusWebhooksSvc *statuswebhooks.Service
}

func NewThirdPartyController(params thirdPartyControllerParams) *ThirdPartyController {
	return &ThirdPartyController{
		Handler: base.Handler{
			Logger:    params.Logger,
			Validator: params.Validator,
		},
		statusWebhooksSvc: params.StatusWebhooksSvc,
	}
}

type statusWebhookRequest struct {
	ID     string   `json:"id,omitempty"`
	URL    string   `json:"url"    validate:"required,url"`
	Events []string `json:"events" validate:"required,min=1"`
}

//	@Summary		List status webhooks
//	@Description	Returns webhooks registered to receive message-status-change events
//	@Security		ApiAuth
//	@Security		JWTAuth
//	@Tags			User, Status Webhooks
//	@Produce		json
//	@Success		200	{array}		statuswebhooks.StatusWebhookDTO
//	@Failure		401	{object}	smsgateway.ErrorResponse	"Unauthorized"
//	@Failure		403	{object}	smsgateway.ErrorResponse	"Forbidden"
//	@Failure		500	{object}	smsgateway.ErrorResponse	"Internal server error"
//	@Router			/3rdparty/v1/status-webhooks [get]
//
// List status webhooks.
func (h *ThirdPartyController) get(userID string, c *fiber.Ctx) error {
	items, err := h.statusWebhooksSvc.Select(userID)
	if err != nil {
		return fmt.Errorf("failed to select status webhooks: %w", err)
	}

	return c.JSON(items)
}

//	@Summary		Register a status webhook
//	@Description	Registers a webhook POSTed to on message status changes (sent/delivered/failed/cancelled). Signed with HMAC-SHA256 in the X-Signature-256 header. If the webhook with the same ID already exists, it's replaced.
//	@Security		ApiAuth
//	@Security		JWTAuth
//	@Tags			User, Status Webhooks
//	@Accept			json
//	@Produce		json
//	@Param			request	body		statusWebhookRequest			true	"Status webhook"
//	@Success		201		{object}	statuswebhooks.StatusWebhookDTO	"Created - secret is only ever shown here, at creation/update time"
//	@Failure		400		{object}	smsgateway.ErrorResponse		"Invalid request"
//	@Failure		401		{object}	smsgateway.ErrorResponse		"Unauthorized"
//	@Failure		403		{object}	smsgateway.ErrorResponse		"Forbidden"
//	@Failure		500		{object}	smsgateway.ErrorResponse		"Internal server error"
//	@Router			/3rdparty/v1/status-webhooks [post]
//
// Register status webhook.
func (h *ThirdPartyController) post(userID string, c *fiber.Ctx) error {
	req := new(statusWebhookRequest)
	if err := h.BodyParserValidator(c, req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	dto, err := h.statusWebhooksSvc.Replace(userID, req.ID, req.URL, req.Events)
	if err != nil {
		if statuswebhooks.IsValidationError(err) {
			return fiber.NewError(fiber.StatusBadRequest, err.Error())
		}
		return fmt.Errorf("failed to replace status webhook: %w", err)
	}

	return c.Status(fiber.StatusCreated).JSON(dto)
}

//	@Summary		Delete a status webhook
//	@Description	Deletes a status webhook
//	@Security		ApiAuth
//	@Security		JWTAuth
//	@Tags			User, Status Webhooks
//	@Param			id	path	string	true	"Webhook ID"
//	@Success		204	"Successfully removed"
//	@Failure		401	{object}	smsgateway.ErrorResponse	"Unauthorized"
//	@Failure		403	{object}	smsgateway.ErrorResponse	"Forbidden"
//	@Failure		500	{object}	smsgateway.ErrorResponse	"Internal server error"
//	@Router			/3rdparty/v1/status-webhooks/{id} [delete]
//
// Delete status webhook.
func (h *ThirdPartyController) delete(userID string, c *fiber.Ctx) error {
	id := c.Params("id")

	if err := h.statusWebhooksSvc.Delete(userID, id); err != nil {
		return fmt.Errorf("failed to delete status webhook: %w", err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (h *ThirdPartyController) Register(router fiber.Router) {
	router.Get("", permissions.RequireScope(ScopeList), userauth.WithUserID(h.get))
	router.Post("", permissions.RequireScope(ScopeWrite), userauth.WithUserID(h.post))
	router.Delete("/:id", permissions.RequireScope(ScopeDelete), userauth.WithUserID(h.delete))
}
