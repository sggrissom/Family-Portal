export interface PdfPage {
  jpeg: Uint8Array;
  widthPx: number;
  heightPx: number;
}

export const letterPage = { width: 612, height: 792 };

/** A PDF with one full-page JPEG per page, at US Letter size. */
export function jpegPagesToPdf(pages: PdfPage[]): Uint8Array<ArrayBuffer> {
  const encoder = new TextEncoder();
  const chunks: Uint8Array[] = [];
  const offsets: number[] = [];
  let length = 0;

  const write = (part: string | Uint8Array) => {
    const bytes = typeof part === "string" ? encoder.encode(part) : part;
    chunks.push(bytes);
    length += bytes.length;
  };
  const object = (id: number, body: () => void) => {
    offsets[id] = length;
    write(`${id} 0 obj\n`);
    body();
    write("\nendobj\n");
  };

  const pageIds = pages.map((_, i) => 3 + i * 3);
  write("%PDF-1.4\n%\xE2\xE3\xCF\xD3\n");
  object(1, () => write("<< /Type /Catalog /Pages 2 0 R >>"));
  object(2, () =>
    write(
      `<< /Type /Pages /Kids [${pageIds.map(id => `${id} 0 R`).join(" ")}] /Count ${pages.length} >>`
    )
  );

  pages.forEach((page, i) => {
    const pageId = pageIds[i];
    const contentId = pageId + 1;
    const imageId = pageId + 2;
    const { width, height } = letterPage;
    const content = `q ${width} 0 0 ${height} 0 0 cm /Im0 Do Q`;
    object(pageId, () =>
      write(
        `<< /Type /Page /Parent 2 0 R /MediaBox [0 0 ${width} ${height}] ` +
          `/Resources << /XObject << /Im0 ${imageId} 0 R >> >> /Contents ${contentId} 0 R >>`
      )
    );
    object(contentId, () => {
      write(`<< /Length ${content.length} >>\nstream\n${content}\nendstream`);
    });
    object(imageId, () => {
      write(
        `<< /Type /XObject /Subtype /Image /Width ${page.widthPx} /Height ${page.heightPx} ` +
          `/ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /DCTDecode /Length ${page.jpeg.length} >>\nstream\n`
      );
      write(page.jpeg);
      write("\nendstream");
    });
  });

  const count = 3 + pages.length * 3;
  const xref = length;
  write(`xref\n0 ${count}\n0000000000 65535 f \n`);
  for (let id = 1; id < count; id++) {
    write(`${String(offsets[id]).padStart(10, "0")} 00000 n \n`);
  }
  write(`trailer\n<< /Size ${count} /Root 1 0 R >>\nstartxref\n${xref}\n%%EOF\n`);

  const out = new Uint8Array(new ArrayBuffer(length));
  let at = 0;
  for (const chunk of chunks) {
    out.set(chunk, at);
    at += chunk.length;
  }
  return out;
}
