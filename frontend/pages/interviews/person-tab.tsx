import * as preact from "preact";
import * as vlens from "vlens";
import * as server from "../../server";
import { copy } from "../../lib/copy";
import { canContribute } from "../../lib/authCache";
import { SegmentedControl } from "../../components/SegmentedControl";
import { ThumbnailImage } from "../../components/ResponsiveImage";
import {
  GridCell,
  TimelineSort,
  columnLabel,
  filterTimelines,
  firstName,
  interviewGrid,
  interviewTitle,
  normalizeAnswer,
  questionTimelines,
  rowLabel,
  sortTimelines,
} from "../../lib/interviews";
import "./interview-styles";

const t = copy.interview.tab;

type View = "years" | "questions";

type TabState = { view: View; sort: TimelineSort; query: string };

const useTabState = vlens.declareHook(
  (_personId: number): TabState => ({ view: "years", sort: "list", query: "" })
);

export function hasInterviewTab(
  bundle: server.GetPersonInterviewsResponse | null,
  familyId: number
) {
  if (!bundle) return false;
  return bundle.interviews.length > 0 || canContribute(familyId);
}

export const InterviewsTab = ({ bundle }: { bundle: server.GetPersonInterviewsResponse }) => {
  const person = bundle.person;
  const state = useTabState(person.id);
  const editable = canContribute(person.familyId);
  const inProgress = bundle.interviews.find(i => i.status === "in-progress");
  const name = firstName(person.name);

  if (bundle.interviews.length === 0) {
    return (
      <div className="iv-empty">
        <p>{t.empty(name)}</p>
        {editable && (
          <>
            <a className="btn btn-primary" href={`/interview/new/${person.id}`}>
              {t.startFirst}
            </a>
            <p className="iv-tab-links">
              <a className="iv-link" href={`/interview/past/${person.id}`}>
                {t.typeIn}
              </a>
              <a className="iv-link" href={`/interview/import?person=${person.id}`}>
                {t.pasteIn}
              </a>
            </p>
          </>
        )}
      </div>
    );
  }

  return (
    <div className="iv-tab">
      <div className="iv-tab-head">
        <SegmentedControl
          label={t.viewLabel}
          options={[
            { value: "years", label: t.years },
            { value: "questions", label: t.questions },
          ]}
          value={state.view}
          onChange={(view: View) => {
            state.view = view;
            vlens.scheduleRedraw();
          }}
        />
        {editable &&
          (inProgress ? (
            <a className="btn btn-primary btn-small" href={`/interview/${inProgress.id}/ask`}>
              {t.continue}
            </a>
          ) : (
            <a className="btn btn-secondary btn-small" href={`/interview/new/${person.id}`}>
              {t.start}
            </a>
          ))}
      </div>

      <ul className="iv-interview-list">
        {[...bundle.interviews].reverse().map(i => (
          <li key={i.id}>
            <a href={`/interview/${i.id}`}>
              {i.photoId > 0 && <ThumbnailImage photoId={i.photoId} alt="" className="iv-thumb" />}
              <span>{interviewTitle(i, person)}</span>
              <small>
                {columnLabel(i, person)}
                {i.status === "in-progress" && ` · ${t.inProgress}`}
              </small>
            </a>
          </li>
        ))}
      </ul>

      {editable && (
        <p className="iv-tab-links">
          <a className="iv-link" href={`/interview/past/${person.id}`}>
            {t.typeIn}
          </a>
          <a className="iv-link" href={`/interview/import?person=${person.id}`}>
            {t.pasteIn}
          </a>
          <a className="iv-link" href="/interview/compare">
            {copy.interview.compare.link}
          </a>
        </p>
      )}

      {state.view === "years" ? (
        <YearsView bundle={bundle} />
      ) : (
        <QuestionsView bundle={bundle} state={state} />
      )}
    </div>
  );
};

