package routes

import (
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/swaggo/swag"

	_ "filebox/docs" // registers swagger spec
	"filebox/internal/config"
	"filebox/internal/handler"
	"filebox/internal/middleware"
	"filebox/internal/service"
	"filebox/internal/utils"
	"filebox/internal/web"
)

// Deps bundles the handlers and config routes need.
type Deps struct {
	Config        *config.Config
	JWT           *utils.JWTManager
	RateLimitMax  int
	AuthHandler   *handler.AuthHandler
	ItemHandler   *handler.ItemHandler
	ChunkHandler  *handler.ChunkHandler
	FriendHandler *handler.FriendHandler
	APIKeyHandler *handler.APIKeyHandler
	APIKeyService *service.APIKeyService
	AdminHandler  *handler.AdminHandler
}

// Register wires global middleware, API routes, Swagger and the SPA fallback.
func Register(app *fiber.App, deps Deps) {
	// 1. Register global middlewares
	app.Use(recover.New())
	app.Use(middleware.Logger())
	app.Use(compress.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization, X-Requested-With",
		AllowMethods: "GET, POST, PUT, DELETE, OPTIONS",
	}))
	if deps.RateLimitMax > 0 {
		app.Use(limiter.New(limiter.Config{
			Max:        deps.RateLimitMax,
			Expiration: time.Second,
			Next: func(c *fiber.Ctx) bool {
				// Only rate-limit API calls; static assets, swagger and health are excluded.
				return !strings.HasPrefix(c.Path(), "/api")
			},
			LimitReached: func(c *fiber.Ctx) error {
				return utils.Error(c, fiber.StatusTooManyRequests, "rate limit exceeded, please slow down")
			},
		}))
	}

	// 2. Health check + Swagger (custom handler, loads Swagger UI from CDN and spec from /swagger/doc.json)
	app.Get("/health", health)
	app.Get("/swagger", func(c *fiber.Ctx) error { return c.Redirect("/swagger/index.html") })
	app.Get("/swagger/index.html", swaggerIndex)
	app.Get("/swagger/doc.json", swaggerDoc)

	// 3. Public auth routes
	api := app.Group("/api/v1")
	auth := api.Group("/auth")
	auth.Post("/register", deps.AuthHandler.Register)
	auth.Post("/login", deps.AuthHandler.Login)
	auth.Post("/refresh", deps.AuthHandler.Refresh)
	auth.Post("/logout", deps.AuthHandler.Logout)

	// 4. Protected routes (require a valid access token or API key)
	protected := api.Group("", middleware.Auth(deps.JWT, deps.APIKeyService))
	protected.Get("/profile", deps.AuthHandler.Profile)

	// 5. Unified item routes (files + folders)
	items := protected.Group("/item")
	items.Get("/list", deps.ItemHandler.List)
	items.Get("/detail", deps.ItemHandler.Detail)
	items.Post("/folder", deps.ItemHandler.CreateFolder)
	items.Post("/upload", deps.ItemHandler.Upload)
	items.Put("/update", deps.ItemHandler.Update)
	items.Delete("/delete", deps.ItemHandler.Delete)
	items.Get("/download", deps.ItemHandler.Download)
	items.Get("/preview", deps.ItemHandler.Preview)
	items.Get("/search", deps.ItemHandler.Search)
	items.Get("/shared", deps.ItemHandler.Shared)
	items.Get("/favorites", deps.ItemHandler.Favorites)

	// 6. User key routes (disabled: server-side disk encryption removes the need
	// for per-user RSA keys and the post-login encryption key modal).

	// 7. Friend routes
	friends := protected.Group("/friends")
	friends.Get("/", deps.FriendHandler.List)
	friends.Post("/", deps.FriendHandler.Create)
	friends.Delete("/:id", deps.FriendHandler.Delete)
	friends.Get("/requests", deps.FriendHandler.ListRequests)
	friends.Post("/requests/:id/accept", deps.FriendHandler.AcceptRequest)
	friends.Post("/requests/:id/reject", deps.FriendHandler.RejectRequest)
	friends.Delete("/requests/:id", deps.FriendHandler.CancelRequest)

	// 8. Chunked upload routes (functional but mostly hidden from Swagger)
	protected.Get("/config/upload", deps.ChunkHandler.UploadConfig)
	protected.Post("/upload/chunk/init", deps.ChunkHandler.InitChunk)
	protected.Post("/upload/chunk/:id", deps.ChunkHandler.UploadChunk)
	protected.Get("/upload/chunk/:id/status", deps.ChunkHandler.ChunkStatus)
	protected.Post("/upload/chunk/:id/complete", deps.ChunkHandler.CompleteChunk)

	// 9. API keys
	apiKeys := protected.Group("/api-keys")
	apiKeys.Get("/", deps.APIKeyHandler.List)
	apiKeys.Post("/", deps.APIKeyHandler.Create)
	apiKeys.Post("/:id/reveal", deps.APIKeyHandler.Reveal)
	apiKeys.Delete("/:id", deps.APIKeyHandler.Revoke)

	// 10. Optional admin endpoints (gated by HAS_ADMIN_PAGE)
	if deps.Config.HasAdminPage {
		admin := protected.Group("/admin")
		admin.Get("/config", deps.AdminHandler.Config)
	}

	// 7. Prevent browsers from caching embedded static assets during development.
	// This ensures frontend changes are visible immediately after rebuilding.
	app.Use("/", func(c *fiber.Ctx) error {
		p := c.Path()
		if !strings.HasPrefix(p, "/api") && !strings.HasPrefix(p, "/swagger") && p != "/health" {
			c.Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
			c.Set("Pragma", "no-cache")
			c.Set("Expires", "0")
		}
		return c.Next()
	})

	// 8. Serve embedded vanilla JS SPA (everything not matched above). API,
	// Swagger and health paths are skipped so they reach their handlers.
	app.Use("/", filesystem.New(filesystem.Config{
		Root:         http.FS(web.StaticFS),
		Index:        "index.html",
		NotFoundFile: "index.html", // SPA fallback for client-side routes
		Next: func(c *fiber.Ctx) bool {
			p := c.Path()
			return strings.HasPrefix(p, "/api") || strings.HasPrefix(p, "/swagger") || p == "/health"
		},
	}))
}

