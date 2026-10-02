import * as preact from "preact";
import * as vlens from "vlens";
import * as core from "vlens/core";
import * as server from "../../server";
import { Header, Footer } from "../../layout";
import { requireAuthInView } from "../../lib/authHelpers";
import { getIdFromRoute } from "../../lib/routeHelpers";
import { copy } from "../../lib/copy";
import { When, whenProblem, whenRequest } from "../../lib/when";
import {
  CheckupEntry,
  EDIT_WEIGHT_UNITS,
  entryForRecord,
  heightValue,
  weightValue,
} from "../../lib/checkup";
import { ErrorPage, ViewOnlyPage } from "../../components/ErrorPage";
import { canContribute } from "../../lib/authCache";
import { HeightField, WeightField } from "../../components/MeasurementFields";
import { SegmentedControl } from "../../components/SegmentedControl";
import { WhenControl } from "../../components/WhenControl";
import "../../components/entry-form-styles";

type MeasurementKind = "height" | "weight";

const KINDS: { value: MeasurementKind; label: string }[] = [
  { value: "height", label: copy.measurement.height },
  { value: "weight", label: copy.measurement.weight },
];

type EditGrowthForm = {
  kind: MeasurementKind;
  entry: CheckupEntry;
  when: When;
  error: string;
  saving: boolean;
};

const useEditGrowthForm = vlens.declareHook(
  (record: server.GrowthData): EditGrowthForm => ({
    kind: record.measurementType === server.Height ? "height" : "weight",
    entry: entryForRecord(record),
    when: { mode: "date", date: record.measurementDate.split("T")[0], ageYears: "", ageMonths: "" },
    error: "",
    saving: false,
  })
);

export async function fetch(route: string, prefix: string) {
  const growthId = getIdFromRoute(route);

  if (!growthId) {
    throw new Error("Growth record ID is required");
  }

  return server.GetGrowthData({ id: growthId });
}

export function view(
  route: string,
  prefix: string,
  data: server.GetGrowthDataResponse
): preact.ComponentChild {
  const currentAuth = requireAuthInView();
  if (!currentAuth) {
    return;
  }

  if (!data.growthData) {
    return (
      <ErrorPage
        title="Growth Record Not Found"
        message="The growth record you're trying to edit could not be found"
        containerClass="entry-container"
      />
    );
  }

  if (!canContribute(data.growthData.familyId)) {
    return <ViewOnlyPage backLink={`/view-growth/${data.growthData.id}`} />;
  }

  const form = useEditGrowthForm(data.growthData);

  return (
    <div>
      <Header isHome={false} />
      <main id="app" className="entry-container">
        <EditGrowthPage form={form} record={data.growthData} />
      </main>
      <Footer />
    </div>
  );
}

function setKind(form: EditGrowthForm, kind: MeasurementKind) {
  form.kind = kind;
  form.error = "";
  vlens.scheduleRedraw();
}

async function save(form: EditGrowthForm, record: server.GrowthData, event: Event) {
  event.preventDefault();
  if (form.saving) return;

  const parsed = form.kind === "height" ? heightValue(form.entry) : weightValue(form.entry);
  const problem =
    parsed.error || (parsed.value ? "" : `Enter a ${form.kind}`) || whenProblem(form.when);
  if (problem || !parsed.value) {
    form.error = problem;
    vlens.scheduleRedraw();
    return;
  }

  form.saving = true;
  form.error = "";
  vlens.scheduleRedraw();

  const when = whenRequest(form.when, new Date());
  const [resp, err] = await server.UpdateGrowthData({
    id: record.id,
    measurementType: form.kind,
    value: parsed.value.value,
    unit: parsed.value.unit,
    inputType: when.inputType,
    measurementDate: when.date,
    ageYears: when.ageYears,
    ageMonths: when.ageMonths,
  });
  if (!resp) {
    form.error = err || "That measurement could not be saved.";
    form.saving = false;
    vlens.scheduleRedraw();
    return;
  }
  core.setRoute(`/profile/${record.personId}`);
}

const EditGrowthPage = ({ form, record }: { form: EditGrowthForm; record: server.GrowthData }) => {
  const disabled = form.saving;
  return (
    <div className="entry-card">
      <h1 className="entry-title">Edit measurement</h1>

      <form className="entry-form" onSubmit={vlens.cachePartial(save, form, record)} noValidate>
        <div className="entry-subject">
          <SegmentedControl
            label="Measurement type"
            options={KINDS}
            value={form.kind}
            onChange={vlens.cachePartial(setKind, form)}
            disabled={disabled}
          />
          <WhenControl when={form.when} disabled={disabled} />
        </div>

        {form.error && (
          <div className="error-message" role="alert">
            {form.error}
          </div>
        )}

        {form.kind === "height" ? (
          <HeightField entry={form.entry} disabled={disabled} />
        ) : (
          <WeightField entry={form.entry} disabled={disabled} units={EDIT_WEIGHT_UNITS} />
        )}

        <div className="entry-actions">
          <a href={`/profile/${record.personId}`} className="btn btn-secondary">
            {copy.measurement.cancel}
          </a>
          <button type="submit" className="btn btn-primary" disabled={disabled}>
            {form.saving ? copy.measurement.saving : copy.measurement.save}
          </button>
        </div>
      </form>
    </div>
  );
};
