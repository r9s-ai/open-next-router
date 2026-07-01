const providerInput = document.getElementById("provider");
const providerSelect = document.getElementById("providerSelect");
const contentEl = document.getElementById("content");
const editorIssuesEl = document.getElementById("editorIssues");
const statusEl = document.getElementById("status");
const onrBaseUrlEl = document.getElementById("onrBaseUrl");
const onrKEl = document.getElementById("onrK");
const onrUKEl = document.getElementById("onrUK");
const testApiEl = document.getElementById("testApi");
const testModelEl = document.getElementById("testModel");
const curlOutputEl = document.getElementById("curlOutput");
const execOutputEl = document.getElementById("execOutput");
const requestIdInputEl = document.getElementById("requestIdInput");
const dumpOutputEl = document.getElementById("dumpOutput");
const semanticClassByType = {
  keyword: "tok-keyword",
  string: "tok-string",
  number: "tok-number",
  comment: "tok-comment",
  operator: "tok-operator",
  namespace: "tok-namespace",
  property: "tok-property",
  enumMember: "tok-enumMember"
};
let editorAnalysisTimer = 0;
let editorAnalysisSeq = 0;
let semanticMarks = [];
let diagnosticMarks = [];
let diagnosticLineClasses = [];
let editorHoverTimer = 0;
let editorHoverSeq = 0;
let editorHoverEl = null;
let editorHoverMark = null;

const editor = window.CodeMirror ? window.CodeMirror.fromTextArea(contentEl, {
  lineNumbers: true,
  lineWrapping: false,
  indentUnit: 2,
  tabSize: 2,
  viewportMargin: Infinity
}) : null;

if (editor) {
  editor.setSize(null, 460);
  editor.on("change", () => {
    queueEditorAnalysis();
    hideEditorHover();
  });
  editor.on("scroll", hideEditorHover);
  const wrapper = editor.getWrapperElement();
  wrapper.addEventListener("mousemove", handleEditorMouseMove);
  wrapper.addEventListener("mouseleave", hideEditorHover);
}

function setStatus(obj) {
  statusEl.textContent = typeof obj === "string" ? obj : JSON.stringify(obj, null, 2);
}

function currentProvider() {
  return (providerInput.value || "").trim().toLowerCase();
}

function escapeHTML(v) {
  return String(v || "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}

function editorValue() {
  return editor ? editor.getValue() : (contentEl.value || "");
}

function setEditorValue(value) {
  const next = String(value || "");
  if (editor) {
    if (editor.getValue() !== next) {
      editor.setValue(next);
    }
    return;
  }
  contentEl.value = next;
}

function queueEditorAnalysis() {
  clearTimeout(editorAnalysisTimer);
  editorAnalysisTimer = setTimeout(runEditorAnalysis, 350);
}

async function runEditorAnalysis() {
  const provider = currentProvider();
  const content = editorValue();
  if (!provider) {
    renderDiagnostics([]);
    clearSemanticMarks();
    return;
  }
  const seq = ++editorAnalysisSeq;
  try {
    const [diagnostics, semantic] = await Promise.all([
      fetchEditorDiagnostics(provider, content),
      fetchEditorSemanticTokens(provider, content)
    ]);
    if (seq !== editorAnalysisSeq) {
      return;
    }
    renderDiagnostics(diagnostics.diagnostics || []);
    renderSemanticTokens(content, semantic.legend || {}, semantic.tokens || {});
  } catch (err) {
    if (seq !== editorAnalysisSeq) {
      return;
    }
    clearSemanticMarks();
    editorIssuesEl.innerHTML = `<div class="issue">${escapeHTML(String(err))}</div>`;
  }
}

async function fetchEditorDiagnostics(provider, content) {
  const res = await fetch("/api/editor/diagnostics", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ provider, content })
  });
  const data = await res.json();
  if (!res.ok || !data.ok) {
    throw new Error(data.error || "diagnostics failed");
  }
  return data;
}

async function fetchEditorSemanticTokens(provider, content) {
  const res = await fetch("/api/editor/semantic-tokens", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ provider, content })
  });
  const data = await res.json();
  if (!res.ok || !data.ok) {
    throw new Error(data.error || "semantic tokens failed");
  }
  return data;
}

