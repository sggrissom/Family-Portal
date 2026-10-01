import * as preact from "preact";
import * as core from "vlens/core";
import * as rpc from "vlens/rpc";
import * as server from "../../server";
import { Header } from "../../layout";
import { requireAuthInView } from "../../lib/authHelpers";
import { getIdFromRoute } from "../../lib/routeHelpers";
import { ResponsiveImage } from "../../components/ResponsiveImage";
import { SegmentedControl } from "../../components/SegmentedControl";
import {
  Block,
  Book,
  BookMoment,
  BookPhoto,
  BookSource,
  DENSITIES,
  Density,
  GrowthPoint,
  Selection,
  assembleBook,
  isDensity,
  shortDay,
} from "../../lib/book";
import { SAMPLES, Sample, isPlaceholderPhoto, isSample, sampleBook } from "../../lib/bookFixtures";
import "./book-styles";

type BookData = {
  source: BookSource;
  selection: Selection;
  bookId: number;
  canEdit: boolean;
  sample: Sample | null;
  density: Density;
};

export function selectionOf(book: server.Book): Selection {
  return {
    title: book.title,
    startDate: book.startDate,
    endDate: book.endDate,
    coverPhotoId: book.coverPhotoId,
    introduction: book.introduction,
    letter: book.letter,
    signature: book.signature,
    showGrowth: book.showGrowth,
    items: book.items ?? [],
  };
}

export async function fetch(route: string, prefix: string): Promise<rpc.Response<BookData>> {
  const params = new URLSearchParams(route.split("?")[1] ?? "");
  const sample = params.get("sample");
  if (isSample(sample)) {
    const density = params.get("density");
    const chosen = isDensity(density) ? density : "balanced";
    return rpc.ok({
      ...sampleBook(sample, chosen),
      bookId: 0,
      canEdit: false,
      sample,
      density: chosen,
    });
  }
  const [resp, err] = await server.GetBook({ id: getIdFromRoute(route) || 0 });
  if (!resp) return [null, err || "Failed to load book"];
  return rpc.ok({
    source: resp.sources,
    selection: selectionOf(resp.book),
    bookId: resp.book.id,
    canEdit: resp.canEdit,
    sample: null,
    density: isDensity(resp.book.density) ? resp.book.density : "balanced",
  });
}

function samplePath(sample: Sample, density: Density) {
  const params = new URLSearchParams({ sample });
  if (density !== "balanced") params.set("density", density);
  return `/book/sample?${params}`;
}

export function view(route: string, prefix: string, data: BookData): preact.ComponentChild {
  if (!requireAuthInView()) return;
  const book = assembleBook(data.source, data.selection);
  return (
    <div className="book-page">
      <Header isHome={false} />
      {data.sample ? (
        <SampleBar sample={data.sample} density={data.density} />
      ) : (
        <nav className="book-reader-bar" aria-label="Book">
          <a href="/books">← Books</a>
          {data.canEdit && (
            <a className="book-reader-edit" href={`/edit-book/${data.bookId}`}>
              Edit book
            </a>
          )}
        </nav>
      )}
      <main id="app">
        <BookReader book={book} />
        {data.sample && <EditorNotes book={book} />}
      </main>
    </div>
  );
}

const SampleBar = ({ sample, density }: { sample: Sample; density: Density }) => (
  <div className="book-proto-bar">
    <span className="book-proto-tag">Sample</span>
    <SegmentedControl
      label="Sample"
      options={SAMPLES}
      value={sample}
      onChange={next => core.replaceRoute(samplePath(next, density))}
    />
    <SegmentedControl
      label="Length"
      options={DENSITIES.map(d => ({ value: d, label: d[0].toUpperCase() + d.slice(1) }))}
      value={density}
      onChange={next => core.replaceRoute(samplePath(sample, next))}
    />
  </div>
);

export const EditorNotes = ({ book }: { book: Book }) => {
  const n = book.notes;
  return (
    <details className="book-editor-notes">
      <summary>What went into this book</summary>
      <ul>
        <li>
          {n.milestonesUsed} of {n.milestonesInRange} records from the year
        </li>
        <li>
          {n.photosUsed} of {n.photosInRange} photos from the year
        </li>
        {n.missing > 0 && (
          <li>{n.missing} items were deleted or are no longer visible to you and are skipped</li>
        )}
        {n.undatedPhotos > 0 && <li>{n.undatedPhotos} photos have no date and were left out</li>}
        {n.unreadyPhotos > 0 && <li>{n.unreadyPhotos} photos are still processing or failed</li>}
        {n.hiddenMonths.length > 0 && (
          <li>
            Nothing included at {n.hiddenMonths.map(m => `${m} months`).join(", ")}; those months
            are folded into their neighbours
          </li>
        )}
      </ul>
    </details>
  );
};

