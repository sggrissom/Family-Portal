package backend

import (
	"family/cfg"
	"net/http"
	"time"

	"go.hasen.dev/vbeam"
)

func RegisterSEOHandlers(app *vbeam.Application) {
	app.HandleFunc("/robots.txt", robotsHandler)

	app.HandleFunc("/sitemap.xml", sitemapHandler)
}

func robotsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Cache-Control", "public, max-age=86400")

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
	w.Header().Set("Cache-Control", "public, max-age=86400")

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
  <url>
    <loc>` + cfg.SiteURL + `/privacy</loc>
    <lastmod>` + time.Now().Format("2006-01-02") + `</lastmod>
    <changefreq>yearly</changefreq>
    <priority>0.3</priority>
  </url>
  <url>
    <loc>` + cfg.SiteURL + `/terms</loc>
    <lastmod>` + time.Now().Format("2006-01-02") + `</lastmod>
    <changefreq>yearly</changefreq>
    <priority>0.3</priority>
  </url>
  <url>
    <loc>` + cfg.SiteURL + `/support</loc>
    <lastmod>` + time.Now().Format("2006-01-02") + `</lastmod>
    <changefreq>yearly</changefreq>
    <priority>0.3</priority>
  </url>
</urlset>`

	w.Write([]byte(sitemapContent))
}
