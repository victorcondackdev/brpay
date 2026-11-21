// brpay playground: wires the WebAssembly build (brpay.wasm) to the page.
// The wasm module registers brpayDecode and brpayPixStatic, both of which
// take and return strings (JSON); see wasm/main.go.
"use strict";

const EXAMPLES = {
  pix: "00020126580014br.gov.bcb.pix0136123e4567-e12b-12d1-a456-4266554400005204000053039865802BR5913Fulano de Tal6008BRASILIA62070503***63041D3D",
  boleto: "34191.09016 00000.012344 56789.000017 9 16560000015990",
  typo: "00190.00009 00000.500000 00000.000075 9 10000000010000",
  broken: "00020126330014br.gov.bcb.pix0111529982247255204000053039865405" +
    "10,005802BR5906Fulano6008Niterói62070503***63041D3D",
  cnpj: "12.ABC.345/01DE-35",
};

const KIND_LABELS = {
  pix: "Pix copia e cola",
  boleto: "Boleto",
  document: "Documento",
  pix_key: "Chave Pix",
  end_to_end_id: "EndToEndId",
  unknown: "Não reconhecido",
};

const KEY_TYPES = { cpf: "CPF", cnpj: "CNPJ", email: "E-mail", phone: "Celular", evp: "Aleatória (EVP)" };

const $ = (id) => document.getElementById(id);

// ---- formatting helpers -------------------------------------------------

const brl = (cents) =>
  (cents / 100).toLocaleString("pt-BR", { style: "currency", currency: "BRL" });

function isoToBR(date) {
  const [y, m, d] = date.split("-");
  return `${d}/${m}/${y}`;
}

function el(tag, attrs = {}, ...children) {
  const node = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs)) {
    if (k === "class") node.className = v;
    else node.setAttribute(k, v);
  }
  for (const c of children) node.append(c);
  return node;
}

// ---- rows per kind ------------------------------------------------------
// Each entry is [label, value, mono?]; empty values are skipped.

function pixRows(p) {
  const rows = [
    ["Tipo de QR", p.initiation === "static" ? "Estático (dados no próprio código)" : "Dinâmico (dados buscados no PSP)"],
    ["Uso", p.reusable ? "Reutilizável" : "Uso único"],
    ["Chave", p.key, true],
    ["Tipo de chave", KEY_TYPES[p.key_type] || (p.key ? "Não reconhecido" : "")],
    ["URL do payload", p.url && "https://" + p.url, true],
    ["Recebedor", p.merchant_name],
    ["Cidade", p.merchant_city],
    ["CEP", p.postal_code],
    ["Valor", p.amount ? brl(p.amount_cents) : "Aberto (o pagador escolhe)"],
    ["Identificador (txid)", p.txid, true],
    ["Descrição", p.description],
    ["Categoria (MCC)", p.mcc, true],
    ["Moeda", p.currency === "986" ? "986 (real)" : p.currency],
    ["CRC", p.crc, true],
  ];
  for (const [id, gui] of Object.entries(p.templates || {})) rows.push([`Template ${id}`, gui || "(sem GUI)", true]);
  return rows;
}

function boletoRows(b) {
  if (b.type === "bancario") {
    return [
      ["Tipo", "Bancário (boleto de cobrança)"],
      ["Linha digitável", b.formatted, true],
      ["Código de barras", b.barcode, true],
      ["Banco", [b.bank_code, b.bank_name].filter(Boolean).join(" – ")],
      ["Valor", b.amount_cents ? brl(b.amount_cents) : "Não informado no código"],
      ["Vencimento", b.due_date ? isoToBR(b.due_date) : "Sem vencimento"],
      ["Fator de vencimento", b.factor ? String(b.factor) : ""],
      ["Campo livre", b.free_field, true],
    ];
  }
  return [
    ["Tipo", "Arrecadação (convênio)"],
    ["Linha digitável", b.formatted, true],
    ["Código de barras", b.barcode, true],
    ["Segmento", `${b.segment} – ${b.segment_name || "não definido"}`],
    [b.value_is_reference ? "Valor de referência" : "Valor",
      b.value_is_reference ? String(b.amount_cents) : brl(b.amount_cents)],
    ["Dígito verificador", `Módulo ${b.modulus}`],
    ["Empresa / órgão", b.company_id, true],
    ["CNPJ (base)", b.cnpj_base, true],
    ["Campo livre", b.free_field, true],
  ];
}

function documentRows(d) {
  return [
    ["Tipo", d.type.toUpperCase()],
    ["Formatado", d.formatted, true],
    ["Somente caracteres", d.number, true],
    ["Mascarado (para logs)", d.masked, true],
    ["Formato", d.type === "cnpj" ? (d.alphanumeric ? "Alfanumérico (a partir de 2026)" : "Numérico") : ""],
  ];
}

const keyRows = (k) => [["Tipo", KEY_TYPES[k.type] || k.type], ["Chave normalizada", k.key, true]];

function e2eRows(e) {
  const t = new Date(e.time);
  return [
    ["ISPB do PSP", e.ispb, true],
    ["Data e hora", t.toLocaleString("pt-BR", { timeZone: "UTC" }) + " UTC"],
    ["Sequencial", e.sequence, true],
  ];
}

