// Render one page of a PDF to PNG with pdf.js (Firefox's engine) on a native
// canvas — an independent raster judge next to poppler, MuPDF, Ghostscript,
// Quartz and pdfium. Usage: node pdfjs-render.mjs file.pdf out.png [page=1] [scale=1]
import { readFileSync, writeFileSync } from "node:fs";
import { createCanvas } from "@napi-rs/canvas";
import { getDocument } from "pdfjs-dist/legacy/build/pdf.mjs";

const [, , file, out, pageArg = "1", scaleArg = "1"] = process.argv;
const data = new Uint8Array(readFileSync(file));
const doc = await getDocument({ data, useSystemFonts: true, disableFontFace: true, verbosity: 0 }).promise;
const page = await doc.getPage(Number(pageArg));
const vp = page.getViewport({ scale: Number(scaleArg) });
const canvas = createCanvas(Math.ceil(vp.width), Math.ceil(vp.height));
const ctx = canvas.getContext("2d");
ctx.fillStyle = "#fff";
ctx.fillRect(0, 0, canvas.width, canvas.height);
await page.render({ canvasContext: ctx, viewport: vp }).promise;
writeFileSync(out, canvas.toBuffer("image/png"));
process.stdout.write(`pages ${doc.numPages}\n`);
