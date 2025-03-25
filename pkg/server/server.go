package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"golang.org/x/crypto/acme/autocert"
)

type Server interface {
	Router() *fiber.App
	Run(ctx context.Context, port string) error
}

type server struct {
	app          *fiber.App
	listenConfig fiber.ListenConfig
}

func New(opts ...Option) (Server, error) {
	c := &config{
		healthcheckPath: "/healthz",
	}
	for _, opt := range opts {
		opt(c)
		if c.error != nil {
			return nil, c.error
		}
	}
	app := fiber.New()
	app.Get(c.healthcheckPath, func(c fiber.Ctx) error {
		return c.SendString("OK")
	})
	app.Use(cors.New()) // TODO: Add cors config
	listenConfig := fiber.ListenConfig{
		DisableStartupMessage: true,
	}
	if len(c.certificates) > 0 {
		listenConfig.AutoCertManager = &autocert.Manager{}
		tlsHandler := &fiber.TLSHandler{}
		listenConfig.TLSConfigFunc = func(tlsConfig *tls.Config) {
			tlsConfig.Certificates = c.certificates
			tlsConfig.GetCertificate = tlsHandler.GetClientInfo
			tlsConfig.NextProtos = nil
			if c.clientCAs != nil {
				tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
				tlsConfig.ClientCAs = c.clientCAs
			}
		}
		app.SetTLSHandler(tlsHandler)
	}
	return &server{
		app:          app,
		listenConfig: listenConfig,
	}, nil
}

func (s *server) Router() *fiber.App {
	return s.app
}

func (s *server) Run(ctx context.Context, port string) error {
	s.app.Hooks().OnListen(func(listenData fiber.ListenData) error {
		protocol := "http"
		if listenData.TLS {
			protocol = "https"
		}
		slog.InfoContext(ctx, "Running server", slog.String("url", fmt.Sprintf("%s://%s:%s", protocol, listenData.Host, listenData.Port)))
		return nil
	})
	s.app.Hooks().OnShutdown(func() error {
		slog.InfoContext(ctx, "Shutting down server")
		return nil
	})

	port = ":" + strings.TrimPrefix(port, ":")
	gracefulContext, cancel := signal.NotifyContext(ctx, os.Interrupt, os.Kill)
	defer cancel()
	s.listenConfig.GracefulContext = gracefulContext
	s.listenConfig.OnShutdownError = func(err error) {
		slog.ErrorContext(ctx, "Shutdown server error", slog.Any("error", err))
	}
	s.listenConfig.OnShutdownSuccess = func() {
		slog.InfoContext(ctx, "Shutdown server successfully")
	}
	return s.app.Listen(port, s.listenConfig)
}