function scrollToChapter(id: string) {
  const reduce = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  document
    .getElementById(`chapter-${id}`)
    ?.scrollIntoView({ behavior: reduce ? "auto" : "smooth", block: "start" });
}

export const BookReader = ({ book }: { book: Book }) => (
  <article className="book" aria-label={book.title}>
    <header className="book-cover">
      {book.cover && (
        <BookImage photo={book.cover} className="book-cover-photo" sizes="100vw" eager />
      )}
      <div className="book-cover-text">
        <h1>{book.title}</h1>
        <p>{book.dates}</p>
      </div>
    </header>

    <nav className="book-contents" aria-label="Chapters">
      <h2>Contents</h2>
      <ol>
        {book.chapters.map(chapter => (
          <li key={chapter.id}>
            <button type="button" onClick={() => scrollToChapter(chapter.id)}>
              <span>{chapter.title}</span>
              {chapter.dates && <span className="book-contents-dates">{chapter.dates}</span>}
            </button>
          </li>
        ))}
      </ol>
    </nav>

    {book.chapters.map(chapter => (
      <section
        key={chapter.id}
        id={`chapter-${chapter.id}`}
        className="book-chapter"
        aria-labelledby={`chapter-${chapter.id}-title`}
      >
        <header className="book-chapter-head">
          {chapter.dates && <p className="book-chapter-dates">{chapter.dates}</p>}
          <h2 id={`chapter-${chapter.id}-title`}>{chapter.title}</h2>
        </header>
        {chapter.blocks.map((block, i) => (
          <BlockView key={i} block={block} />
        ))}
      </section>
    ))}

    <footer className="book-end">
      <span aria-hidden="true">❦</span>
      <p>{book.ending}</p>
    </footer>
  </article>
);

const BlockView = ({ block }: { block: Block }) => {
  switch (block.kind) {
    case "hero":
      return <PhotoFigure photo={block.photo} className="book-hero" sizes="100vw" />;
    case "photos":
      return (
        <div className={`book-photos book-photos-${block.photos.length}`}>
          {block.photos.map(photo => (
            <PhotoFigure
              key={photo.id}
              photo={photo}
              sizes="(max-width: 700px) 50vw, 400px"
              className={photo.height > photo.width ? "is-tall" : ""}
            />
          ))}
        </div>
      );
    case "moment":
      return <MomentView moment={block.moment} photo={block.photo} />;
    case "quote":
      return (
        <blockquote className="book-quote">
          <p>{block.moment.text}</p>
          {block.moment.context && <p className="book-quote-context">{block.moment.context}</p>}
          <When moment={block.moment} />
        </blockquote>
      );
    case "artwork":
      return (
        <figure className="book-artwork">
          <div className="book-artwork-mat">
            <BookImage photo={block.photo} sizes="(max-width: 700px) 90vw, 520px" />
          </div>
          <figcaption>
            <strong>{block.moment.text}</strong>
            {block.moment.context && <span>{block.moment.context}</span>}
            <When moment={block.moment} />
          </figcaption>
        </figure>
      );
    case "notes":
      return (
        <ul className="book-notes">
          {block.moments.map((moment, i) => (
            <li key={i}>
              <When moment={moment} />
              <p>{moment.text}</p>
            </li>
          ))}
        </ul>
      );
    case "facts":
      return (
        <ul className="book-facts">
          {block.lines.map(line => (
            <li key={line}>{line}</li>
          ))}
        </ul>
      );
    case "letter":
      return (
        <div className="book-letter">
          <p>{block.text}</p>
          {block.signature && <p className="book-letter-signature">{block.signature}</p>}
        </div>
      );
    case "growth":
      return (
        <div className="book-growth">
          {block.weight.length > 0 && (
            <GrowthChart label="Weight" points={block.weight} color="var(--weight-color)" />
          )}
          {block.height.length > 0 && (
            <GrowthChart label="Length" points={block.height} color="var(--height-color)" />
          )}
        </div>
      );
  }
};

const When = ({ moment }: { moment: BookMoment }) => (
  <p className="book-when">
    {shortDay(moment.day)}
    {moment.age && <span> · {moment.age}</span>}
  </p>
);

