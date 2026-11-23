import * as preact from "preact";
import * as rpc from "vlens/rpc";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { copy } from "../../lib/copy";
import { familyPageSlug } from "../../lib/familyPage";
import "./family-page-styles";

type Data = { page: server.PublicFamilyPage | null };

/** The server's single answer for an unknown or unpublished slug (ErrFamilyPageUnavailable). */
const unavailableError = "This page isn't available";

export async function fetch(route: string, prefix: string): Promise<rpc.Response<Data>> {
  const slug = familyPageSlug(route);
  if (!slug) return rpc.ok<Data>({ page: null });
  const [resp, err] = await server.GetPublicFamilyPage({ slug });
  if (resp) return rpc.ok<Data>({ page: resp.page });
  if (err === unavailableError) return rpc.ok<Data>({ page: null });
  return [null, err || "Couldn't load this page"];
}

export function view(route: string, prefix: string, data: Data): preact.ComponentChild {
  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="family-page-container">
        {data.page ? <FamilyPageView page={data.page} /> : <Unavailable />}
      </main>
      <Footer />
    </div>
  );
}

const Unavailable = () => (
  <article className="family-page">
    <h1>{copy.familyPage.unavailableTitle}</h1>
    <p className="family-page-intro">{copy.familyPage.unavailable}</p>
  </article>
);

export const FamilyPageView = ({ page }: { page: server.PublicFamilyPage }) => (
  <article className="family-page">
    <h1>{page.title}</h1>
    {page.intro && <p className="family-page-intro">{page.intro}</p>}
    {page.members.length > 0 && (
      <ul className="family-page-people">
        {page.members.map((member, i) => (
          <li key={i} className="family-page-person">
            <Avatar member={member} />
            <span className="family-page-person-name">{member.name}</span>
            {member.ageYears !== null && member.ageYears !== undefined && (
              <p className="family-page-person-meta">{copy.familyPage.age(member.ageYears)}</p>
            )}
            {member.line && <p className="family-page-person-meta">{member.line}</p>}
          </li>
        ))}
      </ul>
    )}
    {page.photoUrls.length > 0 && (
      <div className="family-page-photos">
        {page.photoUrls.map(url => (
          <img key={url} src={url} alt="" loading="lazy" />
        ))}
      </div>
    )}
    <p className="family-page-footer-note">{copy.familyPage.sharedFrom}</p>
  </article>
);

const Avatar = ({ member }: { member: server.PublicFamilyPageMember }) => {
  if (!member.photoUrl) {
    return (
      <span className="family-page-avatar" aria-hidden="true">
        {member.name.charAt(0).toUpperCase()}
      </span>
    );
  }
  const style = {
    transform: `scale(${member.cropScale || 1})`,
    transformOrigin: `${member.cropX || 50}% ${member.cropY || 50}%`,
  };
  return (
    <span className="family-page-avatar">
      <img src={member.photoUrl} alt="" style={style} />
    </span>
  );
};