async function fetchEditorHover(provider, content, position) {
  const res = await fetch("/api/editor/hover", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ provider, content, position })
  });
  const data = await res.json();
  if (!res.ok || !data.ok) {
    throw new Error(data.error || "hover failed");
  }
  return data;
}

function renderDiagnostics(diagnostics) {
  const list = Array.isArray(diagnostics) ? diagnostics : [];
  clearDiagnosticMarks();
  if (list.length === 0) {
    editorIssuesEl.innerHTML = `<div class="issue clean">No editor diagnostics.</div>`;
    return;
  }
  markDiagnostics(list);
  editorIssuesEl.innerHTML = list.slice(0, 8).map((diag) => {
    const line = Number(diag.range?.start?.line || 0) + 1;
    const col = Number(diag.range?.start?.character || 0) + 1;
    return `<div class="issue">${escapeHTML(`${line}:${col} ${diag.message || ""}`)}</div>`;
  }).join("");
}

function clearDiagnosticMarks() {
  for (const mark of diagnosticMarks) {
    mark.clear();
  }
  diagnosticMarks = [];
  if (!editor) {
    return;
  }
  for (const line of diagnosticLineClasses) {
    editor.removeLineClass(line, "background", "cm-diagnostic-line");
  }
  diagnosticLineClasses = [];
}

function markDiagnostics(diagnostics) {
  if (!editor) {
    return;
  }
  const lineCount = editor.lineCount();
  for (const diag of diagnostics) {
    const startLine = Math.max(0, Math.min(Number(diag.range?.start?.line || 0), lineCount - 1));
    const startCh = Math.max(0, Math.min(Number(diag.range?.start?.character || 0), editor.getLine(startLine).length));
    const endLineRaw = Number(diag.range?.end?.line || startLine);
    const endChRaw = Number(diag.range?.end?.character || startCh + 1);
    const endLine = Math.max(startLine, Math.min(endLineRaw, lineCount - 1));
    const maxEndCh = editor.getLine(endLine).length;
    let endCh = Math.max(0, Math.min(endChRaw, maxEndCh));
    if (endLine === startLine && endCh <= startCh) {
      endCh = Math.min(startCh + 1, maxEndCh);
    }
    editor.addLineClass(startLine, "background", "cm-diagnostic-line");
    diagnosticLineClasses.push(startLine);
    if (endLine > startLine || endCh > startCh) {
      diagnosticMarks.push(editor.markText(
        { line: startLine, ch: startCh },
        { line: endLine, ch: endCh },
        { className: "cm-diagnostic-token", title: String(diag.message || "") }
      ));
    }
  }
}

function decodeSemanticSpans(data, legend) {
  const tokenTypes = Array.isArray(legend.tokenTypes) ? legend.tokenTypes : [];
  const spans = [];
  let line = 0;
  let start = 0;
  for (let i = 0; i + 4 < data.length; i += 5) {
    line += Number(data[i] || 0);
    start = Number(data[i] || 0) === 0 ? start + Number(data[i + 1] || 0) : Number(data[i + 1] || 0);
    const length = Number(data[i + 2] || 0);
    const typeName = tokenTypes[Number(data[i + 3] || 0)] || "";
    if (length > 0 && line >= 0 && start >= 0) {
      spans.push({ line, start, length, typeName });
    }
  }
  return spans;
}

function clearSemanticMarks() {
  for (const mark of semanticMarks) {
    mark.clear();
  }
  semanticMarks = [];
}

function renderSemanticTokens(text, legend, tokens) {
  clearSemanticMarks();
  if (!editor) {
    return;
  }
  const data = Array.isArray(tokens.data) ? tokens.data : [];
  if (data.length === 0) {
    return;
  }
  const spans = decodeSemanticSpans(data, legend);
  const lineCount = editor.lineCount();
  for (const span of spans) {
    const cls = semanticClassByType[span.typeName] || "";
    if (!cls || span.line < 0 || span.line >= lineCount) {
      continue;
    }
    const lineLength = editor.getLine(span.line).length;
    const fromCh = Math.max(0, Math.min(span.start, lineLength));
    const toCh = Math.max(fromCh, Math.min(span.start + span.length, lineLength));
    if (toCh <= fromCh) {
      continue;
    }
    semanticMarks.push(editor.markText(
      { line: span.line, ch: fromCh },
      { line: span.line, ch: toCh },
      { className: cls }
    ));
  }
}

