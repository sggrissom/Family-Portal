import * as preact from "preact";
import * as rpc from "vlens/rpc";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { copy } from "../../lib/copy";
import "./recap-styles";

type Data = { recap: server.PublishedRecap | null };

/** The server's single answer for an unknown, expired or revoked token (ErrRecapUnavailable). */
const unavailableError = "This link isn't available";

/** The token is exactly the one segment after /recap/; anything under it is not a recap. */
export function recapToken(route: string): string {
  const path = route.split(/[?#]/)[0];
  const token = path.startsWith("/recap/") ? path.slice("/recap/".length) : "";
  return /^[0-9a-f]{32}$/.test(token) ? token : "";
}

export async function fetch(route: string, prefix: string): Promise<rpc.Response<Data>> {
  const token = recapToken(route);
  if (!token) return rpc.ok<Data>({ recap: null });
  const [resp, err] = await server.GetRecap({ token });
  if (resp) return rpc.ok<Data>({ recap: resp.recap });
  if (err === unavailableError) return rpc.ok<Data>({ recap: null });
  return [null, err || "Couldn't load these results"];
}

export function view(route: string, prefix: string, data: Data): preact.ComponentChild {
  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="recap-container">
        {data.recap ? <Recap recap={data.recap} /> : <Unavailable />}
      </main>
      <Footer />
    </div>
  );
}

const Unavailable = () => (
  <article className="recap-page">
    <h1>{copy.share.recapPage.unavailableTitle}</h1>
    <p className="recap-meta">{copy.share.recapPage.unavailable}</p>
  </article>
);

const Recap = ({ recap }: { recap: server.PublishedRecap }) => (
  <article className="recap-page">
    <header>
      <h1>{recap.eventName}</h1>
      {[recap.subtitle, recap.place].filter(Boolean).map(line => (
        <p key={line} className="recap-meta">
          {line}
        </p>
      ))}
      {recap.snapshotLabel && <p className="recap-snapshot">{recap.snapshotLabel}</p>}
    </header>
    {recap.routines.map((routine, i) => (
      <section key={i} className="recap-routine">
        <h2>{routine.name}</h2>
        {[routine.descriptors, routine.roster, routine.summary].filter(Boolean).map(line => (
          <p key={line} className="recap-meta">
            {line}
          </p>
        ))}
        {routine.performances.map((performance, j) => (
          <Performance key={j} performance={performance} />
        ))}
      </section>
    ))}
    <p className="recap-footer-note">{copy.share.recapPage.sharedFrom}</p>
  </article>
);

const Performance = ({ performance }: { performance: server.PublishedPerformance }) => {
  const heading = [performance.when, performance.matchup].filter(Boolean).join(" · ");
  const empty = performance.results.length === 0 && !performance.matchup;
  return (
    <div className="recap-performance">
      {heading && <h3>{heading}</h3>}
      <ul className="recap-results">
        {empty && <li className="recap-detail">{copy.share.recapPage.noResults}</li>}
        {performance.results.map((result, k) => (
          <li key={k}>
            <span className="recap-result">{result.text}</span>
            {result.detail && <span className="recap-detail"> ({result.detail})</span>}
          </li>
        ))}
      </ul>
    </div>
  );
};
