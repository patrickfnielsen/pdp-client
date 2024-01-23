package handlers

import (
	"log/slog"

	"github.com/gofiber/fiber/v2"
	"github.com/patrickfnielsen/pdp-client/internal/models"
	"github.com/patrickfnielsen/pdp-client/internal/util"
	"github.com/patrickfnielsen/pdp-client/pkg/pdp"
)

type PdpRoutes struct {
	Permit *pdp.PermitClient
}

func (r *PdpRoutes) PdpCheck(c *fiber.Ctx) error {
	req, valErrs := util.ReadAndValidate[pdp.DecisionRequest](c)
	if valErrs != nil {
		return c.Status(fiber.StatusBadRequest).JSON(valErrs)
	}

	// verify that policies have been loaded
	if !r.Permit.Ready() {
		return fiber.NewError(fiber.StatusServiceUnavailable, "PDP not ready: no policies loaded")
	}

	// make a permit decision
	decision, err := r.Permit.Decision(c.UserContext(), pdp.DecisionOptions{
		RemoteAddr: c.IP(),
		Path:       req.Path,
		Input:      req,
	})
	if err != nil {
		slog.Error("decision error", slog.String("error", err.Error()))
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}

	return c.JSON(models.DecisionResponse{DecisionID: decision.ID, Result: decision.Result})
}
