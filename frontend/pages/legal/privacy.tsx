import * as preact from "preact";
import * as rpc from "vlens/rpc";
import { Header, Footer } from "../../layout";
import { LegalNav } from "./legal-nav";
import "./legal-styles";

type Data = {};

export async function fetch(route: string, prefix: string) {
  return rpc.ok<Data>({});
}

export function view(route: string, prefix: string, data: Data): preact.ComponentChild {
  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="legal-container">
        <PrivacyPage />
      </main>
      <Footer />
    </div>
  );
}

const PrivacyPage = () => (
  <article className="legal-page">
    <h1>Privacy</h1>
    <p className="legal-updated">Last updated 19 August 2026</p>

    <p className="legal-lead">
      Family Record holds photographs and health measurements of children. This page says exactly
      what it stores, what leaves the server, and how to get rid of any of it.
    </p>

    <h2>Who runs this</h2>
    <p>
      Family Record is run by one person, not a company. There is no advertising, no analytics
      product, no data broker, and nothing in the database is sold, rented, or shared with anyone
      outside your family. Contact details are on the <a href="/support">support page</a>.
    </p>

    <h2>What is stored</h2>
    <div className="legal-table-wrap">
      <table className="legal-table">
        <thead>
          <tr>
            <th scope="col">Category</th>
            <th scope="col">What it is</th>
          </tr>
        </thead>
        <tbody>
          <tr>
            <td>Account</td>
            <td>
              Your name, email address, a bcrypt hash of your password (never the password), account
              creation and last login times, and which family you belong to.
            </td>
          </tr>
          <tr>
            <td>People</td>
            <td>
              A record for each family member you add: name, birth date, gender, and whether they
              are a parent or a child.
            </td>
          </tr>
          <tr>
            <td>Growth</td>
            <td>Height and weight measurements with the date each was taken.</td>
          </tr>
          <tr>
            <td>Milestones and activities</td>
            <td>
              Dated events with a description and category; sports and hobby seasons, competitions,
              routines, and results.
            </td>
          </tr>
          <tr>
            <td>Photos</td>
            <td>
              The original file you uploaded, resized copies generated from it, any caption and
              tags, the date, and which people are in it. Dates and orientation are read from the
              image's EXIF metadata.
            </td>
          </tr>
          <tr>
            <td>Chat</td>
            <td>Messages you send in family chat, with sender and timestamp.</td>
          </tr>
          <tr>
            <td>Sessions</td>
            <td>
              Session and refresh tokens, stored hashed. Refresh tokens rotate on use, and a token
              reused after rotation revokes the whole session family.
            </td>
          </tr>
          <tr>
            <td>Devices</td>
            <td>
              If you install the companion app, an Apple push token for that device, its platform
              and app bundle ID. Nothing else about the device.
            </td>
          </tr>
          <tr>
            <td>Logs</td>
            <td>
              Server logs of requests and errors: timestamps, paths, status codes, durations, and
              the user or family ID involved. Email addresses in logs are masked. Photo contents,
              chat text, invite codes, tokens, and AI request or response bodies are not logged.
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <h2>Face recognition</h2>
    <p>
      When you upload a photo, the server can look for faces in it and suggest which family members
      appear. This runs entirely on the server that hosts your data, using a local face-recognition
      library. <strong>No image and no face data is sent to any third party for this.</strong>
    </p>
    <p>What is kept and what is not:</p>
    <ul>
      <li>
        For each person, a 128-number mathematical summary computed from their profile photo. It is
        not an image and cannot be turned back into one, but it does identify that person's face.
      </li>
      <li>
        Face measurements computed from an uploaded photo are used to compare against those
        summaries and are then discarded. Only the resulting tag — "this person is in this photo" —
        is stored, and a tag is marked so you can tell an automatic suggestion from one you made
        yourself.
      </li>
      <li>
        A person's face summary is deleted when that person is deleted, and every summary in a
        family is deleted when the family is.
      </li>
      <li>
        Face recognition is a convenience, not a certainty. It is wrong sometimes, and you can
        remove or correct any tag.
      </li>
    </ul>

    <h2>What leaves the server</h2>
    <p>
      Almost nothing does. Two features involve someone else's servers, and they are optional.
    </p>

    <h3>Sign in with Google</h3>
    <p>
      If you sign in with Google, Google tells the site your email address, name, and verification
      status. That is what is used to find or create your account; nothing is written back to
      Google, and the site gets no access to your Gmail, Drive, contacts, or anything else.
    </p>

    <h3>Push notifications</h3>
    <p>
      If you use the companion app and enable notifications, notification text is delivered through
      Apple's Push Notification service, which necessarily sees it. Email — password resets and
      backup alerts — is delivered through whichever mail provider the server is configured to use.
    </p>

    <h2>Who inside a family can see what</h2>
    <ul>
      <li>
        A family is the unit of privacy. Every member of your family can see all of that family's
        people, growth records, milestones, photos, activities, and chat. There are no per-member
        restrictions inside a family.
      </li>
      <li>
        Another family cannot see any of it. Every request is checked against the caller's family,
        and there is a test suite whose only job is to call each operation as an outsider and
        confirm it is refused.
      </li>
      <li>
        A family can invite another family to link, and a link shares only the specific people and
        the specific categories it names — for example milestones and photos for one child, without
        growth measurements. Chat is never shared by a link. A link grants nothing until it is
        accepted and nothing once it is revoked.
      </li>
      <li>
        I hold the server, so technically I can read what is on it. I do not, other than as needed
        to keep it running or to answer something you have asked me to look at.
      </li>
    </ul>

    <h2>How long things are kept</h2>
    <ul>
      <li>
        <strong>Your records</strong> are kept until you delete them. Deleting a person, photo,
        measurement, or message removes it from the database; deleted photo files are removed from
        disk along with the copies derived from them.
      </li>
      <li>
        <strong>Server logs</strong> rotate and are deleted after about ten days. The web server in
        front of the app keeps its own access log with IP addresses on a similar rotation.
      </li>
      <li>
        <strong>Backups</strong> are taken nightly, encrypted, and stored off the server. Seven
        daily and four weekly copies are kept, so something you delete today can persist in a backup
        for up to about a month before ageing out. Backups exist to survive the server dying and are
        not browsed.
      </li>
      <li>
        <strong>Expired sessions</strong> are purged automatically.
      </li>
    </ul>

    <h2>Deleting your account</h2>
    <p>
      Settings → Delete Account is where it lives. It requires typing your email address, plus your
      password if the account has one. It is immediate and it is not reversible.
    </p>
    <p>What deletion removes:</p>
    <ul>
      <li>Your account, password hash, sessions, refresh tokens, and password reset links.</li>
      <li>Your push device registrations.</li>
      <li>
        Your chat messages. Chat is the one place the site stores your words under your name, so
        deleting the account deletes them.
      </li>
    </ul>
    <p>What stays:</p>
    <ul>
      <li>
        People, growth records, milestones, photos, and tags stay with the family.
      </li>
    </ul>
    <div className="legal-callout">
      <p>
        The exception is a family your departure empties. Once nobody is left in a family, nobody
        can ever reach its contents again and that family is destroyed along with everything in it,
        including the photo files on disk and the face summaries derived from them. If you are the
        only member, deleting your account deletes everything.
      </p>
    </div>
    <p>
      Background work in progress is checked against the database before it writes, so a photo
      deleted mid-upload does not come back afterwards. Backups still holding your data age out on
      the schedule above.
    </p>

    <h2>Getting your data out</h2>
    <p>
      Settings → Data Management produces a file containing your family's records. You do not have
      to ask anyone for it and you do not have to be leaving to use it.
    </p>

    <h2>Children</h2>
    <p>
      This site is for adults to record information about their own children. Accounts are for
      adults.
    </p>

    <h2>Security</h2>
    <ul>
      <li>All traffic is served over HTTPS; session cookies are marked secure.</li>
      <li>Passwords are stored as bcrypt hashes; session and refresh tokens are stored hashed.</li>
      <li>
        Authentication endpoints, uploads, imports, AI calls, and WebSocket connections are rate
        limited.
      </li>
      <li>
        Login and password reset return the same response whether or not an account exists, so
        neither can be used to find out who has an account.
      </li>
    </ul>
    <p>
      No system is perfect, and this one is run by one person. If a breach affects your data, I will
      tell you at the address on your account.
    </p>

    <h2>Changes</h2>
    <p>
      If this page changes in a way that matters — new data collected, a new third party involved,
      or a change in how long things are kept — the date at the top changes and the change is
      announced in the app before it takes effect.
    </p>

    <h2>Questions</h2>
    <p>
      Ask on the <a href="/support">support page</a>. That includes requests to see, correct, or
      delete data that the app itself will not let you reach.
    </p>

    <LegalNav current="privacy" />
  </article>
);