const YearsView = ({ bundle }: { bundle: server.GetPersonInterviewsResponse }) => {
  const { interviews, sections } = interviewGrid(bundle);
  return (
    <>
      <div className="iv-grid-wrap">
        <table className="iv-grid">
          <thead>
            <tr>
              <th scope="col" className="iv-grid-q" aria-label={t.question} />
              {interviews.map(i => (
                <th key={i.id} scope="col">
                  <a href={`/interview/${i.id}`}>
                    {i.photoId > 0 && (
                      <ThumbnailImage photoId={i.photoId} alt="" className="iv-thumb" />
                    )}
                    {columnLabel(i, bundle.person)}
                  </a>
                </th>
              ))}
            </tr>
          </thead>
          {sections.map(section => (
            <tbody key={section.name}>
              <tr className="iv-grid-section">
                <th scope="rowgroup" colSpan={interviews.length + 1}>
                  {section.name}
                </th>
              </tr>
              {section.rows.map(row => (
                <tr key={row.question.id}>
                  <th scope="row" className="iv-grid-q">
                    {rowLabel(row)}
                  </th>
                  {row.cells.map((cell, i) => (
                    <td key={interviews[i].id} className={cellClass(cell)}>
                      <Cell cell={cell} />
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          ))}
        </table>
      </div>
      <p className="iv-hint iv-grid-legend">
        <span className="iv-legend-changed">{t.legendChanged}</span> · {t.legendSame}
      </p>
      <div className="iv-grid-narrow">
        <QuestionCards bundle={bundle} sort="list" query="" />
      </div>
    </>
  );
};

function cellClass(cell: GridCell): string {
  if (cell.skipped) return "iv-cell skipped";
  return cell.trend ? `iv-cell ${cell.trend}` : "iv-cell";
}

const Cell = ({ cell }: { cell: GridCell }) => {
  if (cell.answer) return <a href={`/milestone/${cell.answer.id}`}>{cell.answer.description}</a>;
  if (cell.skipped) return <span aria-label={t.skipped}>–</span>;
  return <span aria-label={cell.asked ? t.unanswered : t.notPlanned}>—</span>;
};

const QuestionsView = ({
  bundle,
  state,
}: {
  bundle: server.GetPersonInterviewsResponse;
  state: TabState;
}) => (
  <>
    <div className="iv-questions-tools">
      <input
        type="search"
        aria-label={t.filter}
        placeholder={t.filter}
        value={state.query}
        onInput={e => {
          state.query = e.currentTarget.value;
          vlens.scheduleRedraw();
        }}
      />
      <SegmentedControl
        label={t.sortLabel}
        options={[
          { value: "list", label: t.sortList },
          { value: "consistent", label: t.sortConsistent },
          { value: "changed", label: t.sortChanged },
        ]}
        value={state.sort}
        onChange={(sort: TimelineSort) => {
          state.sort = sort;
          vlens.scheduleRedraw();
        }}
      />
    </div>
    <QuestionCards bundle={bundle} sort={state.sort} query={state.query} />
  </>
);

const QuestionCards = ({
  bundle,
  sort,
  query,
}: {
  bundle: server.GetPersonInterviewsResponse;
  sort: TimelineSort;
  query: string;
}) => {
  const timelines = sortTimelines(filterTimelines(questionTimelines(bundle), query), sort);
  if (timelines.length === 0) return <p className="iv-hint">{t.noMatches}</p>;
  return (
    <div className="iv-cards">
      {timelines.map(tl => {
        const latest = tl.points[tl.points.length - 1];
        return (
          <section key={tl.question.id} className="iv-card">
            <h3>
              <a
                href={`/interview/compare?question=${tl.question.id}`}
                title={copy.interview.compare.link}
              >
                {latest?.answer.context || tl.question.text}
              </a>
            </h3>
            <ol className="iv-timeline">
              {tl.points.map((p, i) => {
                const prev = tl.points[i - 1];
                const same =
                  prev &&
                  normalizeAnswer(prev.answer.description) ===
                    normalizeAnswer(p.answer.description);
                return (
                  <li key={p.answer.id} className={same ? "same" : ""}>
                    <span className="iv-timeline-age">{t.age(p.age)}</span>
                    <a href={`/milestone/${p.answer.id}`}>{p.answer.description}</a>
                  </li>
                );
              })}
            </ol>
          </section>
        );
      })}
    </div>
  );
};