function handleEditorMouseMove(ev) {
  if (!editor) {
    return;
  }
  const provider = currentProvider();
  if (!provider) {
    hideEditorHover();
    return;
  }
  const pos = editor.coordsChar({ left: ev.clientX, top: ev.clientY }, "client");
  const lineNo = Math.max(0, Math.min(pos.line, editor.lineCount() - 1));
  const line = editor.getLine(lineNo) || "";
  const normalizedPos = {
    line: lineNo,
    character: Math.max(0, Math.min(pos.ch, line.length))
  };
  const mouse = { x: ev.clientX, y: ev.clientY };
  const seq = ++editorHoverSeq;
  clearTimeout(editorHoverTimer);
  editorHoverTimer = setTimeout(() => runEditorHover(seq, normalizedPos, mouse), 180);
}

async function runEditorHover(seq, position, mouse) {
  const provider = currentProvider();
  if (!provider) {
    hideEditorHover();
    return;
  }
  try {
    const data = await fetchEditorHover(provider, editorValue(), position);
    if (seq !== editorHoverSeq) {
      return;
    }
    if (!data.hover) {
      hideEditorHover({ keepSeq: true });
      return;
    }
    showEditorHover(data.hover, mouse);
  } catch (_err) {
    if (seq === editorHoverSeq) {
      hideEditorHover({ keepSeq: true });
    }
  }
}

function showEditorHover(hover, mouse) {
  clearEditorHoverMark();
  if (editor && hover.range) {
    editorHoverMark = editor.markText(
      { line: hover.range.start.line, ch: hover.range.start.character },
      { line: hover.range.end.line, ch: hover.range.end.character },
      { className: "cm-hover-token" }
    );
  }
  if (!editorHoverEl) {
    editorHoverEl = document.createElement("div");
    editorHoverEl.className = "editor-hover";
    document.body.appendChild(editorHoverEl);
  }
  editorHoverEl.innerHTML = renderHoverMarkdown(hover.contents?.value || "");
  editorHoverEl.style.display = "block";
  positionEditorHover(mouse.x, mouse.y);
}

function positionEditorHover(x, y) {
  if (!editorHoverEl) {
    return;
  }
  let left = x + 12;
  let top = y + 16;
  const rect = editorHoverEl.getBoundingClientRect();
  const margin = 8;
  if (left + rect.width > window.innerWidth - margin) {
    left = Math.max(margin, x - rect.width - 12);
  }
  if (top + rect.height > window.innerHeight - margin) {
    top = Math.max(margin, y - rect.height - 12);
  }
  editorHoverEl.style.left = `${left}px`;
  editorHoverEl.style.top = `${top}px`;
}

