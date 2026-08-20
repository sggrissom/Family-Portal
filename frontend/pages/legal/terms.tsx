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
        <TermsPage />
      </main>
      <Footer />
    </div>
  );
}

const TermsPage = () => (
  <article className="legal-page">
    <h1>Terms of use</h1>
    <p className="legal-updated">Last updated 19 August 2026</p>

    <p className="legal-lead">
      Family Record is run by one person as a personal project, offered to a small number of
      families. These terms are short on purpose. Using the site means agreeing to them.
    </p>

    <h2>What you get</h2>
    <p>
      An account, a family, and space to keep your family's records and photos on a server I
      maintain. There is no charge.
    </p>

    <h2>Your account</h2>
    <ul>
      <li>
        Keep your password to yourself and keep your invite code out of public view. Anyone holding
        a valid invite code can join your family and see everything in it; rotate it from settings
        if you are unsure who has it.
      </li>
      <li>You are responsible for what happens under your account.</li>
    </ul>

    <h2>What you put in</h2>
    <p>
      Your records and photos are yours. You keep every right you had in them; hosting them gives me
      no ownership and no licence to use them for anything beyond running the service for you.
    </p>
    <p>By uploading, you confirm that:</p>
    <ul>
      <li>You have the right to upload it.</li>
      <li>
        For photographs and information about a child, you are that child's parent or guardian, or
        you have their parent's or guardian's agreement.
      </li>
      <li>
        For photographs of other adults, they are fine with you storing them in a private family
        archive.
      </li>
    </ul>

    <h2>What you may not do</h2>
    <ul>
      <li>Upload anything illegal.</li>
      <li>Upload material you do not have the right to, or use the site to harass anyone.</li>
      <li>
        Try to reach another family's data, break authentication, or work around the rate limits.
      </li>
      <li>
        Automate against the service in a way that degrades it for others, or resell access to it.
      </li>
    </ul>
    <p>If you find a security flaw, please report it.</p>

    <h2>Families and shared content</h2>
    <p>
      Every member of a family can see and edit that family's records. When someone leaves a family
      or deletes their account, the family's people, growth records, milestones, and photos stay
      with the family; their own chat messages go with them. A family that nobody is left in is
      deleted along with its contents, because nothing could ever reach it again. The{" "}
      <a href="/privacy">privacy page</a> spells this out in full.
    </p>
    <p>
      Family links share only the people and categories the link names, and either family can revoke
      a link at any time.
    </p>

    <h2>Availability</h2>
    <p>
      This runs on one server, maintained in my spare time. It will go down sometimes, occasionally
      without warning, and features may change or be removed. Backups are taken nightly and restores
      have been tested, but I cannot guarantee that nothing is ever lost. Keep your own copy of
      anything you would be devastated to lose — Settings → Data Management exists for exactly that,
      and using it now and then is a good idea.
    </p>

    <h2>Ending it</h2>
    <p>
      You can delete your account at any time from Settings → Delete Account. Export first if you
      want a copy; deletion is immediate and not reversible.
    </p>
    <p>
      I may suspend or remove an account that breaks these terms, particularly the rules about
      children and consent, or that puts the service or other families at risk. Where circumstances
      allow, I will tell you first and give you a chance to export.
    </p>
    <p>
      If the service is ever shut down for good, you will get at least 30 days' notice by email and
      the export will keep working through that period.
    </p>

    <h2>No warranty, and limits</h2>
    <p>
      The service is provided as it is, without warranty of any kind. To the fullest extent the law
      allows, I am not liable for lost data, lost profits, or any indirect or consequential loss
      arising from using it. Nothing here limits liability that cannot legally be limited.
    </p>

    <h2>Changes to these terms</h2>
    <p>
      If these terms change, the date at the top changes and you will be told in the app before the
      change takes effect. Continuing to use the site after that means accepting the new version.
    </p>

    <h2>Questions</h2>
    <p>
      Everything goes to the <a href="/support">support page</a>.
    </p>

    <LegalNav current="terms" />
  </article>
);
