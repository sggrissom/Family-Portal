import * as preact from "preact";
import * as rpc from "vlens/rpc";
import * as auth from "../../lib/authCache";
import * as core from "vlens/core";
import { Header, Footer } from "../../layout";
import { ensureNoAuthInFetch } from "../../lib/authHelpers";
import "./landing-styles";

type Data = {};

export async function fetch(route: string, prefix: string) {
  if (!(await ensureNoAuthInFetch())) {
    return rpc.ok<Data>({});
  }

  return rpc.ok<Data>({});
}

export function view(route: string, prefix: string, data: Data): preact.ComponentChild {
  const currentAuth = auth.getAuth();
  if (currentAuth && currentAuth.id > 0) {
    core.setRoute("/dashboard");
    return null;
  }

  return (
    <div>
      <Header isHome={true} />
      <main id="app" className="landing-container">
        <LandingPage />
      </main>
      <Footer />
    </div>
  );
}

const Shot = ({ src, alt, caption }: { src: string; alt: string; caption: string }) => (
  <figure className="shot">
    <img className="shot-img" src={src} alt={alt} loading="lazy" />
    <figcaption>{caption}</figcaption>
  </figure>
);

const Actions = () => (
  <div className="intro-actions">
    <a href="/create-account" className="btn btn-primary">
      Create an account
    </a>
    <a href="/login" className="btn">
      Log in
    </a>
  </div>
);

const LandingPage = () => (
  <div className="landing-page">
    <section className="landing-intro">
      <h1>Family Record</h1>
      <p className="intro-lead">
        A private record of your family: who is in it, how the children are growing, what they have
        done, and the photographs that go with it.
      </p>
      <Actions />
    </section>

    <section className="landing-what">
      <h2>What you can keep here</h2>
      <ul>
        <li>Each family member, with birth dates and how everyone is related.</li>
        <li>Height and weight over time, in metric or imperial, charted per person or together.</li>
        <li>Milestones: first steps, a lost tooth, the first day of a school year.</li>
        <li>Photos with captions, dates, tags, and a note of who is in them.</li>
        <li>Sports seasons and dance years: the games, the teams, and how they placed.</li>
        <li>A message thread for the family, and a timeline of everything in date order.</li>
      </ul>
    </section>

    <section className="landing-shots">
      <Shot
        src="/images/screenshots/person.png"
        alt="A person's page"
        caption="Each person has a page gathering their milestones, measurements, and photos."
      />
      <Shot
        src="/images/screenshots/growth.png"
        alt="The family growth chart"
        caption="Growth charted by age, one child or several on the same axes."
      />
      <Shot
        src="/images/screenshots/timeline.png"
        alt="The family timeline"
        caption="The timeline puts everything in one list, filtered by person or by kind."
      />
    </section>

    <section className="landing-close">
      <Actions />
    </section>
  </div>
);
