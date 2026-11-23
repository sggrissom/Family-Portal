# Family Portal sharing plan

Family Portal should let someone share the useful part of a screen while the
moment is fresh. Start with sending a new measurement and its comparisons to a
spouse, and sending a full competition recap to relatives who may have no
Family Portal account. Keep the public family summary and newsletter ideas as
separate publishing flows.

## Share from an existing screen

Put a **Share** action on the measurement detail or post-save view and on the
competition results page. Open a preview already filled in from the current
record. The sender can review what will go out, change what's included, and
share it through the system share sheet on iOS. On the web, use native file
sharing where supported and offer download, copy text, or copy link as
appropriate. Opening the preview creates no hosted page.

| From | Default format | Other options |
| --- | --- | --- |
| Measurement | One image card that fits in a message | Copy text with the same values and comparisons. If both people already have access, copy a private app link once the measurement has synced. |
| Competition results | One complete recap attachment. Use an image for a short recap and a paginated PDF for a longer one. | Copy the full results as text, or create a read-only recap link for recipients who want to open it in a browser. |

### Measurement card

- Show the person's display name, measurement, units, and age or measurement
  date as available. Preserve the source precision and any approximate age or
  date label.
- Carry over the comparisons shown for that measurement, in the same order,
  with enough context to make sense on their own. For example, show who or
  what each comparison is against, the comparison age and value when
  available, and any percentile label already shown in the app. Reuse the
  existing comparison calculations and formatting.
- Give the sender a way to omit individual comparison rows or all comparisons.
  Leave out birth dates, notes, and the rest of the growth history. If the
  screen has no comparison data, share the measurement on its own.
- The image and text use the data already available on the device. Show the
  preview immediately after a local save on iOS, even while sync is pending.
  A private app link should open the specific measurement under existing
  family access rules. Keep measurement cards out of account-free hosted
  sharing in this first release.

### Competition recap

- Share the whole competition results view as one item. Include the event name
  and date, then keep the page's routine order and result groupings. Include
  placements, adjudications, and judges' awards where recorded. Preserve
  whether a result applies to a routine or a particular dancer when the page
  shows that distinction.
- Start with all routines in the results view. Let the sender omit individual
  routines and optionally show participant names in an outside share. Leave
  out private notes and unrelated family records. If results are still being
  entered, allow sharing the current recap and label it as a snapshot so a
  later correction can be sent as a new recap.
- Size the image for legibility in a phone message. If it would take an
  oversized image or smaller type, use a single PDF with page breaks between
  routines when possible. Keep all included results in the attachment.
- Offer **Create recap link** as an explicit action for an account-free,
  mobile-friendly results page. Show the exact included results before
  publishing. Let the sender revoke the link or set an expiry. Edits to the
  underlying event leave the published recap as approved. The sender can
  publish a fresh recap after a correction.
- Render attachments from available local results on iOS, including pending
  edits. Hosted links require a connection and the approved results to be
  synced. While offline or waiting for sync, keep the attachment and copy-text
  actions available for a locally complete recap. If some results are
  unavailable, tell the sender in the preview and offer to share the
  available results explicitly.

### Rendering and recipient access

- Build each preview from the existing screen's data and ordering. Use the
  same selected fields for its supported formats so the text, file, and
  hosted recap match the preview. A lightweight share payload per screen is
  enough for these two uses.
- A private app link is a pointer to a record and uses existing authorization.
  An account-free recap link serves only an approved snapshot. Creating one
  requires permission to publish every included record. Family owners can
  revoke existing recap links, including ones created by another family
  member if publication is delegated later.
- Use a dedicated public read handler for hosted recaps. Redact tokens from
  logs, omit private record identifiers from responses, and apply noindex,
  no-referrer, and rate limits. Check revocation and expiry on every public
  page and asset request. Keep messaging previews generic by default.
- Explain in the preview that sent files, copied text, and recipients' saved
  copies can be forwarded. Revocation ends access through the hosted link.

### Implementation order and acceptance checks

1. Add the measurement card and competition recap attachments on iOS, with
   copy text for both. Confirm that a newly saved measurement and its visible
   comparisons go out in a single item while sync is pending. Share a short
   event as one readable image and a long event as one PDF, including its
   final routine and judges' awards.
2. Add the same share entry points and attachment/download behavior on the
   web. Add private deep linking to synced measurements using the existing
   family access checks. On both platforms, verify that omissions in the
   preview carry through to the shared output.
3. Add hosted competition recaps and link management. Verify that a recipient
   can open the complete approved recap from a phone browser without signing
   in, a source edit leaves the snapshot intact, and expired or revoked links
   fail on both page and asset requests. Reuse the public publishing
   infrastructure as the summary page and newsletters are implemented.

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
- Use a dedicated public handler and slug-to-page resolver. Existing
  procedures keep their authorization checks.
- Serve a published snapshot, not live data. Editing a person in the app should
  not silently change what strangers see, and unpublishing should be one delete.
- Serve approved photo derivatives through the public publishing path shared
  with Christmas-card updates. Require an active publication on every asset
  request.
- Rate limit by slug and only cache in ways that honor unpublishing.
- `robots.txt` currently disallows everything but the marketing pages. Default
  published pages to noindex and make indexing an explicit opt-in.

Decided (2026-11-22): the page lives at `/f/<slug>`, so app routes never
collide with a slug and the reserved list only blocks impersonation words. Each
person's profile photo is opt-in. Rate limits are per client, like recaps; a
per-slug limit would let one visitor lock a family's page for everyone.

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
- The share token grants access to the update. Use the same public link,
  revocation, and referrer controls as hosted competition recaps.
- Anything with a share link should be assumed cached by whatever it was sent
  through — link previews work by the platform fetching and storing the page and
  its cover image. Show the preview content to the sender and explain that
  messaging services may retain it after the link is revoked.
- Apply hard rate limits, length caps, plain text, and family-side moderation
  to reactions and comments.

Open question: whether the summary page and updates share one audience concept
or stay separate. Separate is simpler — a summary page is one public thing, an
update is many private links — and nothing yet needs them unified.
