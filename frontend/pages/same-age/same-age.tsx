import * as preact from "preact";
import * as vlens from "vlens";
import * as core from "vlens/core";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView } from "../../lib/authHelpers";
import { SameAgeRows } from "../../components/SameAgeRows";
import { SameAgeMontage } from "../../components/SameAgeMontage";
import { localDateString } from "../../lib/when";
import { copy } from "../../lib/copy";
import { ageStep, ageTitle, nextAge, parseAgeParam, prevAge, sameAgePath } from "../../lib/sameAge";
import "./same-age-styles";

export async function fetch(route: string, prefix: string) {
  const params = new URLSearchParams(route.split("?")[1] ?? "");
  return server.GetSameAge({
    ageMonths: parseAgeParam(params.get("age")),
    fromPersonId: parseInt(params.get("from") ?? "") || 0,
    today: localDateString(new Date()),
  });
}

type SliderState = { value: number | null };
const useSlider = vlens.declareHook((): SliderState => ({ value: null }));

export function view(
  route: string,
  prefix: string,
  data: server.GetSameAgeResponse
): preact.ComponentChild {
  if (!requireAuthInView()) return;

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="same-age-container">
        <SameAgePage data={data} />
      </main>
      <Footer />
    </div>
  );
}

function goTo(data: server.GetSameAgeResponse, ageMonths: number) {
  core.replaceRoute(sameAgePath(ageMonths, data.fromPersonId));
}

function snap(ageMonths: number): number {
  const step = ageStep(ageMonths);
  return Math.round(ageMonths / step) * step;
}

function sliderInput(slider: SliderState, event: Event) {
  slider.value = snap(parseInt((event.target as HTMLInputElement).value));
  vlens.scheduleRedraw();
}

function sliderChange(slider: SliderState, data: server.GetSameAgeResponse) {
  if (slider.value !== null && slider.value !== data.ageMonths) goTo(data, slider.value);
}

const SameAgePage = ({ data }: { data: server.GetSameAgeResponse }) => {
  const slider = useSlider();
  const shown = slider.value ?? data.ageMonths;
  const today = localDateString(new Date());
  const max = Math.max(data.maxAgeMonths, data.ageMonths);

  return (
    <div className="same-age-page">
      <h1>{copy.sameAge.title}</h1>
      <div className="same-age-control">
        <span className="same-age-at">{copy.sameAge.at}</span>
        <button
          type="button"
          className="same-age-step"
          aria-label={copy.sameAge.younger}
          disabled={data.ageMonths <= 0}
          onClick={() => goTo(data, prevAge(data.ageMonths))}
        >
          ◀
        </button>
        <strong className="same-age-age" aria-live="polite">
          {ageTitle(shown)}
        </strong>
        <button
          type="button"
          className="same-age-step"
          aria-label={copy.sameAge.older}
          disabled={data.ageMonths >= max}
          onClick={() => goTo(data, nextAge(data.ageMonths, max))}
        >
          ▶
        </button>
      </div>
      <input
        type="range"
        className="same-age-slider"
        aria-label={copy.sameAge.title}
        min={0}
        max={max}
        value={shown}
        onInput={vlens.cachePartial(sliderInput, slider)}
        onChange={() => sliderChange(slider, data)}
      />

      {(data.rows ?? []).length === 0 ? (
        <p className="same-age-none">{copy.sameAge.empty}</p>
      ) : (
        <>
          <SameAgeMontage rows={data.rows} ageMonths={data.ageMonths} />
          <SameAgeRows rows={data.rows} ageMonths={data.ageMonths} today={today} />
        </>
      )}
    </div>
  );
};
