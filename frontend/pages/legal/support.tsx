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
        <SupportPage />
      </main>
      <Footer />
    </div>
  );
}

const SupportPage = () => (
  <article className="legal-page">
    <h1>Support</h1>
    <p className="legal-updated">
      One person reads this address. Expect a reply within a few days.
    </p>

    <div className="legal-contact">
      <p className="legal-contact-address">
        <a href={`mailto:${SUPPORT_EMAIL}`}>{SUPPORT_EMAIL}</a>
      </p>
      <p>
        Anything at all: a bug, a question, a request to delete something, or a feature you wish
        existed.
      </p>
    </div>

    <h2>Things you can fix yourself, faster</h2>
    <ul>
      <li>
        <strong>Forgot your password</strong> — use <a href="/forgot-password">forgot password</a>.
        The link is single-use and expires shortly, so request a fresh one rather than reusing an
        old email.
      </li>
      <li>
        <strong>Change your password</strong> — Settings → Security. Changing it signs out every
        other device.
      </li>
      <li>
        <strong>Someone should not have your invite code</strong> — Settings → Invite Family
        Members, rotate the code. The old one stops working immediately.
      </li>
      <li>
        <strong>Remove a family member</strong> — the family owner can do this from Settings →
        Family Members.
      </li>
      <li>
        <strong>Leave a family</strong> — Settings → Family Members. The last remaining member
        cannot leave; delete the account instead, which removes the family with it.
      </li>
      <li>
        <strong>Export your data</strong> — Settings → Data Management, any time, without asking.
      </li>
      <li>
        <strong>Delete your account</strong> — Settings → Delete Account. Immediate and not
        reversible; export first if you want a copy. What goes and what stays is on the{" "}
        <a href="/privacy">privacy page</a>.
      </li>
      <li>
        <strong>A photo is stuck processing</strong> — large photos take a moment to resize. If it
        is still unfinished after a few minutes, email me the photo's date and caption.
      </li>
      <li>
        <strong>The wrong person was tagged in a photo</strong> — face suggestions are frequently
        wrong. Open the photo and correct the tag; nothing you fix gets re-suggested.
      </li>
    </ul>

    <h2>Reporting a problem</h2>
    <p>What makes a report quick to act on:</p>
    <ul>
      <li>What you were trying to do, and what happened instead.</li>
      <li>The page you were on, and roughly when.</li>
      <li>
        The reference code, if the error page showed one. It points straight at the matching server
        log.
      </li>
      <li>Browser and device, if it looks like a display problem.</li>
    </ul>
    <p>
      Please do not email passwords or invite codes. I never need them, and I will never ask for
      them.
    </p>

    <h2>Security reports</h2>
    <p>
      Use the same address and put "security" in the subject. Reports made in good faith are welcome
      and will not get your account in trouble. Please give me a chance to fix an issue before
      telling anyone else about it.
    </p>

    <h2>Privacy requests</h2>
    <p>
      Most requests — seeing your data, exporting it, deleting it — are handled by the app itself
      and need no email. For anything the app will not let you reach, write to the address above
      from the account's email address and say what you want.
    </p>

    <h2>What this service is</h2>
    <p>
      A personal project, run by one person on one server, offered to a small number of families for
      free. There is no support team and no guaranteed response time. What there is: I read every
      message, and I care that this works for your family. The <a href="/terms">terms of use</a> say
      what that does and does not promise.
    </p>

    <LegalNav current="support" />
  </article>
);
