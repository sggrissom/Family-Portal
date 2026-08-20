package backend

import (
	"family/cfg"
	"net/http"
	"time"

	"go.hasen.dev/vbeam"
)

func RegisterSEOHandlers(app *vbeam.Application) {
	// Register robots.txt handler
	app.HandleFunc("/robots.txt", robotsHandler)

	// Register sitemap.xml handler
	app.HandleFunc("/sitemap.xml", sitemapHandler)

}

func robotsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Cache-Control", "public, max-age=86400") // Cache for 24 hours

	// Deny by default, allow the handful of pages that are genuinely public.
	//
	// The old version listed a few private paths and ended with "Allow: /",
	// which meant every route it had not thought of — /photos, /profile/3,
	// /family-timeline, /chat, /settings — was fair game. Enumerating what is
	// private is a losing game when the private set is "everything"; the safe
	// direction is the other one, so a route added next year is excluded by a
	// rule nobody has to remember to write.
	//
	// This is a request, not a control. Everything behind it is authenticated,
	// and the pages themselves carry noindex (frontend/lib/pageMetadata.ts).
	robotsContent := `User-agent: *
Disallow: /

Allow: /$
Allow: /login
Allow: /create-account
Allow: /forgot-password
Allow: /privacy
Allow: /terms
Allow: /support
Allow: /images/
Allow: /manifest.json

Sitemap: ` + cfg.SiteURL + `/sitemap.xml

# Be gentle: this is one small server.
Crawl-delay: 10`

	w.Write([]byte(robotsContent))
}

func sitemapHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("Cache-Control", "public, max-age=86400") // Cache for 24 hours

	sitemapContent := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url>
    <loc>` + cfg.SiteURL + `/</loc>
    <lastmod>` + time.Now().Format("2006-01-02") + `</lastmod>
    <changefreq>weekly</changefreq>
    <priority>1.0</priority>
  </url>
  <url>
    <loc>` + cfg.SiteURL + `/login</loc>
    <lastmod>` + time.Now().Format("2006-01-02") + `</lastmod>
    <changefreq>monthly</changefreq>
    <priority>0.5</priority>
  </url>
  <url>
    <loc>` + cfg.SiteURL + `/create-account</loc>
    <lastmod>` + time.Now().Format("2006-01-02") + `</lastmod>
    <changefreq>monthly</changefreq>
    <priority>0.5</priority>
  </url>
</urlset>`

	w.Write([]byte(sitemapContent))
}
