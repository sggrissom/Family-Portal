# Sharing outside the family

Two ways to hand part of the record to people who aren't in the app. Both are
new read paths with no session behind them, and they share the public photo
derivative path, so they're planned together.

## Public family summary page

One public page per family at a URL the family picks, meant to be handed to
someone you've just met: here's my family, here's who's in it, here's a couple
of pictures. Not a profile and not a feed — a single page that stays roughly the
same for years.

Scope:
- A family-chosen slug, unique across the install and changeable, with a
  reserved-word list so nobody claims `login` or `admin`.
- Off by default. Publishing is an explicit action by a family owner, and
  unpublishing takes the page down immediately.
- Per-person opt-in: which members appear, what name is shown (a first name or
  nickname, not necessarily the name used inside the app), and whether an age
  shows at all. Full birth dates for kids never belong on a public page — the
  default is nothing, and the most it should ever offer is an age in years.
- A short intro paragraph plus an optional one-liner per person.
- A handful of photos chosen from the existing library.
- Nothing else. No growth data, no milestones, no chat, no timeline. The page
  renders only what was explicitly marked public; it is not a filtered view of
  the private app.

Technical notes:
- This is the first read path in the app with no session behind it. It gets its
  own handler and its own slug-to-page resolver rather than relaxing auth on any
  existing proc.
- Serve a published snapshot, not live data. Editing a person in the app should
  not silently change what strangers see, and unpublishing should be one delete.
- Photos need a public derivative path with unguessable filenames, separate from
  the authenticated photo endpoints. The Christmas-card updates below need the
  same thing, so build it once.
- Rate limit and cache by slug. It is the one URL in the app anyone can hammer.
- `robots.txt` currently disallows everything but the marketing pages. Default
  published pages to noindex and make indexing an explicit opt-in.

Open question: a chosen slug is memorable but guessable and enumerable, while a
random token is neither guessable nor speakable. Slug plus noindex is probably
the right trade for a page whose whole purpose is being handed to a person.

## Christmas-card updates

Periodic "here's what we've been up to" posts — some text, a few photos, a
recap of a vacation — shared with specific people rather than the world. The
model is a Christmas card or a family newsletter, not a timeline.

Scope:
- An update: title, date, body text, photos from the library, and optionally the
  people it's about.
- Draft and publish states. Drafts are visible only to the family.
- A share link per update: unguessable token, revocable, optionally expiring.
- Built for link previews. Open Graph and Twitter card tags, a chosen cover
  image at the right dimensions, and a title and description that read well in a
  Facebook, iMessage, or WhatsApp preview. The preview is what recipients
  actually see, so it is the feature, not a detail.
- Lite social elements, deliberately bounded:
  - Reactions from a small fixed set, attributed if the viewer is a known
    recipient and anonymous otherwise.
  - Comments — name and plain text — that the family can delete, and can turn
    off per update.
  - No follower graph, no feed of other families, no notifications to anyone
    outside the family, nothing algorithmic.
- A per-family archive of past updates, optionally linked from the summary page
  above.
- Notify family members on publish through the existing mail and push workers,
  plus an optional list of outside email addresses — sending it to Grandma is
  the actual use case.

Technical notes:
- The share token is the entire authorization story, so it goes in the path
  rather than a query string that leaks through referrers, and the page sets
  `Referrer-Policy: no-referrer` and noindex.
- Anything with a share link should be assumed cached by whatever it was sent
  through — link previews work by the platform fetching and storing the page and
  its cover image. That is inherent to the feature and the first time this app
  hands content to a third party at all, so say it plainly in the UI when a
  share link is created.
- Reactions and comments are the first writes from unauthenticated visitors:
  hard rate limits, length caps, no HTML, and family-side moderation.

Open question: whether the summary page and updates share one audience concept
or stay separate. Separate is simpler — a summary page is one public thing, an
update is many private links — and nothing yet needs them unified.
