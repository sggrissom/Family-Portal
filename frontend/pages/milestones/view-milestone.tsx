import * as preact from "preact";
import * as rpc from "vlens/rpc";
import * as core from "vlens/core";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView } from "../../lib/authHelpers";
import { getIdFromRoute } from "../../lib/routeHelpers";
import { ErrorPage } from "../../components/ErrorPage";
import { PhotoStrip } from "../../components/PhotoPicker";
import { SameAgeStrip } from "../../components/SameAgeRows";
import { getCategoryIcon, getCategoryLabel } from "../../lib/milestoneHelpers";
import { MilestoneText } from "../../components/MilestoneText";
import { formatDate } from "../../lib/dateUtils";
import { formatAgeAtMeasurement, isValidBirthday } from "../../lib/growthPercentiles";
import { monthsOld } from "../../lib/sameAge";
import { localDateString } from "../../lib/when";
import { copy } from "../../lib/copy";
import "./view-milestone-styles";

type ViewMilestoneData = {
  milestone: server.Milestone | null;
  person: server.Person | null;
  sameAge: server.GetSameAgeResponse | null;
  matches: server.MilestoneMatch[];
};

export async function fetch(
  route: string,
  prefix: string
): Promise<rpc.Response<ViewMilestoneData>> {
  const [resp, err] = await server.GetMilestone({ id: getIdFromRoute(route) || 0 });
  if (!resp) return [null, err];
  const milestone = resp.milestone;

  const [people] = await server.ListPeople({});
  const person = people?.people.find(p => p.id === milestone.personId) ?? null;

  let sameAge: server.GetSameAgeResponse | null = null;
  if (person && isValidBirthday(person.birthday)) {
    const age = monthsOld(person.birthday, milestone.milestoneDate);
    if (age >= 0) {
      [sameAge] = await server.GetSameAge({
        ageMonths: age,
        fromPersonId: person.id,
        today: localDateString(new Date()),
      });
    }
  }
  const [matches] = await server.GetMilestoneMatches({ milestoneId: milestone.id });
  return [{ milestone, person, sameAge, matches: matches?.matches ?? [] }, ""];
}

export function view(
  route: string,
  prefix: string,
  data: ViewMilestoneData
): preact.ComponentChild {
  if (!requireAuthInView()) return;

  if (!data.milestone) {
    return (
      <ErrorPage
        title="Milestone Not Found"
        message={copy.milestoneDetail.notFound}
        containerClass="view-milestone-container"
      />
    );
  }

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="view-milestone-container">
        <ViewMilestonePage
          milestone={data.milestone}
          person={data.person}
          sameAge={data.sameAge}
          matches={data.matches}
        />
      </main>
      <Footer />
    </div>
  );
}

function goBack(event: Event) {
  if (window.history.length > 1) {
    event.preventDefault();
    window.history.back();
  }
}

async function deleteMilestone(milestone: server.Milestone) {
  if (!confirm(copy.milestoneDetail.confirmDelete(milestone.description))) return;
  const [resp, err] = await server.DeleteMilestone({ id: milestone.id });
  if (resp?.success) {
    core.setRoute(`/profile/${milestone.personId}`);
  } else {
    alert(err || "That milestone could not be deleted.");
  }
}

const ViewMilestonePage = ({
  milestone,
  person,
  sameAge,
  matches,
}: {
  milestone: server.Milestone;
  person: server.Person | null;
  sameAge: server.GetSameAgeResponse | null;
  matches: server.MilestoneMatch[];
}) => {
  const age =
    person && isValidBirthday(person.birthday)
      ? monthsOld(person.birthday, milestone.milestoneDate)
      : null;

  return (
    <div className="view-milestone-page">
      <a
        href={person ? `/profile/${person.id}` : "/dashboard"}
        className="back-link"
        onClick={goBack}
      >
        ← {copy.milestoneDetail.back}
      </a>

      <article className="milestone-detail">
        <div className="milestone-detail-category">
          <span aria-hidden="true">{getCategoryIcon(milestone.category)}</span>{" "}
          {getCategoryLabel(milestone.category)}
        </div>
        <h1>
          <MilestoneText milestone={milestone} />
        </h1>
        {milestone.context && <p className="milestone-detail-context">{milestone.context}</p>}
        <p className="milestone-detail-meta">
          {person && <a href={`/profile/${person.id}`}>{person.name}</a>}
          {age !== null &&
            age >= 0 &&
            ` · ${copy.milestoneDetail.atAge(formatAgeAtMeasurement(age))}`}
          {` · ${formatDate(milestone.milestoneDate)}`}
        </p>
        <PhotoStrip photoIds={milestone.photoIds} />
        <div className="milestone-detail-actions">
          <a href={`/edit-milestone/${milestone.id}`} className="btn btn-secondary">
            ✏️ {copy.milestoneDetail.edit}
          </a>
          <button
            type="button"
            className="btn btn-danger"
            onClick={() => deleteMilestone(milestone)}
          >
            🗑️ {copy.milestoneDetail.delete}
          </button>
        </div>
      </article>

      {matches.length > 0 && (
        <section className="milestone-matches">
          <h2>The same milestone in the family</h2>
          <ul>
            {matches.map(match => (
              <li key={match.milestone.id}>
                <a href={`/milestone/${match.milestone.id}`}>
                  <strong>{match.person.name.split(" ")[0]}</strong>
                  {match.ageMonths >= 0 && ` at ${formatAgeAtMeasurement(match.ageMonths)}`}
                  <span className="milestone-match-text">
                    <MilestoneText milestone={match.milestone} />
                  </span>
                </a>
              </li>
            ))}
          </ul>
        </section>
      )}

      {person && (
        <SameAgeStrip
          data={sameAge}
          today={localDateString(new Date())}
          exceptPersonId={person.id}
        />
      )}
    </div>
  );
};
