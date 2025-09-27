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

	robotsContent := `User-agent: *
Disallow: /admin/
Disallow: /static/
Disallow: /api/
Disallow: /dashboard
Disallow: /auth/
Allow: /

# Since this is a private family portal, we disallow indexing of sensitive areas
# but allow the home page for potential public information

Sitemap: ` + cfg.SiteURL + `/sitemap.xml

# Crawl delay to be respectful of server resources
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
