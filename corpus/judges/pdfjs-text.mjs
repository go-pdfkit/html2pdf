// Extract the text of every page of a PDF with pdf.js (Firefox's engine) and
// print it — the same signal `pdftotext` gives for poppler, from a second,
// independent parser. Usage: node pdfjs-text.mjs file.pdf
import { readFileSync } from "node:fs";
import { getDocument } from "pdfjs-dist/legacy/build/pdf.mjs";

const data = new Uint8Array(readFileSync(process.argv[2]));
const doc = await getDocument({ data, useSystemFonts: true, disableFontFace: true, verbosity: 0 }).promise;
let out = "";
for (let i = 1; i <= doc.numPages; i++) {
  const page = await doc.getPage(i);
  const tc = await page.getTextContent();
  out += tc.items.map((it) => it.str).join(" ") + "\n";
}
process.stdout.write(`pages ${doc.numPages}\n`);
process.stdout.write(out);
