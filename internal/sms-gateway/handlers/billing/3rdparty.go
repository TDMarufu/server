package billing

import (
	"fmt"

	"github.com/android-sms-gateway/server/internal/sms-gateway/handlers/base"
	"github.com/android-sms-gateway/server/internal/sms-gateway/handlers/middlewares/userauth"
	"github.com/android-sms-gateway/server/internal/sms-gateway/modules/billing"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type thirdPartyControllerParams struct {
	fx.In

	BillingSvc *billing.Service

	Validator *validator.Validate
	Logger    *zap.Logger
}

type ThirdPartyController struct {
	base.Handler

	billingSvc *billing.Service
}

func NewThirdPartyController(params thirdPartyControllerParams) *ThirdPartyController {
	return &ThirdPartyController{
		Handler: base.Handler{
			Logger:    params.Logger,
			Validator: params.Validator,
		},
		billingSvc: params.BillingSvc,
	}
}

type statusResponse struct {
	Active           bool    `json:"active"`
	CurrentPeriodEnd *string `json:"currentPeriodEnd,omitempty"`
}

type subscribeResponse struct {
	RedirectURL string `json:"redirectUrl"`
}

type pesepayCallbackRequest struct {
	ReferenceNumber string `json:"referenceNumber"`
}

//	@Summary		Subscribe
//	@Description	Starts a new monthly subscription payment, returns a Pesepay checkout URL to redirect to
//	@Security		ApiAuth
//	@Security		JWTAuth
//	@Tags			User, Billing
//	@Produce		json
//	@Success		200	{object}	subscribeResponse
//	@Failure		401	{object}	smsgateway.ErrorResponse	"Unauthorized"
//	@Failure		500	{object}	smsgateway.ErrorResponse	"Internal server error"
//	@Router			/3rdparty/v1/billing/subscribe [post]
//
// Subscribe.
func (h *ThirdPartyController) subscribe(userID string, c *fiber.Ctx) error {
	redirectURL, err := h.billingSvc.Subscribe(c.Context(), userID)
	if err != nil {
		return fmt.Errorf("failed to start subscription: %w", err)
	}

	return c.JSON(subscribeResponse{RedirectURL: redirectURL})
}

//	@Summary		Subscription status
//	@Description	Returns whether the account's subscription is currently active
//	@Security		ApiAuth
//	@Security		JWTAuth
//	@Tags			User, Billing
//	@Produce		json
//	@Success		200	{object}	statusResponse
//	@Failure		401	{object}	smsgateway.ErrorResponse	"Unauthorized"
//	@Failure		500	{object}	smsgateway.ErrorResponse	"Internal server error"
//	@Router			/3rdparty/v1/billing/status [get]
//
// Subscription status.
func (h *ThirdPartyController) status(userID string, c *fiber.Ctx) error {
	sub, err := h.billingSvc.Status(c.Context(), userID)
	if err != nil {
		return fmt.Errorf("failed to get subscription status: %w", err)
	}

	if sub == nil {
		return c.JSON(statusResponse{Active: false})
	}

	active, err := h.billingSvc.IsActive(c.Context(), userID)
	if err != nil {
		return fmt.Errorf("failed to check subscription status: %w", err)
	}

	periodEnd := sub.CurrentPeriodEnd.Format("2006-01-02T15:04:05Z07:00")
	return c.JSON(statusResponse{Active: active, CurrentPeriodEnd: &periodEnd})
}

//	@Summary		Pesepay result callback
//	@Description	Receives Pesepay's payment-result notification. The body is not trusted directly - every call independently re-verifies via Pesepay's Check Payment API before crediting anything.
//	@Tags			Billing
//	@Accept			json
//	@Success		200	"Acknowledged"
//	@Failure		400	{object}	smsgateway.ErrorResponse	"Invalid request"
//	@Router			/3rdparty/v1/billing/pesepay/callback [post]
//
// Pesepay result callback.
func (h *ThirdPartyController) pesepayCallback(c *fiber.Ctx) error {
	req := new(pesepayCallbackRequest)
	if err := h.BodyParserValidator(c, req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	if req.ReferenceNumber == "" {
		return fiber.NewError(fiber.StatusBadRequest, "referenceNumber is required")
	}

	if err := h.billingSvc.HandleCallback(c.Context(), req.ReferenceNumber); err != nil {
		h.Logger.Error("failed to handle pesepay callback", zap.Error(err), zap.String("reference", req.ReferenceNumber))
		// Still acknowledge with 200: Pesepay does not retry failed deliveries
		// (per their docs), so a 5xx here just loses the notification outright.
		// The error is logged for reconciliation; HandleCallback is idempotent
		// and safe to miss once since status can also be polled separately.
	}

	return c.SendStatus(fiber.StatusOK)
}

// RegisterPublic registers the Pesepay callback route, which must be reachable
// without our own account authentication - Pesepay can't send Basic auth or a
// Bearer token. Must be mounted before the auth middleware .Use() call, exactly
// like the health handler.
func (h *ThirdPartyController) RegisterPublic(router fiber.Router) {
	router.Post("/billing/pesepay/callback", h.pesepayCallback)
}

// Register registers the authenticated billing endpoints.
func (h *ThirdPartyController) Register(router fiber.Router) {
	router.Post("/subscribe", userauth.WithUserID(h.subscribe))
	router.Get("/status", userauth.WithUserID(h.status))
}