// ---- rendering ----------------------------------------------------------

function renderRows(rows) {
  const body = $("fields").tBodies[0];
  body.replaceChildren();
  for (const [label, value, mono] of rows) {
    if (!value) continue;
    body.append(el("tr", {}, el("th", { scope: "row" }, label), el("td", mono ? { class: "mono" } : {}, value)));
  }
  $("fields").hidden = body.rows.length === 0;
}

function renderIssues(issues = []) {
  const list = $("issues");
  list.replaceChildren(...issues.map((is) =>
    el("li", { class: is.severity },
      el("span", { class: "sev" }, is.severity === "error" ? "Erro" : "Aviso"),
      is.field ? el("span", { class: "field-id" }, `campo ${is.field}`) : "",
      is.message)));
  $("issues-box").hidden = issues.length === 0;
}

// Highlights the digits of a suggested line that differ from what was typed.
function highlighted(formatted, line, typedDigits) {
  const code = el("code");
  let i = 0;
  for (const ch of formatted) {
    if (/\d/.test(ch)) {
      code.append(line[i] !== typedDigits[i] ? el("mark", {}, ch) : ch);
      i++;
    } else {
      code.append(ch);
    }
  }
  return code;
}

function renderSuggestions(suggestions = [], input) {
  const typed = input.replace(/\D/g, "");
  $("suggestions").replaceChildren(...suggestions.map((s) => {
    const use = el("button", { type: "button" }, "Usar esta");
    use.addEventListener("click", () => setInput(s.formatted));
    return el("li", {}, highlighted(s.formatted, s.line, typed), use);
  }));
  $("suggestions-box").hidden = suggestions.length === 0;
}

function render(input) {
  const status = $("status");
  if (!input.trim()) {
    $("result").hidden = true;
    status.textContent = "";
    return;
  }
  const r = JSON.parse(globalThis.brpayDecode(input));
  status.textContent = "";
  $("result").hidden = false;

  let badge = KIND_LABELS[r.kind] || r.kind;
  if (r.document) badge = r.document.type.toUpperCase();
  $("kind").textContent = badge;

  const verdict = $("verdict");
  verdict.textContent = r.valid ? "Válido" : "Inválido";
  verdict.className = "verdict " + (r.valid ? "ok" : "bad");

  const err = $("error");
  err.hidden = !r.error;
  err.textContent = r.error || "";

  let rows = [];
  if (r.pix) rows = pixRows(r.pix);
  else if (r.boleto) rows = boletoRows(r.boleto);
  else if (r.document) rows = documentRows(r.document);
  else if (r.pix_key) rows = keyRows(r.pix_key);
  else if (r.end_to_end_id) rows = e2eRows(r.end_to_end_id);
  renderRows(rows);
  renderIssues(r.issues);
  renderSuggestions(r.suggestions, input);
}

function setInput(value) {
  const input = $("input");
  input.value = value;
  render(value);
  input.focus();
}

// ---- static Pix form ----------------------------------------------------

function onGenerate(event) {
  event.preventDefault();
  const options = Object.fromEntries(new FormData(event.target));
  const res = JSON.parse(globalThis.brpayPixStatic(JSON.stringify(options)));
  $("static-out").hidden = false;
  $("static-error").hidden = !res.error;
  $("static-error").textContent = res.error || "";
  $("static-ok").hidden = !res.payload;
  $("payload").value = res.payload || "";
}

async function copyPayload() {
  const area = $("payload");
  try {
    await navigator.clipboard.writeText(area.value);
  } catch {
    area.select();
    document.execCommand("copy");
  }
  const btn = $("copy");
  btn.textContent = "Copiado";
  setTimeout(() => { btn.textContent = "Copiar"; }, 1500);
}

// ---- boot ---------------------------------------------------------------

async function loadWasm() {
  const go = new Go();
  const ready = new Promise((resolve) => { globalThis.brpayReady = resolve; });
  const url = "brpay.wasm";
  let result;
  try {
    result = await WebAssembly.instantiateStreaming(fetch(url), go.importObject);
  } catch {
    // Some servers send the wrong MIME type for .wasm; fall back.
    const bytes = await (await fetch(url)).arrayBuffer();
    result = await WebAssembly.instantiate(bytes, go.importObject);
  }
  go.run(result.instance);
  await ready;
}

function init() {
  const input = $("input");
  let timer;
  input.addEventListener("input", () => {
    clearTimeout(timer);
    timer = setTimeout(() => render(input.value), 80);
  });
  for (const btn of document.querySelectorAll("[data-example]")) {
    btn.addEventListener("click", () => setInput(EXAMPLES[btn.dataset.example]));
  }
  $("static-form").addEventListener("submit", onGenerate);
  $("copy").addEventListener("click", copyPayload);
  $("inspect").addEventListener("click", () => {
    setInput($("payload").value);
    window.scrollTo({ top: 0, behavior: "smooth" });
  });

  loadWasm().then(() => {
    input.disabled = false;
    document.querySelector("#static-form button[type=submit]").disabled = false;
    $("status").textContent = "";
    if (input.value) render(input.value);
    input.focus();
  }).catch((err) => {
    $("status").textContent = "Não foi possível carregar o WebAssembly: " + err.message;
  });
}

init();
