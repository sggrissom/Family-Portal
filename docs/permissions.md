# Permissions

Two mechanisms decide who can see what, and they are not variations on each
other. **Membership** puts a user inside a household and gives them everything
in it. A **family link** lets one household show a named person's records to
another household, read-only, one scope at a time. Everything below follows
from that split.

The whole model lives in `backend/access.go` and `backend/family_link.go`.
`backend/family_link_test.go` is the executable specification — every claim in
this document has a test behind it.

---

## 1. Access levels

```go
AccessNone       // 0
AccessView       // 1
AccessContribute // 2
AccessAdmin      // 3
```

They cross the wire as those integers, not as strings. Membership in a family
grants **admin**. A link grants **view** and can never grant more — the ceiling
is a constant:

```go
const MaxLinkAccess = AccessView
```

`clampLinkAccess` pins the stored value into `[View, View]` on write, and
`linkGrants` clamps again on read, so a link row hand-edited to `Access: 3`
still yields view. `TestLinkNeverGrantsWrites` sets exactly that trap.

---

## 2. Membership — your own household

A user has one **primary** family (`User.FamilyId`) plus a row in
`FamilyMembership` for every family they belong to. Registering with someone's
invite code joins that family instead of creating one; either way the new user
gets `AccessAdmin` in it (`AddUserTx`, `backend/users.go`).

`familiesVisibleTo` is the list of families a user is a *member* of — primary
first, the rest sorted. Membership is the only thing that lands in that list;
links never do.

Inside a family, membership is total. Everyone with a membership row sees every
person, measurement, milestone, photo, activity, and chat message in it, and at
admin can write and delete all of it. There is no per-record sharing and no
read-only role for household members — the `Role` field on the membership row
supports one, and `min(membership.Role, …)` in `CanAccessFamily` honours it, but
nothing currently issues a membership below admin.

Effective access to a family:

```go
CanAccessFamily(tx, user, familyId, need)
  → any membership where min(role, familyGrants(membershipFamily, familyId)) >= need
```

and `familyGrants` is deliberately blunt: same family → admin, anything else →
none. **A link never widens `CanAccessFamily`.** That is why linked households
can't reach whole-family surfaces.

---

## 3. Family links — the extended family

A `FamilyLink` is a row saying "family *From* shows things to family *To*":

| Field | Meaning |
| --- | --- |
| `FromFamilyId` | the household that owns the data and is doing the sharing |
| `ToFamilyId` | the household that gets to read |
| `Kind` | a free-text label, e.g. `"grandparents"` — cosmetic |
| `Access` | always `AccessView` (clamped) |
| `Scopes` | bitmask of `People / Milestones / Photos / Growth / Activities` |
| `Status` | `Pending` → `Accepted` → `Revoked` |

### It takes two keys, not one

A link by itself shares **nothing**. Access to a person requires both:

1. an **accepted link** from that person's home family to a family you're a
   member of, carrying the scope you're asking for; and
2. a **roster row** (`PersonFamily`) putting that specific person on your
   family's roster.

`canAccessPersonViaLink` checks both, then takes the minimum of your role in
the viewing family and what the link grants:

```go
granted := min(userRoleIn(tx, user, familyId), linkGrants(tx, familyId, person.FamilyId, scope))
```

So the link is the *channel* and the roster row is the *payload*. The owning
family opens the channel once, then decides person by person who travels down
it (`SharePersonWithFamily`, which refuses unless `CanShareIntoFamily` finds an
accepted link carrying the `People` scope).

### Direction matters

Links are one-way. `linkGrants` reads `GetLinksToFamily(actingFamilyId)` and
only counts rows whose `FromFamilyId` is the owner. An A→B link lets B read A's
shared people; it gives A nothing in B. Two-way sharing is two link rows.
`TestLinkAccessIsAsymmetric`.

### No transitivity

A→B and B→C does not give C anything of A's. The loop in
`canAccessPersonViaLink` only ever considers direct links into families the
user is a member of, and `CanShareIntoFamily` refuses to forward someone else's
person onward. `TestNoTransitiveAccessLeak`.

### Pending and revoked grant nothing

`linkGrants` requires `Status == LinkAccepted`. Revoking also calls
`unshareAllThroughLinkTx`, which deletes every roster row the link produced — so
a revoked link leaves no residue. Dropping the `People` scope in
`UpdateFamilyLink` does the same. `TestPendingAndRevokedLinksGrantNothing`,
`TestRevokingALinkUnsharesItsPeople`.

### Scopes are independent

`normalizeLinkScopes` forces `People` on whenever any other scope is set —
there is nothing to share records *about* otherwise. Beyond that they're
independent bits, checked at each call site with the scope that matches the
record type:

| Scope | Gates | Checked in |
| --- | --- | --- |
| `People` | the person record itself | `person.go`, `family_link_procs.go` |
| `Milestones` | milestones and their tags | `milestone.go:232`, `:246` |
| `Photos` | photos the person is tagged in | `photos.go:308`, `CanAccessPhoto` |
| `Growth` | height/weight history | `growth.go:133` |
| `Activities` | activity entries the person appears in | `activity.go:659` |

