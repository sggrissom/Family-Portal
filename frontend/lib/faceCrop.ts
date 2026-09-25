import type * as server from "../server";

export type FaceCropLayout = {
  width: string;
  height: string;
  left: string;
  top: string;
};

const pct = (n: number) => `${(n * 100).toFixed(3)}%`;

export function hasFaceBox(box: server.FaceBox | null | undefined): box is server.FaceBox {
  return !!box && box.right > box.left && box.bottom > box.top;
}

export function faceCropLayout(box: server.FaceBox, padding = 0.35): FaceCropLayout {
  const w = box.right - box.left;
  const h = box.bottom - box.top;
  const left = box.left - w * padding;
  const top = box.top - h * padding;
  const spanW = w * (1 + 2 * padding);
  const spanH = h * (1 + 2 * padding);
  return {
    width: pct(1 / spanW),
    height: pct(1 / spanH),
    left: pct(-left / spanW),
    top: pct(-top / spanH),
  };
}
