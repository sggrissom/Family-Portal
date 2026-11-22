import {
  CardBlock,
  CardLine,
  LaidOutBlock,
  Measure,
  drawBlock,
  layoutBlock,
  muted,
  stackHeight,
} from "./cardLayout";
import { MeasurementShare } from "./measurementShare";
import { CompetitionShare, RecapPerformance, RecapRoutine } from "./competitionShare";
import { paginate } from "./pagination";
import { jpegPagesToPdf, letterPage } from "./pdf";

export const imageWidth = 420;
export const maxImageHeight = 1200;
const margin = 24;
const spacing = 12;
const pageMargin = 40;
const scale = 2;

const typeColor = { height: "#3b82f6", weight: "#ef4444" };
const tint = { height: "#eff6ff", weight: "#fef2f2" };

// MARK: - Blocks

export function measurementBlocks(share: MeasurementShare): CardBlock[] {
  const blocks: CardBlock[] = [
    {
      lines: [
        { text: share.personName, size: 26, weight: 700 },
        {
          text: [share.dateText, share.ageText].filter(Boolean).join(" · "),
          size: 15,
          color: muted,
          gapBefore: 2,
        },
      ],
    },
  ];
  for (const section of share.sections) {
    blocks.push({
      lines: [
        {
          text: section.label,
          size: 15,
          weight: 600,
          color: typeColor[section.type],
          gapBefore: 6,
        },
        { text: section.value, size: 34, weight: 700 },
      ],
    });
    let group: string | null = null;
    for (const row of section.rows) {
      if (row.group && row.group !== group) {
        blocks.push({
          lines: [{ text: row.group, size: 12, weight: 700, color: muted, uppercase: true }],
        });
      }
      group = row.group;
      const lines: CardLine[] = [{ text: row.heading, size: 15, weight: 600 }];
      for (const line of row.lines) {
        lines.push({ text: line.title, size: 15, gapBefore: 4 });
        if (line.detail) lines.push({ text: line.detail, size: 13, color: muted });
      }
      blocks.push({ lines, background: tint[section.type], padding: 12 });
    }
  }
  return blocks;
}

function headerBlock(share: CompetitionShare): CardBlock {
  const lines: CardLine[] = [{ text: share.eventName, size: 24, weight: 700 }];
  if (share.subtitle) lines.push({ text: share.subtitle, size: 14, color: muted, gapBefore: 2 });
  if (share.place) lines.push({ text: share.place, size: 14, color: muted });
  if (share.snapshotLabel) {
    lines.push({
      text: share.snapshotLabel,
      size: 13,
      weight: 700,
      color: "#b45309",
      gapBefore: 6,
    });
  }
  return { lines };
}

function routineLines(routine: RecapRoutine): CardLine[] {
  const lines: CardLine[] = [{ text: routine.name, size: 18, weight: 700 }];
  if (routine.descriptors) lines.push({ text: routine.descriptors, size: 13, color: muted });
  if (routine.roster) lines.push({ text: routine.roster, size: 13, color: muted });
  if (routine.summary) lines.push({ text: routine.summary, size: 15, weight: 600, gapBefore: 2 });
  return lines;
}

function performanceLines(performance: RecapPerformance): CardLine[] {
  const lines: CardLine[] = [];
  const heading = [performance.when, performance.matchup].filter(Boolean).join(" · ");
  if (heading) lines.push({ text: heading, size: 15, weight: 600, gapBefore: 6 });
  if (performance.results.length === 0 && !performance.matchup) {
    lines.push({ text: "No results recorded", size: 13, color: muted, gapBefore: 4 });
  }
  for (const result of performance.results) {
    lines.push({ text: `• ${result.text}`, size: 15, weight: 500, gapBefore: 4 });
    if (result.detail) lines.push({ text: result.detail, size: 13, color: muted, indent: 12 });
  }
  return lines;
}

/** The header, then one group per routine: its heading with the first performance, then each further performance. */
export function competitionGroups(share: CompetitionShare): CardBlock[][] {
  const groups: CardBlock[][] = [[headerBlock(share)]];
  for (const routine of share.routines) {
    const [first, ...rest] = routine.performances;
    groups.push([
      { rule: true, lines: [...routineLines(routine), ...(first ? performanceLines(first) : [])] },
      ...rest.map(performance => ({ lines: performanceLines(performance) })),
    ]);
  }
  return groups;
}

// MARK: - Output