`ScopeFamily` (0) is the sentinel for "not shareable" — `linkGrants` and
`canAccessPersonViaLink` both return early on it. Passing it means the caller
is asking for whole-family access, which no link can give.

---

## 4. Worked example: the mother-in-law

Take the case directly. You and your wife are in **your family**. Your
mother-in-law has her own account and therefore her own family. Your wife is a
person in your family and a daughter in hers.

First, a distinction that decides everything:

- Your **wife the user** is a member of your family. If she also wanted full
  access to her mother's household she'd need a *membership* there — a separate
  thing, granting admin, entirely outside the link system.
- Your **wife the person record** is a row owned by your family. That row is
  what a link can share.

To connect the households:

1. Your MIL gives you her family invite code (`Settings → Connected Families`).
2. Someone with admin in your family calls `CreateFamilyLink` with that code and
   ticks the scopes — say People, Milestones, Photos. This writes
   `From: yours, To: hers, Status: Pending`.
3. Your MIL accepts (`AcceptFamilyLink`, admin in the receiving family).
4. You then share specific people: `SharePersonWithFamily` for your wife, and
   for each grandchild you want her to see. Each call adds a `PersonFamily` row
   putting that person on *her* family's roster, optionally with a
   `Relationship` label she'll see them under.

Now, concretely, your MIL sees:

| She sees | She does not see |
| --- | --- |
| The people you explicitly shared, on her roster alongside her own | Anyone in your family you didn't share — `TestLinkGrantsOnlyWhatItCarries` |
| Their milestones, photos, measurements, activities — but only the scopes on the link | Anything in a scope the link doesn't carry, even for a shared person |
| Photos those people are **tagged in** | Photos with nobody shared tagged in them, including untagged ones |
| Your family's tags, so shared milestones and photos render with real labels (`tags.go:97`) | Your family chat — every chat path uses `CanAccessFamily`, which links never satisfy |
| | Your family's member list, invite code, settings, import/export |
| | Anything of yours in the app's write paths — all of it is view-only |

And she can never write. Not a comment, not a measurement, not a tag on a photo
of her own granddaughter. Every mutating proc asks for `AccessContribute` or
`AccessAdmin`, and the link ceiling stops at view.

Note that this is all one-directional. Her accepting your link gives you nothing
in her family. If you want to see her side too, she creates a second link
pointing the other way.

### Two edges worth knowing

**Shared people appear on the host roster.** `GetFamilyPeople` is roster-based,
so your wife shows up in your MIL's people list — under the role and
relationship *her* roster row carries, not yours. But `GetFamilyOwnPeople` is
keyed on `Person.FamilyId`, so shared people are correctly excluded from her
export and from import de-duplication (`TestOwnPeopleExcludesSharedIn`). Her
export contains her data, not yours.

**A shared photo is shared whole.** `CanAccessPhoto` grants the photo if *any*
tagged person is reachable, and `GetPhoto` then returns every `PhotoPerson` row
on it. So a photo tagged with your shared daughter *and* an unshared sibling
becomes visible with the sibling's name and birthday attached. The sibling's
own records stay locked; only the tag on that photo leaks. If that matters,
share the sibling deliberately or don't tag them.

---

## 5. Resolving the acting family

Procs that create or list data take an optional `familyId`. `ResolveActingFamily`
handles it uniformly:

- `0` or absent means the caller's primary family.
- Non-zero is checked with `RequireFamilyAccess` at the level the proc needs.
  A family the caller can't reach answers the same as one that doesn't exist.
- Procs naming an existing record don't take a `familyId` at all — the record
  names its own family (`ActingFamilyFor`, `ActingFamilyForPerson`).

Because writes resolve through `RequireFamilyAccess` and not through the link
path, a linked family can never be chosen as an acting family for a write.

---

## 6. Where it's enforced

| Surface | Entry point | Link-aware |
| --- | --- | --- |
| Person detail / list | `CanAccessPerson`, `GetVisiblePeople` | yes, per scope |
| Growth | `GetGrowthDataForUser` (`growth.go:133`) | yes |
| Milestones + search | `GetMilestoneForUser`, `SearchVisibleMilestones` | yes |
| Photos list + detail + serve | `GetVisibleImages`, `CanAccessPhoto` | yes |
| Activities | `canAccessEntry` (`activity.go:659`) | yes |
| Tags | `getVisibleTags` via `sharedInFamilies` | yes |
| Chat (REST + WebSocket) | `CanAccessFamily`, `CanFamilyAccess` | **no — household only** |
| Members, invite code, links | `ResolveActingFamily(…, AccessAdmin)` | no |
| Export / import | `GetFamilyOwnPeople` | no — own people only |

Chat being household-only is a design decision, not an oversight: the link
model shares *people*, and a message belongs to a family rather than to any
person.
