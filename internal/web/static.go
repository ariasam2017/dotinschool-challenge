package web

import (
	"bytes"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"

	frontend "github.com/dotinschool/dotinschool/web"
)

// headToken is replaced in index.html at serve time with mode/base meta tags.
// It must appear verbatim in the built index.html <head>.
const headToken = "<!--DOTIN_HEAD-->"

// mountStatic serves the embedded single-page app. The same index.html backs
// three entry points, distinguished by injected <meta> tags:
//
//   - the user panel at "/"
//   - the admin panel under the secret ADMIN_PATH
//   - the admin panel at "/" on the dedicated admin sub-domain (when set)
//
// Hashed build assets under /assets/ are served straight from the embed FS with
// a long immutable cache; index.html is never cached.
func (s *server) mountStatic(r *gin.Engine) {
	assetsFS := frontend.Assets()

	rawIndex, err := fs.ReadFile(assetsFS, "index.html")
	if err != nil {
		s.log.WithError(err).Fatal("embedded frontend is missing index.html — build web/ first")
	}
	fileSrv := http.FileServer(http.FS(assetsFS))

	serveIndex := func(c *gin.Context, admin bool, base string) {
		var meta string
		if admin {
			meta = `<meta name="dotin-mode" content="admin"><meta name="dotin-base" content="` + htmlAttr(base) + `">`
		} else {
			meta = `<meta name="dotin-mode" content="user"><meta name="dotin-base" content="/">`
		}
		body := bytes.Replace(rawIndex, []byte(headToken), []byte(meta), 1)
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "text/html; charset=utf-8", body)
	}

	assetExists := func(p string) bool {
		p = strings.TrimPrefix(p, "/")
		if p == "" {
			return false
		}
		f, err := assetsFS.Open(p)
		if err != nil {
			return false
		}
		defer f.Close()
		if st, err := f.Stat(); err == nil && !st.IsDir() {
			return true
		}
		return false
	}

	r.NoRoute(func(c *gin.Context) {
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.Status(http.StatusNotFound)
			return
		}
		p := path.Clean(c.Request.URL.Path)
		if p == "." || p == "" {
			p = "/"
		}

		onAdminHost := s.onAdminHost(c)

		// Dedicated admin sub-domain: "/" (and any non-asset path) is the admin
		// panel; hashed assets still come from the embed FS.
		if onAdminHost {
			if p != "/" && assetExists(p) {
				setAssetCache(c, p)
				fileSrv.ServeHTTP(c.Writer, c.Request)
				return
			}
			serveIndex(c, true, "/")
			return
		}

		// Real build asset (hashed js/css, fonts, favicon, ...).
		if p != "/" && assetExists(p) {
			setAssetCache(c, p)
			fileSrv.ServeHTTP(c.Writer, c.Request)
			return
		}

		// Secret admin path.
		if p == s.cfg.AdminPath || strings.HasPrefix(p, s.cfg.AdminPath+"/") {
			serveIndex(c, true, s.cfg.AdminPath+"/")
			return
		}

		// A guess at the conventional /admin URL when that is NOT the secret
		// path is rejected outright — the admin panel is only reachable via
		// ADMIN_PATH (or the sub-domain).
		if s.cfg.AdminPath != "/admin" && (p == "/admin" || strings.HasPrefix(p, "/admin/")) {
			c.String(http.StatusNotFound, "یافت نشد")
			return
		}

		// Everything else is the user panel; the client router handles unknown
		// sub-paths.
		serveIndex(c, false, "/")
	})
}

// onAdminHost reports whether the request Host matches the configured admin
// sub-domain (port stripped, case-insensitive).
func (s *server) onAdminHost(c *gin.Context) bool {
	if s.cfg.AdminSubdomain == "" {
		return false
	}
	host := c.Request.Host
	if i := strings.LastIndexByte(host, ':'); i >= 0 {
		host = host[:i]
	}
	return strings.EqualFold(host, s.cfg.AdminSubdomain)
}

// setAssetCache marks hashed build output immutable and other root files short.
func setAssetCache(c *gin.Context, p string) {
	if strings.HasPrefix(p, "/assets/") {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
		return
	}
	c.Header("Cache-Control", "public, max-age=3600")
}

func htmlAttr(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	return s
}