const MomentView = ({ moment, photo }: { moment: BookMoment; photo: BookPhoto | null }) => (
  <div className={photo ? "book-moment has-photo" : "book-moment"}>
    {photo && (
      <BookImage
        photo={photo}
        className="book-moment-photo"
        sizes="(max-width: 700px) 100vw, 420px"
      />
    )}
    <div className="book-moment-text">
      {moment.first && <p className="book-moment-label">A first</p>}
      <p className="book-moment-words">{moment.text}</p>
      {moment.context && <p className="book-moment-context">{moment.context}</p>}
      <When moment={moment} />
    </div>
  </div>
);

const PhotoFigure = ({
  photo,
  className,
  sizes,
}: {
  photo: BookPhoto;
  className?: string;
  sizes: string;
}) => (
  <figure className={`book-figure ${className ?? ""}`}>
    <BookImage photo={photo} sizes={sizes} />
    <figcaption>
      {photo.caption && <span className="book-caption">{photo.caption}</span>}
      <span className="book-caption-when">
        {shortDay(photo.day)}
        {photo.age && ` · ${photo.age}`}
      </span>
    </figcaption>
  </figure>
);

const BookImage = ({
  photo,
  className,
  sizes,
  eager,
}: {
  photo: BookPhoto;
  className?: string;
  sizes: string;
  eager?: boolean;
}) => {
  const ratio = `${photo.width || 4} / ${photo.height || 3}`;
  if (isPlaceholderPhoto(photo.id)) {
    const hue = (Math.abs(photo.id) * 47) % 360;
    return (
      <div
        className={`book-image book-placeholder ${className ?? ""}`}
        style={{ aspectRatio: ratio, "--hue": String(hue) }}
        role="img"
        aria-label={photo.caption || "Sample photo"}
      />
    );
  }
  return (
    <div className={`book-image ${className ?? ""}`} style={{ aspectRatio: ratio }}>
      <ResponsiveImage
        photoId={photo.id}
        alt={photo.caption || `Photo from ${shortDay(photo.day)}`}
        sizes={sizes}
        loading={eager ? "eager" : "lazy"}
        fetchpriority={eager ? "high" : "auto"}
      />
    </div>
  );
};

const GrowthChart = ({
  label,
  points,
  color,
}: {
  label: string;
  points: GrowthPoint[];
  color: string;
}) => {
  const w = 320;
  const h = 150;
  const pad = { l: 8, r: 8, t: 26, b: 22 };
  const values = points.map(p => p.value);
  const min = Math.min(...values);
  const max = Math.max(...values);
  const span = max - min || 1;
  const x = (m: number) => pad.l + (Math.min(12, m) / 12) * (w - pad.l - pad.r);
  const y = (v: number) => h - pad.b - ((v - min) / span) * (h - pad.t - pad.b);
  const path = points.map((p, i) => `${i ? "L" : "M"}${x(p.months)},${y(p.value)}`).join(" ");
  const first = points[0];
  const last = points[points.length - 1];
  const description = `${label}: ${first.label} on ${shortDay(first.day)}, ${last.label} on ${shortDay(last.day)}, from ${points.length} measurements.`;
  return (
    <figure className="book-growth-chart">
      <svg viewBox={`0 0 ${w} ${h}`} role="img" aria-label={description}>
        <line
          x1={pad.l}
          x2={w - pad.r}
          y1={h - pad.b}
          y2={h - pad.b}
          className="book-growth-axis"
        />
        <path d={path} fill="none" stroke={color} stroke-width="2.5" stroke-linejoin="round" />
        {points.map(p => (
          <circle key={p.day} cx={x(p.months)} cy={y(p.value)} r="3.5" fill={color} />
        ))}
        <text x={x(first.months) + 8} y={y(first.value) + 4} className="book-growth-label">
          {first.label}
        </text>
        <text
          x={x(last.months)}
          y={y(last.value) - 10}
          text-anchor="end"
          className="book-growth-label"
        >
          {last.label}
        </text>
        <text x={pad.l} y={h - 4} className="book-growth-tick">
          birth
        </text>
        <text x={w - pad.r} y={h - 4} text-anchor="end" className="book-growth-tick">
          one year
        </text>
      </svg>
      <figcaption>
        <strong>{label}</strong>
        <span>
          {first.label} on {shortDay(first.day)} → {last.label} on {shortDay(last.day)}
        </span>
      </figcaption>
    </figure>
  );
};