// swaggerIndex serves a Swagger UI page that fetches the spec from /swagger/doc.json.
func swaggerIndex(c *fiber.Ctx) error {
	c.Set("Content-Type", "text/html; charset=utf-8")
	return c.SendString(swaggerIndexHTML)
}

// swaggerDoc returns the generated OpenAPI/Swagger spec.
func swaggerDoc(c *fiber.Ctx) error {
	doc, err := swag.ReadDoc("swagger")
	if err != nil {
		return utils.Error(c, fiber.StatusInternalServerError, "failed to read swagger spec")
	}
	c.Set("Content-Type", "application/json; charset=utf-8")
	return c.SendString(doc)
}

const swaggerIndexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>FileBox API - Swagger UI</title>
  <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
  <style>
    html { box-sizing: border-box; overflow: -moz-scrollbars-vertical; overflow-y: scroll; }
    *, *:before, *:after { box-sizing: inherit; }
    body { margin: 0; background: #fafafa; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js" crossorigin></script>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-standalone-preset.js" crossorigin></script>
  <script>
    window.onload = function () {
      window.ui = SwaggerUIBundle({
        url: "/swagger/doc.json",
        dom_id: "#swagger-ui",
        deepLinking: true,
        presets: [SwaggerUIBundle.presets.apis, SwaggerUIStandalonePreset],
        plugins: [SwaggerUIBundle.plugins.DownloadUrl],
        layout: "StandaloneLayout"
      });
    };
  </script>
</body>
</html>`

// health is a simple liveness probe.
func health(c *fiber.Ctx) error {
	return utils.JSON(c, "filebox is running", fiber.Map{"status": "ok"})
}
