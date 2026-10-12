import * as preact from "preact";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView } from "../../lib/authHelpers";
import { canContribute, canContributeAnywhere } from "../../lib/authCache";
import { localDateString } from "../../lib/when";
import { formatLongDate } from "../../lib/dateUtils";
import { copy } from "../../lib/copy";
import { ageYearsAt } from "../../lib/interviews";
import { ProfileImage } from "../../components/ResponsiveImage";
import { initial } from "../../components/PersonChips";
import "./interview-styles";

const t = copy.interview.family;

export async function fetch(route: string, prefix: string) {
  return server.GetFamilyInterviews({ today: localDateString(new Date()) });
}

export function view(
  route: string,
  prefix: string,
  data: server.GetFamilyInterviewsResponse
): preact.ComponentChild {
  if (!requireAuthInView()) return;
  const people = data.people ?? [];
  const children = people.filter(r => r.child || r.count > 0 || r.inProgress.id > 0);
  const others = people.filter(r => !children.includes(r));

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="iv-page">
        <div className="iv-family-head">
          <div>
            <h1>{t.title}</h1>
            <p className="iv-hint">{t.intro}</p>
          </div>
          {canContributeAnywhere() && (
            <div className="iv-family-links">
              <a className="iv-link" href="/interview-questions">
                {t.editQuestions}
              </a>
            </div>
          )}
        </div>

        {people.length === 0 && <p className="iv-hint">{t.empty}</p>}

        <ul className="iv-family">
          {children.map(row => (
            <PersonRow key={row.person.id} row={row} />
          ))}
        </ul>

        {others.length > 0 && (
          <details className="iv-others">
            <summary>{t.others}</summary>
            <ul className="iv-family">
              {others.map(row => (
                <PersonRow key={row.person.id} row={row} />
              ))}
            </ul>
          </details>
        )}
      </main>
      <Footer />
    </div>
  );
}

const PersonRow = ({ row }: { row: server.FamilyInterviewRow }) => {
  const p = row.person;
  const editable = canContribute(p.familyId);
  return (
    <li className="iv-family-row">
      <a className="iv-family-who" href={`/profile/${p.id}?tab=interviews`}>
        <span className="iv-family-avatar">
          {p.profilePhotoId ? (
            <ProfileImage
              photoId={p.profilePhotoId}
              alt=""
              className="iv-family-photo"
              cropX={p.profileCropX}
              cropY={p.profileCropY}
              cropScale={p.profileCropScale}
            />
          ) : (
            initial(p.name)
          )}
        </span>
        <span>
          <strong>{p.name}</strong>
          <small className="iv-hint">
            {row.last.id
              ? t.last(
                  ageYearsAt(p.birthday, row.last.date),
                  formatLongDate(row.last.date),
                  row.count
                )
              : t.never}
          </small>
        </span>
      </a>
      <span className="iv-family-actions">
        {row.due && !row.inProgress.id && <span className="iv-badge">{t.due}</span>}
        {editable &&
          (row.inProgress.id ? (
            <a className="btn btn-primary btn-small" href={`/interview/${row.inProgress.id}/ask`}>
              {t.continue(row.answered, row.inProgress.questionIds.length)}
            </a>
          ) : (
            <a
              className={row.due ? "btn btn-primary btn-small" : "btn btn-secondary btn-small"}
              href={`/interview/new/${p.id}`}
            >
              {t.start}
            </a>
          ))}
      </span>
    </li>
  );
};
