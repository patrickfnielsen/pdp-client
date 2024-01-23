package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"log/slog"

	"github.com/patrickfnielsen/pdp-client/internal/config"
	"github.com/patrickfnielsen/pdp-client/internal/handlers"
	"github.com/patrickfnielsen/pdp-client/internal/util"
	"github.com/patrickfnielsen/pdp-client/pkg/pdp"
)

func main() {
	logger := util.SetupLogger(config.LogLevel, config.Enviroment)
	logger.Info("starting PDP",
		slog.Float64("version", config.VERSION),
		slog.String("environment", config.Enviroment),
		slog.String("log_level", config.LogLevel.String()),
		slog.String("pdp_log_server", config.PolicyLogServer),
		slog.String("pdp_log_endpoint", config.PolicyLogServerEndpoint),
		slog.Bool("pdp_log_tls", config.PolicyLogServerTLS),
		slog.Bool("pdp_log_http", config.PolicyServerLogHTTP),
		slog.Bool("pdp_log_console", config.PolicyServerLogConsole),
	)

	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	// setup the permit client
	permit, err := pdp.New(&pdp.PermitConfig{
		Logger: pdp.DecisionLogConfig{
			ConsoleLog:      config.PolicyServerLogConsole,
			HTTPLog:         config.PolicyServerLogHTTP,
			Endpoint:        util.FormatURL(config.PolicyLogServer, config.PolicyLogServerEndpoint, config.PolicyLogServerTLS),
			EndpointTimeout: 5,
			BearerToken:     config.PolicyLogServerToken,
		},
	})
	if err != nil {
		logger.Error("failed to start permit client", err)
		panic(err)
	}

	// setup policy updater
	updater := pdp.NewPolicyUpdater(
		config.PolicyRepository,
		config.PolicyRepositoryKey,
		config.PolicyRepositoryBranch,
		func(ctx context.Context, b []pdp.PolicyBundle) {
			for i := 0; i < len(b); i++ {
				slog.Info("policy update", slog.String("name", b[i].Name))

				err := permit.Activate(ctx, b[i].Name, string(b[i].Data))
				if err != nil {
					slog.Error("failed to activate policy", slog.String("error", err.Error()))
					return
				}
			}
		},
	)

	// sync the initial policies, and then start a periodic sync afterwards
	err = updater.RunUpdate(ctx)
	if err != nil {
		logger.Error("failed sync permissions", err)
		panic(err)
	}

	go updater.Start(ctx)

	// setup fiber + routes
	app := fiber.New(fiber.Config{
		ErrorHandler:          util.CustomErrorHandler,
		DisableStartupMessage: true,
		AppName:               "PDP",
		ServerHeader:          fmt.Sprintf("PDP - %f", config.VERSION),
	})
	app.Use(recover.New())

	// register pdp routes
	PdpRoutes := handlers.PdpRoutes{
		Permit: permit,
	}

	route := app.Group("/api/v1")
	route.Post("/pdp/decision", PdpRoutes.PdpCheck)

	// listen for system interrupts like ctrl+c
	quit := make(chan struct{})
	cleanup := func() {
		if ctx.Err() != nil {
			return
		}

		//shutdown down services gracefully
		logger.Info("service shutting down")
		err := permit.Close(ctx)
		err = errors.Join(app.Shutdown(), err)
		if err != nil {
			logger.Error("Service shutdown with errors", err)
		}

		// cancel the context and anything waiting for it
		cancelCtx()
		close(quit)
	}

	go util.MonitorSystemSignals(func(s os.Signal) {
		cleanup()
	})

	// start the app and handles errors
	err = app.Listen(":3000")
	if err != nil {
		logger.Error("service exited in a non-standard way", err)
		cleanup()
	}

	// wait for shutdown
	<-quit
}