export interface RenderedShare {
  file: File;
  previews: string[];
  kind: "image" | "pdf";
}

function canvasMeasure(): Measure {
  const ctx = document.createElement("canvas").getContext("2d")!;
  return (text, font) => {
    ctx.font = font;
    return ctx.measureText(text).width;
  };
}

function canvas(width: number, height: number): [HTMLCanvasElement, CanvasRenderingContext2D] {
  const element = document.createElement("canvas");
  element.width = Math.ceil(width * scale);
  element.height = Math.ceil(height * scale);
  const ctx = element.getContext("2d")!;
  ctx.scale(scale, scale);
  ctx.fillStyle = "#ffffff";
  ctx.fillRect(0, 0, width, height);
  return [element, ctx];
}

function drawStack(
  ctx: CanvasRenderingContext2D,
  blocks: LaidOutBlock[],
  x: number,
  y: number,
  width: number
) {
  let top = y;
  for (const block of blocks) {
    drawBlock(ctx, block, x, top, width);
    top += block.height + spacing;
  }
}

function toBlob(element: HTMLCanvasElement, type: string): Promise<Blob> {
  return new Promise((resolve, reject) =>
    element.toBlob(blob => (blob ? resolve(blob) : reject(new Error("render"))), type, 0.92)
  );
}

async function renderImage(
  blocks: CardBlock[],
  measure: Measure,
  name: string
): Promise<RenderedShare> {
  const inner = imageWidth - margin * 2;
  const laidOut = blocks.map(block => layoutBlock(block, inner, measure));
  const [element, ctx] = canvas(imageWidth, stackHeight(laidOut, spacing) + margin * 2);
  drawStack(ctx, laidOut, margin, margin, inner);
  const blob = await toBlob(element, "image/png");
  return {
    file: new File([blob], `${name}.png`, { type: "image/png" }),
    previews: [URL.createObjectURL(blob)],
    kind: "image",
  };
}

async function renderPdf(
  groups: CardBlock[][],
  measure: Measure,
  name: string
): Promise<RenderedShare> {
  const inner = letterPage.width - pageMargin * 2;
  const pageInner = letterPage.height - pageMargin * 2;
  const laidOut = groups.map(items => items.map(block => layoutBlock(block, inner, measure)));
  const plan = paginate(
    laidOut.map(items => items.map(item => item.height)),
    pageInner,
    spacing
  );

  const pages = [];
  const previews = [];
  for (const placements of plan) {
    const [element, ctx] = canvas(letterPage.width, letterPage.height);
    let top = pageMargin;
    for (const { group, item } of placements) {
      const block = laidOut[group][item];
      if (block.height > pageInner) {
        const shrink = pageInner / block.height;
        ctx.save();
        ctx.translate(pageMargin, top);
        ctx.scale(shrink, shrink);
        drawBlock(ctx, block, 0, 0, inner);
        ctx.restore();
        top += pageInner + spacing;
      } else {
        drawBlock(ctx, block, pageMargin, top, inner);
        top += block.height + spacing;
      }
    }
    const jpeg = await toBlob(element, "image/jpeg");
    pages.push({
      jpeg: new Uint8Array(await jpeg.arrayBuffer()),
      widthPx: element.width,
      heightPx: element.height,
    });
    previews.push(URL.createObjectURL(jpeg));
  }
  const pdf = jpegPagesToPdf(pages);
  return {
    file: new File([pdf], `${name}.pdf`, { type: "application/pdf" }),
    previews,
    kind: "pdf",
  };
}

function fileName(text: string): string {
  return text.replace(/[\\/:*?"<>|%]+/g, "-").trim() || "Share";
}

export function renderMeasurementShare(share: MeasurementShare): Promise<RenderedShare> {
  const types = share.sections.map(section => section.label).join(" and ");
  return renderImage(
    measurementBlocks(share),
    canvasMeasure(),
    fileName(`${share.personName} ${types}`)
  );
}

export function renderCompetitionShare(share: CompetitionShare): Promise<RenderedShare> {
  const measure = canvasMeasure();
  const groups = competitionGroups(share);
  const name = fileName(`${share.eventName} results`);
  const inner = imageWidth - margin * 2;
  const height = stackHeight(
    groups.flat().map(block => layoutBlock(block, inner, measure)),
    spacing
  );
  return height + margin * 2 <= maxImageHeight
    ? renderImage(groups.flat(), measure, name)
    : renderPdf(groups, measure, name);
}
