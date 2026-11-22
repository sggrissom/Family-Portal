export interface CardLine {
  text: string;
  size: number;
  weight?: number;
  color?: string;
  indent?: number;
  gapBefore?: number;
  uppercase?: boolean;
}

export interface CardBlock {
  lines: CardLine[];
  background?: string;
  padding?: number;
  rule?: boolean;
}

export type Measure = (text: string, font: string) => number;

export interface LaidOutLine {
  line: CardLine;
  rows: string[];
}

export interface LaidOutBlock {
  block: CardBlock;
  lines: LaidOutLine[];
  height: number;
}

export const ink = "#111827";
export const muted = "#6b7280";
const ruleGap = 12;

export function fontFor(line: CardLine): string {
  return `${line.weight ?? 400} ${line.size}px system-ui, -apple-system, "Segoe UI", Roboto, sans-serif`;
}

export function lineHeight(line: CardLine): number {
  return Math.round(line.size * 1.3);
}

/** Word-wraps `text` to `width`, breaking a word only when it is wider than a whole row. */
export function wrap(text: string, width: number, font: string, measure: Measure): string[] {
  const rows: string[] = [];
  let row = "";
  for (const word of text.split(/\s+/).filter(Boolean)) {
    const candidate = row ? `${row} ${word}` : word;
    if (measure(candidate, font) <= width) {
      row = candidate;
      continue;
    }
    if (row) rows.push(row);
    row = word;
    while (measure(row, font) > width && row.length > 1) {
      let cut = row.length - 1;
      while (cut > 1 && measure(row.slice(0, cut), font) > width) cut--;
      rows.push(row.slice(0, cut));
      row = row.slice(cut);
    }
  }
  if (row || rows.length === 0) rows.push(row);
  return rows;
}

export function layoutBlock(block: CardBlock, width: number, measure: Measure): LaidOutBlock {
  const padding = block.padding ?? 0;
  const inner = width - padding * 2;
  let height = padding * 2 + (block.rule ? ruleGap : 0);
  const lines = block.lines.map(line => {
    const text = line.uppercase ? line.text.toUpperCase() : line.text;
    const rows = wrap(text, inner - (line.indent ?? 0), fontFor(line), measure);
    height += (line.gapBefore ?? 0) + rows.length * lineHeight(line);
    return { line, rows };
  });
  return { block, lines, height };
}

export function stackHeight(blocks: LaidOutBlock[], spacing: number): number {
  if (blocks.length === 0) return 0;
  return blocks.reduce((sum, block) => sum + block.height, 0) + spacing * (blocks.length - 1);
}

export function drawBlock(
  ctx: CanvasRenderingContext2D,
  laidOut: LaidOutBlock,
  x: number,
  y: number,
  width: number
) {
  const { block } = laidOut;
  const padding = block.padding ?? 0;
  if (block.background) {
    ctx.fillStyle = block.background;
    roundedRect(ctx, x, y, width, laidOut.height, 10);
    ctx.fill();
  }
  let top = y + padding;
  if (block.rule) {
    ctx.fillStyle = "#e5e7eb";
    ctx.fillRect(x, top, width, 1);
    top += ruleGap;
  }
  ctx.textBaseline = "top";
  for (const { line, rows } of laidOut.lines) {
    top += line.gapBefore ?? 0;
    ctx.font = fontFor(line);
    ctx.fillStyle = line.color ?? ink;
    const height = lineHeight(line);
    for (const row of rows) {
      ctx.fillText(row, x + padding + (line.indent ?? 0), top + (height - line.size) / 2);
      top += height;
    }
  }
}

function roundedRect(
  ctx: CanvasRenderingContext2D,
  x: number,
  y: number,
  w: number,
  h: number,
  r: number
) {
  ctx.beginPath();
  ctx.moveTo(x + r, y);
  ctx.arcTo(x + w, y, x + w, y + h, r);
  ctx.arcTo(x + w, y + h, x, y + h, r);
  ctx.arcTo(x, y + h, x, y, r);
  ctx.arcTo(x, y, x + w, y, r);
  ctx.closePath();
}
