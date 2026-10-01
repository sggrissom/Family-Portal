import * as preact from "preact";
import * as vlens from "vlens";
import * as core from "vlens/core";
import * as rpc from "vlens/rpc";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView } from "../../lib/authHelpers";
import { ThumbnailImage } from "../../components/ResponsiveImage";
import { bookDates, dayOf, draftSelection, firstBirthday } from "../../lib/book";
import { SAMPLES } from "../../lib/bookFixtures";
import { localDateString } from "../../lib/when";
import "./book-styles";

type BooksData = {
  books: server.BookSummary[];
  canEdit: boolean;
  people: server.Person[];
};

export async function fetch(route: string, prefix: string): Promise<rpc.Response<BooksData>> {
  const [[books, booksErr], [people, peopleErr]] = await Promise.all([
    server.ListBooks({ familyId: 0 }),
    server.ListPeople({}),
  ]);
  if (!books) return [null, booksErr || "Failed to load books"];
  if (!people) return [null, peopleErr || "Failed to load people"];
  return rpc.ok({ books: books.books ?? [], canEdit: books.canEdit, people: people.people ?? [] });
}

type StartState = { busyPersonId: number; error: string };
const useStart = vlens.declareHook((): StartState => ({ busyPersonId: 0, error: "" }));

async function startBook(state: StartState, person: server.Person) {
  state.busyPersonId = person.id;
  state.error = "";
  vlens.scheduleRedraw();

  const [sources, err] = await server.GetBookSources({ personId: person.id });
  if (!sources) {
    state.busyPersonId = 0;
    state.error = err || "Could not gather records";
    vlens.scheduleRedraw();
    return;
  }
  const draft = draftSelection(sources.sources);
  const [created, createErr] = await server.CreateBook({
    personId: person.id,
    content: {
      title: draft.title,
      coverPhotoId: draft.coverPhotoId,
      density: "balanced",
      introduction: "",
      letter: "",
      signature: "",
      showGrowth: true,
      items: draft.items,
      excluded: [],
    },
  });
  state.busyPersonId = 0;
  if (!created) {
    state.error = createErr || "Could not create the book";
    vlens.scheduleRedraw();
    return;
  }
  core.setRoute(`/book/${created.book.id}`);
}

export function view(route: string, prefix: string, data: BooksData): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) return;
  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="books-index">
        <BooksPage data={data} familyId={currentAuth.familyId} />
      </main>
      <Footer />
    </div>
  );
}

const BooksPage = ({ data, familyId }: { data: BooksData; familyId: number }) => {
  const state = useStart();
  const today = localDateString(new Date());
  const candidates = data.people
    .filter(p => p.familyId === familyId && !p.isPregnancy)
    .filter(p => dayOf(p.birthday) > "1000" && dayOf(p.birthday) <= today)
    .sort((a, b) => b.birthday.localeCompare(a.birthday));
  const withBook = new Set(data.books.map(b => b.personId));

  return (
    <>
      <h1>Books</h1>
      <p className="books-lede">Turn your family's memories into a story.</p>

      {data.books.length > 0 && (
        <ul className="books-shelf">
          {data.books.map(book => (
            <li key={book.id}>
              <a href={`/book/${book.id}`}>
                <div className="books-shelf-cover">
                  {book.coverPhotoId > 0 && <ThumbnailImage photoId={book.coverPhotoId} alt="" />}
                </div>
                <strong>{book.title}</strong>
                <span>{bookDates(book.startDate, book.endDate)}</span>
              </a>
            </li>
          ))}
        </ul>
      )}

      {data.canEdit && (
        <>
          <h2>Start a first-year book</h2>
          <p className="books-lede">
            We'll gather the photos and milestones from their first year into a draft you can read
            straight away and refine later.
          </p>
          {state.error && (
            <p className="error-message" role="alert">
              {state.error}
            </p>
          )}
          {candidates.length === 0 ? (
            <p className="books-empty">Add someone with a birthday to make a first-year book.</p>
          ) : (
            <ul className="books-list">
              {candidates.map(person => (
                <li key={person.id}>
                  <button
                    type="button"
                    disabled={state.busyPersonId !== 0}
                    onClick={() => startBook(state, person)}
                  >
                    <strong>
                      {state.busyPersonId === person.id
                        ? `Gathering ${person.name}'s first year…`
                        : `${person.name}'s first year`}
                    </strong>
                    <span>
                      {bookDates(person.birthday, firstBirthday(person.birthday))}
                      {withBook.has(person.id) && " · already has a book"}
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </>
      )}

      <p className="books-samples">
        Sample books:{" "}
        {SAMPLES.map((sample, i) => (
          <span key={sample.value}>
            {i > 0 && " · "}
            <a href={`/book/sample?sample=${sample.value}`}>{sample.label.toLowerCase()}</a>
          </span>
        ))}
      </p>
    </>
  );
};
