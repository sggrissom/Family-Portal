import * as preact from "preact";
import * as server from "../server";
import { copy } from "../lib/copy";
import { InterviewGroup, ageYearsAt, firstName, shortQuestion } from "../lib/interviews";
import { ThumbnailImage } from "./ResponsiveImage";
import "./interview-card-styles";

interface InterviewCardProps {
  group: InterviewGroup;
  person?: server.Person;
  interview?: server.Interview;
  showName?: boolean;
  highlights?: number;
}

export const InterviewCard = ({
  group,
  person,
  interview,
  showName = true,
  highlights = 3,
}: InterviewCardProps) => {
  const t = copy.interview.card;
  const age = person?.birthday ? ageYearsAt(person.birthday, group.date) : null;
  const name = person && showName ? firstName(person.name) : null;
  const total = interview?.questionIds?.length ?? 0;
  const count =
    total > group.answers.length
      ? t.someAnswers(group.answers.length, total)
      : t.answers(group.answers.length);
  return (
    <a href={`/interview/${group.interviewId}`} className="interview-card">
      {interview && interview.photoId > 0 ? (
        <ThumbnailImage photoId={interview.photoId} alt="" className="interview-card-photo" />
      ) : (
        <span className="interview-card-icon" aria-hidden="true">
          🎤
        </span>
      )}
      <span className="interview-card-body">
        <span className="interview-card-title">
          <strong>
            {interview?.title
              ? name
                ? `${name} · ${interview.title}`
                : interview.title
              : name
                ? t.whose(name)
                : t.title}
          </strong>
          {age !== null && ` · ${t.age(age)}`}
          {interview?.status === "in-progress" && ` · ${t.inProgress}`}
        </span>
        <span className="interview-card-lines">
          {group.answers.slice(0, highlights).map(a => (
            <span key={a.id} className="interview-card-line">
              <span className="interview-card-q">{shortQuestion(a.context)}:</span> {a.description}
            </span>
          ))}
        </span>
        <span className="interview-card-count">{count}</span>
      </span>
    </a>
  );
};

export const InterviewCards = ({
  groups,
  people,
  interviews,
  subjectId,
}: {
  groups: InterviewGroup[];
  people: Map<number, server.Person>;
  interviews: Map<number, server.Interview>;
  subjectId?: number;
}) => (
  <>
    {groups.map(g => (
      <InterviewCard
        key={`i${g.interviewId}`}
        group={g}
        person={people.get(g.personId)}
        interview={interviews.get(g.interviewId)}
        showName={g.personId !== subjectId}
      />
    ))}
  </>
);