function renderHoverMarkdown(value) {
  const parts = escapeHTML(value).split(/\n{2,}/).filter((part) => part.trim() !== "");
  if (parts.length === 0) {
    return "";
  }
  return parts.map((part) => {
    const html = part
      .replace(/`([^`]+)`/g, "<code>$1</code>")
      .replace(/\n/g, "<br>");
    return `<div class="editor-hover-block">${html}</div>`;
  }).join("");
}

function hideEditorHover(opts = {}) {
  clearTimeout(editorHoverTimer);
  if (!opts.keepSeq) {
    editorHoverSeq++;
  }
  clearEditorHoverMark();
  if (editorHoverEl) {
    editorHoverEl.style.display = "none";
  }
}

function clearEditorHoverMark() {
  if (editorHoverMark) {
    editorHoverMark.clear();
    editorHoverMark = null;
  }
}

async function formatProvider() {
  const provider = currentProvider();
  if (!provider) {
    setStatus("provider is empty");
    return;
  }
  const res = await fetch("/api/editor/format", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ provider, content: editorValue() })
  });
  const data = await res.json();
  if (!res.ok || !data.ok) {
    setStatus(data);
    return;
  }
  setEditorValue(data.content || "");
  setStatus({ ok: true, provider: data.provider, target_file: data.target_file, formatted: true });
  await runEditorAnalysis();
}

function escapeSingleQuote(v) {
  return String(v || "").replace(/'/g, "'\"'\"'");
}

function normalizeBaseURL(v) {
  const raw = String(v || "").trim();
  if (!raw) {
    return "http://127.0.0.1:3300";
  }
  return raw.replace(/\/+$/, "");
}

function testRequestFor(api, model) {
  const normalizedModel = (model || "").trim() || "gpt-4o-mini";
  if (api === "responses") {
    return {
      path: "/v1/responses",
      body: {
        model: normalizedModel,
        input: [
          {
            role: "user",
            content: "hi"
          }
        ]
      }
    };
  }
  if (api === "embeddings") {
    return {
      path: "/v1/embeddings",
      body: {
        model: normalizedModel,
        input: "hello"
      }
    };
  }
  if (api === "claude.messages") {
    return {
      path: "/v1/messages",
      body: {
        model: normalizedModel,
        max_tokens: 64,
        messages: [
          { role: "user", content: "hello" }
        ]
      }
    };
  }
  return {
    path: "/v1/chat/completions",
    body: {
      model: normalizedModel,
      messages: [
        { role: "user", content: "hello" }
      ]
    }
  };
}

function buildOnrTokenKey(rawK, rawUK, provider, model) {
  const params = new URLSearchParams();
  if (rawK) {
    params.set("k", rawK);
  }
  if (rawUK) {
    params.set("uk", rawUK);
  }
  if (provider) {
    params.set("p", provider);
  }
  if (model) {
    params.set("m", model);
  }
  return `onr:v1?${params.toString()}`;
}

function buildTestContext() {
  const provider = currentProvider();
  if (!provider) {
    setStatus("provider is empty");
    return null;
  }
  const api = String(testApiEl.value || "chat.completions").trim();
  const model = String(testModelEl.value || "").trim();
  const k = String(onrKEl.value || "").trim();
  const uk = String(onrUKEl.value || "").trim();
  if (!k && !uk) {
    setStatus("k or uk is required.");
    return null;
  }
  const baseURL = normalizeBaseURL(onrBaseUrlEl.value);
  const req = testRequestFor(api, model);
  const onrTokenKey = buildOnrTokenKey(k, uk, provider, String(req.body.model || "").trim());
  const payload = JSON.stringify(req.body);
  return {
    provider,
    k,
    uk,
    baseURL,
    path: req.path,
    payload,
    authorization: `Bearer ${onrTokenKey}`
  };
}

function generateCurl() {
  const ctx = buildTestContext();
  if (!ctx) {
    return;
  }

  const curl = [
    `curl -sS ${ctx.baseURL}${ctx.path} \\`,
    `  -H 'Authorization: ${escapeSingleQuote(ctx.authorization)}' \\`,
    `  -H 'Content-Type: application/json' \\`,
    `  -H 'x-onr-provider: ${escapeSingleQuote(ctx.provider)}' \\`,
    `  -d '${escapeSingleQuote(ctx.payload)}'`
  ].join("\n");

  curlOutputEl.value = curl;
  if (!ctx.k && ctx.uk) {
    setStatus("Generated token with uk only. If auth.token_key.allow_byok_without_k=false, this request will be rejected.");
  }
}

async function runRequest() {
  const ctx = buildTestContext();
  if (!ctx) {
    return;
  }
  execOutputEl.textContent = "Running request...";
  const res = await fetch("/api/test/request", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({
      base_url: ctx.baseURL,
      path: ctx.path,
      authorization: ctx.authorization,
      provider: ctx.provider,
      payload: ctx.payload
    })
  });
  const data = await res.json();
  if (!res.ok || !data.ok) {
    execOutputEl.textContent = JSON.stringify(data, null, 2);
    setStatus(data);
    return;
  }
  execOutputEl.textContent = formatCurlLikeResponse(data, true);
  const rid = extractRequestID(data.headers || {});
  if (rid) {
    requestIdInputEl.value = rid;
  }
  if (!ctx.k && ctx.uk) {
    setStatus("Request executed with uk only. If auth.token_key.allow_byok_without_k=false, this request will be rejected.");
  }
}

function formatCurlLikeResponse(data, withHeaders) {
  const body = String(data.body || "");
  if (!withHeaders) {
    return body;
  }
  const statusCode = Number(data.status || 0);
  const headers = data.headers || {};
  const keys = Object.keys(headers).sort((a, b) => a.localeCompare(b));
  const headerLines = keys.map((k) => `${k}: ${headers[k]}`);
  const statusLine = statusCode > 0 ? `HTTP/1.1 ${statusCode}` : "HTTP/1.1";
  return [statusLine, ...headerLines, "", body].join("\n");
}

function headerValue(headers, key) {
  const target = String(key || "").trim().toLowerCase();
  if (!target) {
    return "";
  }
  for (const [k, v] of Object.entries(headers || {})) {
    if (String(k || "").toLowerCase() === target) {
      return String(v || "").trim();
    }
  }
  return "";
}

function extractRequestID(headers) {
  return headerValue(headers, "x-onr-request-id") || headerValue(headers, "x-request-id");
}

async function loadDumpByRequestID() {
  const rid = String(requestIdInputEl.value || "").trim();
  if (!rid) {
    setStatus("request_id is empty.");
    return;
  }
  dumpOutputEl.textContent = "Loading dump...";
  const res = await fetch("/api/dumps/by-request-id?request_id=" + encodeURIComponent(rid));
  const data = await res.json();
  if (!res.ok || !data.ok) {
    dumpOutputEl.textContent = "";
    setStatus(data);
    return;
  }
  dumpOutputEl.textContent = String(data.content || "");
  setStatus({
    ok: true,
    request_id: data.request_id || rid,
    file_name: data.file_name || "",
    path: data.path || "",
    truncated: !!data.truncated
  });
}

async function copyCurl() {
  const text = String(curlOutputEl.value || "").trim();
  if (!text) {
    setStatus("cURL is empty. Click Generate cURL first.");
    return;
  }
  try {
    await navigator.clipboard.writeText(text);
    setStatus("cURL copied.");
  } catch (_) {
    curlOutputEl.focus();
    curlOutputEl.select();
    setStatus("Clipboard API failed. Selected cURL text, copy manually.");
  }
}

async function refreshProviders() {
  const res = await fetch("/api/providers");
  const data = await res.json();
  if (!res.ok || !data.ok) {
    setStatus(data);
    return;
  }
  providerSelect.innerHTML = "";
  for (const p of data.providers || []) {
    const opt = document.createElement("option");
    opt.value = p;
    opt.textContent = p;
    providerSelect.appendChild(opt);
  }
  if ((data.providers || []).length > 0 && !providerInput.value) {
    providerInput.value = data.providers[0];
  }
}

async function loadProvider() {
  const name = currentProvider();
  if (!name) {
    setStatus("provider is empty");
    return;
  }
  const res = await fetch("/api/provider?name=" + encodeURIComponent(name));
  const data = await res.json();
  setStatus(data);
  if (res.ok && data.ok) {
    setEditorValue(data.content || "");
    await runEditorAnalysis();
  }
}

async function validateProvider() {
  const body = { provider: currentProvider(), content: editorValue() };
  const res = await fetch("/api/providers/validate", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(body)
  });
  const data = await res.json();
  setStatus(data);
  if (res.ok) {
    await refreshProviders();
  }
}

async function saveProvider() {
  const body = { provider: currentProvider(), content: editorValue() };
  const res = await fetch("/api/providers/save", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(body)
  });
  const data = await res.json();
  setStatus(data);
  if (res.ok) {
    await refreshProviders();
  }
}

providerSelect.addEventListener("change", () => {
  providerInput.value = providerSelect.value;
  queueEditorAnalysis();
});
providerInput.addEventListener("input", queueEditorAnalysis);
if (!editor) {
  contentEl.addEventListener("input", queueEditorAnalysis);
}
document.getElementById("loadBtn").addEventListener("click", loadProvider);
document.getElementById("validateBtn").addEventListener("click", validateProvider);
document.getElementById("formatBtn").addEventListener("click", formatProvider);
document.getElementById("saveBtn").addEventListener("click", saveProvider);
document.getElementById("genCurlBtn").addEventListener("click", generateCurl);
document.getElementById("runRequestBtn").addEventListener("click", runRequest);
document.getElementById("copyCurlBtn").addEventListener("click", copyCurl);
document.getElementById("loadDumpBtn").addEventListener("click", loadDumpByRequestID);

refreshProviders()
  .then(() => runEditorAnalysis())
  .catch((err) => setStatus(String(err)));
