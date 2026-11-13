package publicpage

import (
	"context"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// Register preserves real product images and normalizes only public documents.
func (h *Handler) Register(routes gin.IRoutes, assets fs.FS) {
	register := func(path string, handler gin.HandlerFunc) {
		routes.GET(path, handler)
		routes.HEAD(path, handler)
		if path != "/" {
			redirect := func(c *gin.Context) {
				target := strings.TrimSuffix(c.Request.URL.EscapedPath(), "/")
				if c.Request.URL.RawQuery != "" {
					target += "?" + c.Request.URL.RawQuery
				}
				c.Redirect(http.StatusMovedPermanently, target)
			}
			routes.GET(path+"/", redirect)
			routes.HEAD(path+"/", redirect)
		}
	}
	if source, ok := h.source.(SitePages); ok {
		for path, load := range map[string]func(context.Context) (Page, error){
			"/": source.Home, "/about": source.About, "/products": source.Products,
			"/terms": source.Terms, "/privacy": source.Privacy,
		} {
			register(path, func(c *gin.Context) {
				h.serve(c, func(ctx context.Context, _ string) (Page, error) { return load(ctx) })
			})
		}
		for path, load := range map[string]func(context.Context, int, string) (Page, error){
			"/blog": source.BlogList, "/notice": source.NoticeList,
		} {
			register(path, func(c *gin.Context) {
				h.serve(c, func(ctx context.Context, _ string) (Page, error) {
					page := 1
					if raw := c.Query("page"); raw != "" {
						value, err := strconv.Atoi(raw)
						if err != nil || value < 1 || value > 100000 {
							return Page{}, ErrNotFound
						}
						page = value
					}
					search := strings.TrimSpace(c.Query("search"))
					if len([]rune(search)) > 200 {
						return Page{}, ErrNotFound
					}
					return load(ctx, page, search)
				})
			})
		}
	}
	for kind, load := range map[string]func(context.Context, string) (Page, error){
		"products": h.source.Product, "blog": h.source.Post, "categories": h.source.Category,
	} {
		register("/"+kind+"/:slug", func(c *gin.Context) {
			if assets != nil {
				if info, err := fs.Stat(assets, strings.TrimPrefix(c.Request.URL.Path, "/")); err == nil && !info.IsDir() {
					c.Header("Cache-Control", "no-cache")
					http.FileServer(http.FS(assets)).ServeHTTP(c.Writer, c.Request)
					return
				}
			}
			h.serve(c, load)
		})
	}
}
