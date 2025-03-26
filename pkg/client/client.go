package client

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
	fclient "github.com/gofiber/fiber/v3/client"
)

type Client interface {
	Get(ctx context.Context, path string) (*Response, error)
	Post(ctx context.Context, path string, body any) (*Response, error)
	Put(ctx context.Context, path string, body any) (*Response, error)
	Patch(ctx context.Context, path string, body any) (*Response, error)
	Delete(ctx context.Context, path string) (*Response, error)
}

type client struct {
	app *fclient.Client
}

type Response struct {
	Body       []byte
	StatusCode int
}

type contextKey string

var requestTimeKey contextKey = "request_time"

func New(baseUrl string, opts ...Option) (Client, error) {
	c := &config{}
	for _, opt := range opts {
		opt(c)
		if c.error != nil {
			return nil, c.error
		}
	}
	app := fclient.New()
	app.SetBaseURL(baseUrl)
	if len(c.certificates) > 0 {
		app.SetTLSConfig(&tls.Config{
			Certificates: c.certificates,
		})
	}
	if len(c.headers) > 0 {
		app.SetHeaders(c.headers)
	}
	if c.proxy != "" {
		app.SetProxyURL(c.proxy)
	}
	app.AddRequestHook(func(c *fclient.Client, req *fclient.Request) error {
		req.SetContext(context.WithValue(req.Context(), requestTimeKey, time.Now()))
		return nil
	})
	app.AddResponseHook(func(_ *fclient.Client, res *fclient.Response, req *fclient.Request) error {
		if !c.requestLog {
			return nil
		}
		ctx := req.Context()
		latency := time.Since(ctx.Value(requestTimeKey).(time.Time))
		slog.InfoContext(ctx, "request",
			slog.String("request_url", req.RawRequest.URI().String()),
			slog.String("request_method", req.Method()),
			slog.String("request_body", string(req.RawRequest.Body())),
			slog.String("response_body", string(res.Body())),
			slog.Int("response_status", res.StatusCode()),
			slog.Int64("latency_ms", latency.Milliseconds()),
		)
		return nil
	})
	return &client{
		app: app,
	}, nil
}

func (c *client) request(ctx context.Context, body any) *fclient.Request {
	req := c.app.R()
	req.SetContext(ctx)
	if body != nil {
		var b []byte
		switch body := body.(type) {
		case []byte:
			b = body
		default:
			var err error
			b, err = json.Marshal(body)
			if err != nil {
				b = []byte(fmt.Sprint(body))
			}
		}
		req.SetRawBody(b)
	}
	return req
}

func (c *client) response(req *fclient.Request, method, path string) (*Response, error) {
	res, err := req.SetMethod(method).SetURL(path).Send()
	if err != nil {
		return nil, err
	}
	defer res.Close()
	return &Response{
		Body:       res.Body(),
		StatusCode: res.StatusCode(),
	}, nil
}

func (c *client) Get(ctx context.Context, path string) (*Response, error) {
	return c.response(c.request(ctx, nil), fiber.MethodGet, path)
}

func (c *client) Post(ctx context.Context, path string, body any) (*Response, error) {
	return c.response(c.request(ctx, body), fiber.MethodPost, path)
}

func (c *client) Put(ctx context.Context, path string, body any) (*Response, error) {
	return c.response(c.request(ctx, body), fiber.MethodPut, path)
}

func (c *client) Patch(ctx context.Context, path string, body any) (*Response, error) {
	return c.response(c.request(ctx, body), fiber.MethodPatch, path)
}

func (c *client) Delete(ctx context.Context, path string) (*Response, error) {
	return c.response(c.request(ctx, nil), fiber.MethodDelete, path)
}
