import * as preact from "preact";
import * as rpc from "vlens/rpc";
import { Header } from "./header"

type Data = {};

export async function fetch(route: string, prefix: string) {
  return rpc.ok<Data>({});
}

export function view(
  route: string,
  prefix: string,
  data: Data,
): preact.ComponentChild {
  return (
    <div>
      <Header isHome={true} />
      <main id="app" class="app">
        <Hero />
      </main>
      <Footer />
    </div>
  );
}

const Hero = () => (
  <section className="hero">
    <div>
      <h1>Family Portal</h1>
      <h2>Family Stuff</h2>
      <p>family.</p>
    </div>
  </section>
);

const Footer = () => (
  <footer className="site-footer">
    <p>
      © <span id="year">2025</span> Family Portal. All rights reserved.
    </p>
  </footer>
);
