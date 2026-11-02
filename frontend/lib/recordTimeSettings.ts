import * as vlens from "vlens";
import * as server from "../server";

let approximate: boolean | null = null;

export function approximateDatesEnabled(): boolean {
  if (approximate === null) {
    approximate = false;
    if (BROWSER) {
      server.GetRecordTimeSettings({}).then(([resp]) => {
        approximate = !!resp?.approximateDates;
        if (approximate) vlens.scheduleRedraw();
      });
    }
  }
  return approximate;
}
