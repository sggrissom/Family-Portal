import * as preact from "preact";
import * as rpc from "vlens/rpc";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView } from "../../lib/authHelpers";
import { ThumbnailImage } from "../../components/ResponsiveImage";
import { bookDates, joinNames } from "../../lib/book";
import { SAMPLES } from "../../lib/bookFixtures";
import { PRESET_CHOICES } from "./new-book";
import "./book-styles";

export async function fetch(
  route: string,
  prefix: string
): Promise<rpc.Response<server.ListBooksResponse>> {
  return server.ListBooks({ familyId: 0 });
}

export function view(
  route: string,
  prefix: string,
  data: server.ListBooksResponse
): preact.ComponentChild {
  if (!requireAuthInView()) return;
  const books = data.books ?? [];
  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="books-index">
        <h1>Books</h1>
        <p className="books-lede">Turn your family's memories into a story.</p>

        {books.length > 0 && (
          <ul className="books-shelf">
            {books.map(book => (
              <li key={book.id}>
                <a href={`/book/${book.id}`}>
                  <div className="books-shelf-cover">
                    {book.coverPhotoId > 0 && <ThumbnailImage photoId={book.coverPhotoId} alt="" />}
                  </div>
                  <strong>{book.title}</strong>
                  <span>
                    {book.personNames.length > 1 && `${joinNames(book.personNames)} · `}
                    {bookDates(book.startDate, book.endDate)}
                  </span>
                </a>
              </li>
            ))}
          </ul>
        )}

        {data.canEdit && (
          <>
            <h2>Start a book</h2>
            <ul className="books-list">
              {PRESET_CHOICES.map(choice => (
                <li key={choice.value}>
                  <a href={`/new-book?preset=${choice.value}`}>
                    <strong>{choice.label}</strong>
                    <span>{choice.blurb}</span>
                  </a>
                </li>
              ))}
            </ul>
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
      </main>
      <Footer />
    </div>
  );
}
