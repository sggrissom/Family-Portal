import * as preact from "preact";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView } from "../../lib/authHelpers";
import { addDays, dayOf, firstBirthday, longDay } from "../../lib/book";
import { SAMPLES } from "../../lib/bookFixtures";
import { localDateString } from "../../lib/when";
import "./book-styles";

export async function fetch(route: string, prefix: string) {
  return server.ListPeople({});
}

export function view(
  route: string,
  prefix: string,
  data: server.ListPeopleResponse
): preact.ComponentChild {
  if (!requireAuthInView()) return;
  const today = localDateString(new Date());
  const babies = (data.people ?? [])
    .filter(p => !p.isPregnancy && /^\d{4}-\d{2}-\d{2}/.test(p.birthday ?? ""))
    .filter(p => dayOf(p.birthday) > "1000" && dayOf(p.birthday) <= today)
    .sort((a, b) => b.birthday.localeCompare(a.birthday));

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="books-index">
        <h1>Books</h1>
        <p className="books-lede">
          Turn your family's memories into a story. This is an early prototype of a first-year book,
          built from what you have already recorded. Nothing here is saved.
        </p>

        <h2>First year</h2>
        {babies.length === 0 ? (
          <p className="books-empty">Add someone with a birthday to make a first-year book.</p>
        ) : (
          <ul className="books-list">
            {babies.map(person => (
              <li key={person.id}>
                <a href={`/book/${person.id}`}>
                  <strong>{person.name}'s first year</strong>
                  <span>
                    {longDay(dayOf(person.birthday))} –{" "}
                    {longDay(addDays(firstBirthday(person.birthday), -1))}
                  </span>
                </a>
              </li>
            ))}
          </ul>
        )}

        <h2>Sample books</h2>
        <ul className="books-list">
          {SAMPLES.map(sample => (
            <li key={sample.value}>
              <a href={`/book/sample?sample=${sample.value}`}>
                <strong>{sample.label} sample</strong>
                <span>Made-up records with placeholder photos</span>
              </a>
            </li>
          ))}
        </ul>
      </main>
      <Footer />
    </div>
  );
}
