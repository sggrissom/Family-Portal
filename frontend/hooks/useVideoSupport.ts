import * as vlens from "vlens";
import * as server from "../server";
import { noVideo } from "../lib/mediaUpload";

// Whether a family takes videos, fetched once per family and filled in when it
// arrives; until then it reads as photos only.
export const useVideoSupport = vlens.declareHook(
  (familyId: number): server.GetVideoSupportResponse => {
    const support = { ...noVideo };
    server.GetVideoSupport({ familyId }).then(([resp]) => {
      if (!resp) return;
      Object.assign(support, resp);
      vlens.scheduleRedraw();
    });
    return support;
  }
);
