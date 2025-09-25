package backend

import (
	"net/http"
	"time"

	"go.hasen.dev/vbeam"
)

func RegisterSEOHandlers(app *vbeam.Application) {
	// Register robots.txt handler
	app.HandleFunc("/robots.txt", robotsHandler)

	// Register sitemap.xml handler
	app.HandleFunc("/sitemap.xml", sitemapHandler)

	// Register manifest.json handler
	app.HandleFunc("/manifest.json", manifestHandler)
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

Sitemap: /sitemap.xml

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
    <loc>/</loc>
    <lastmod>` + time.Now().Format("2006-01-02") + `</lastmod>
    <changefreq>weekly</changefreq>
    <priority>1.0</priority>
  </url>
  <url>
    <loc>/login</loc>
    <lastmod>` + time.Now().Format("2006-01-02") + `</lastmod>
    <changefreq>monthly</changefreq>
    <priority>0.5</priority>
  </url>
  <url>
    <loc>/create-account</loc>
    <lastmod>` + time.Now().Format("2006-01-02") + `</lastmod>
    <changefreq>monthly</changefreq>
    <priority>0.5</priority>
  </url>
</urlset>`

	w.Write([]byte(sitemapContent))
}

func manifestHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/manifest+json")
	w.Header().Set("Cache-Control", "public, max-age=86400") // Cache for 24 hours

	manifestContent := `{
  "name": "Family Portal - Track Your Family's Growth & Milestones",
  "short_name": "Family Portal",
  "description": "A private family portal to track children's growth, milestones, and precious memories",
  "start_url": "/",
  "display": "standalone",
  "background_color": "#f4f5f7",
  "theme_color": "#10b981",
  "orientation": "portrait-primary",
  "scope": "/",
  "categories": ["lifestyle", "utilities"],
  "icons": [
    {
      "src": "/images/favicon-16x16.png",
      "sizes": "16x16",
      "type": "image/png",
      "purpose": "any maskable"
    },
    {
      "src": "/images/favicon-32x32.png",
      "sizes": "32x32",
      "type": "image/png",
      "purpose": "any maskable"
    },
    {
      "src": "/images/apple-touch-icon.png",
      "sizes": "180x180",
      "type": "image/png",
      "purpose": "any maskable"
    },
    {
      "src": "/images/android-chrome-192x192.png",
      "sizes": "192x192",
      "type": "image/png",
      "purpose": "any maskable"
    },
    {
      "src": "/images/android-chrome-512x512.png",
      "sizes": "512x512",
      "type": "image/png",
      "purpose": "any maskable"
    }
  ],
  "shortcuts": [
    {
      "name": "Dashboard",
      "short_name": "Dashboard",
      "description": "View family dashboard",
      "url": "/dashboard",
      "icons": [
        {
          "src": "/images/favicon-32x32.png",
          "sizes": "32x32"
        }
      ]
    },
    {
      "name": "Add Photo",
      "short_name": "Add Photo",
      "description": "Add a new family photo",
      "url": "/add-photo",
      "icons": [
        {
          "src": "/images/favicon-32x32.png",
          "sizes": "32x32"
        }
      ]
    }
  ],
  "lang": "en-US",
  "dir": "ltr"
}`

	w.Write([]byte(manifestContent))
}

