"use strict";

const $ = (id) => document.getElementById(id);
const token = new URLSearchParams(location.search).get("token") || "";
const withToken = (p) => (token ? p + (p.includes("?") ? "&" : "?") + "token=" + encodeURIComponent(token) : p);
const api = async (p, opts) => {
  const res = await fetch(withToken(p), opts);
  const body = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(body.error || res.statusText);
  return body;
};
const esc = (s) => String(s ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
const svgNS = "http://www.w3.org/2000/svg";
const el = (tag, attrs = {}) => { const n = document.createElementNS(svgNS, tag); for (const [k, v] of Object.entries(attrs)) n.setAttribute(k, v); return n; };
const outWord = { timeout: "hang", closed: "reset", mismatch: "mismatch", error: "error" };

let modelData = null;     // {nodes, edges, flows}
let findings = [];
let status = { running: false, ran: false, done: 0, total: 0 };
let selected = null;

/* control bytes rendered visibly */
function showBytes(s) {
  return String(s).replace(/[\x00-\x1f\x7f-\xff]/g, (c) => {
    if (c === "\r") return "\\r";
    if (c === "\n") return "\\n";
    if (c === "\t") return "\\t";
    return "\\x" + c.charCodeAt(0).toString(16).padStart(2, "0");
  });
}

function show(page) {
  document.querySelectorAll(".page").forEach((p) => { p.hidden = p.id !== "page-" + page; });
  document.querySelectorAll("nav a").forEach((a) => a.classList.toggle("on", a.dataset.page === page));
}

function route() {
  const page = (location.hash.split("/")[1] || "overview");
  const pages = ["overview", "findings", "model", "mutators", "about"];
  const target = pages.includes(page) ? page : "overview";
  show(target);
  ({ overview: renderOverview, findings: renderFindings, model: renderModel, mutators: renderMutators, about: () => {} })[target]();
}

/* ---------- overview ---------- */

function renderOverview() {
  if (modelData) drawGraph($("graph"), modelData, litNodes(), false);
  updateScope();
  $("t-cases").textContent = status.total || (status.ran ? status.done : 0);
  const c = status.counts || {};
  $("t-hang").textContent = c.timeout || 0;
  $("t-reset").textContent = c.closed || 0;
  $("t-mismatch").textContent = c.mismatch || 0;
  $("t-baseline").textContent = !status.ran ? "—" : status.baseline === false ? "failed" : "passed";
  $("t-baseline").style.color = status.baseline === false ? "var(--hot)" : status.baseline ? "var(--phos)" : "";
}

function updateScope() {
  const s = $("scope-state");
  const total = status.total || 0, done = status.done || 0;
  if (status.running) {
    s.className = "scope-state running"; s.textContent = "running";
  } else if (!status.ran) {
    s.className = "scope-state idle"; s.textContent = "idle";
  } else if (status.baseline === false) {
    s.className = "scope-state failed"; s.textContent = "baseline failed";
  } else {
    s.className = "scope-state done"; s.textContent = "done";
  }
  const pct = total ? Math.round((done / total) * 100) : (status.running ? 5 : 0);
  $("progress-fill").style.width = pct + "%";
  $("progress-text").textContent = total ? `${done} / ${total}` : status.running ? "starting" : "";
  $("lede-sub").textContent = !status.ran
    ? "One mutation at a time, watching for a hang, a reset or a broken reply."
    : status.baseline === false
      ? "The target did not complete the baseline flows. Check it is running and speaks this protocol."
      : status.running
        ? `Sending case ${done} of ${total}.`
        : `${total} cases sent. ${findings.length} findings.`;
}

function litNodes() {
  const lit = {};
  for (const f of findings) {
    const sev = f.outcome;
    const cur = lit[f.step];
    if (!cur || weight(sev) > weight(cur)) lit[f.step] = sev;
  }
  return lit;
}
function weight(o) { return { closed: 3, timeout: 2, mismatch: 1 }[o] || 0; }

/* ---------- the state graph ---------- */

function drawGraph(svg, data, lit, big) {
  svg.replaceChildren();
  const W = svg.clientWidth || 800;
  const H = big ? 360 : 300;
  svg.setAttribute("viewBox", `0 0 ${W} ${H}`);
  svg.style.height = H + "px";

  // layered layout: connect at left, then columns by order of first appearance
  const order = data.nodes;
  const depth = {}; order.forEach((n, i) => { depth[n] = 0; });
  for (const n of order) depth[n] = n === "connect" ? 0 : depth[n];
  // compute depth from edges
  let changed = true, guard = 0;
  while (changed && guard++ < 50) {
    changed = false;
    for (const e of data.edges) {
      if (depth[e.to] < depth[e.from] + 1) { depth[e.to] = depth[e.from] + 1; changed = true; }
    }
  }
  const cols = {};
  for (const n of order) (cols[depth[n]] ||= []).push(n);
  const maxDepth = Math.max(...Object.keys(cols).map(Number));
  const colW = (W - 40) / (maxDepth + 1);
  const pos = {};
  for (const [d, nodes] of Object.entries(cols)) {
    const rowH = H / (nodes.length + 1);
    nodes.forEach((n, i) => { pos[n] = { x: 20 + Number(d) * colW + colW / 2, y: rowH * (i + 1) }; });
  }

  const defs = el("defs");
  const marker = el("marker", { id: "ah", markerWidth: 8, markerHeight: 8, refX: 7, refY: 4, orient: "auto" });
  marker.append(el("path", { d: "M0 0 L8 4 L0 8 Z", class: "edge-head" }));
  defs.append(marker); svg.append(defs);

  for (const e of data.edges) {
    const a = pos[e.from], b = pos[e.to];
    if (!a || !b) continue;
    const midx = (a.x + b.x) / 2;
    svg.append(el("path", { d: `M ${a.x + 34} ${a.y} C ${midx} ${a.y}, ${midx} ${b.y}, ${b.x - 38} ${b.y}`, class: "edge", "marker-end": "url(#ah)" }));
  }
  for (const n of order) {
    const p = pos[n]; if (!p) continue;
    const g = el("g", { class: n === "connect" ? "node-connect" : "" });
    const litClass = lit[n] ? " lit-" + lit[n] : "";
    const w = 72, h = 30;
    const rect = el("rect", { x: p.x - w / 2, y: p.y - h / 2, width: w, height: h, rx: 8, class: "node-rect" + litClass });
    if (lit[n] && !matchMedia("(prefers-reduced-motion: reduce)").matches) {
      const pulse = el("animate", { attributeName: "opacity", values: "1;0.55;1", dur: "2s", repeatCount: "indefinite" });
      rect.append(pulse);
    }
    g.append(rect);
    const label = el("text", { x: p.x, y: p.y + 4, "text-anchor": "middle", class: "node-label" });
    label.textContent = n;
    g.append(label);
    svg.append(g);
  }
}

/* placeholders */
let outFilter = "";

function renderFindings() {
  const rows = findings.filter((f) => f.flow && (!outFilter || f.outcome === outFilter));
  const body = $("rows");
  body.innerHTML = rows.length ? rows.map((f, i) => `
    <tr data-i="${findings.indexOf(f)}" class="${selected === f ? "sel" : ""}" tabindex="0">
      <td><span class="badge ${f.outcome}">${outWord[f.outcome] || f.outcome}</span></td>
      <td>${esc(f.flow)} / <b>${esc(f.step)}</b></td>
      <td class="tgt">${esc(f.target)}</td>
      <td class="mut">${esc(f.mutation)}</td>
    </tr>`).join("") : `<tr><td colspan="4" class="empty" style="padding:28px;text-align:center;color:var(--dim)">${status.ran ? "Nothing here — the target held up." : "Run a campaign to see findings."}</td></tr>`;
  if (selected && !rows.includes(selected)) { selected = null; renderDetail(); }
}

function renderDetail() {
  const d = $("detail");
  if (!selected) { d.innerHTML = `<p class="empty">Select a finding to see its replay.</p>`; return; }
  const f = selected;
  const repro = (f.repro || []).map((line, i) => {
    const marked = i === f.repro.length - 1 ? `<span class="inj">${esc(showBytes(line))}</span>` : esc(showBytes(line));
    return marked;
  }).join("\n");
  d.innerHTML = `
    <h3>${esc(f.flow)} / ${esc(f.step)}</h3>
    <p class="meta">The target <b>${outWord[f.outcome] || f.outcome}d</b> when <code>${esc(f.target)}</code> was set to the <b>${esc(f.mutation)}</b> variant.</p>
    <div class="label">Replay — send these lines in order</div>
    <pre>${repro}</pre>
    ${f.reply ? `<div class="label">Last reply</div><div class="reply">${esc(showBytes(f.reply))}</div>` : `<div class="label">Last reply</div><div class="reply" style="color:var(--dim)">— the connection ${f.outcome === "timeout" ? "went silent" : "was dropped"} —</div>`}`;
}

function renderModel() {
  if (modelData) drawGraph($("graph-big"), modelData, litNodes(), true);
  $("flows").innerHTML = (modelData.flows || []).map((flow) => `
    <section class="flow"><h2>${esc(flow.name)}</h2><ol>` +
    flow.steps.map((st) => {
      const fields = Object.entries(st.fields || {}).map(([n, f]) => `${n}:${f.type}`).join("  ");
      return `<li><span class="sname">${esc(st.name)}</span><span><span class="send">${esc(st.send)}</span>${fields ? `<div class="fields">${esc(fields)}</div>` : ""}</span></li>`;
    }).join("") + `</ol></section>`).join("");
}

function renderMutators() {
  $("mutators").innerHTML = (window.frayMutators || []).map((m) =>
    `<article class="mut-card"><h3>${esc(m.Name || m.name)}</h3><p>${esc(m.Describe || m.describe)}</p></article>`).join("");
}

/* ---------- run + stream ---------- */

async function startRun() {
  $("run").setAttribute("aria-busy", "true");
  $("run").textContent = "Running";
  findings = []; status.counts = {}; status.running = true; status.ran = true; status.done = 0;
  updatePip(); renderOverview();
  try {
    await api("/api/v1/run", { method: "POST" });
  } catch (err) {
    $("lede-sub").textContent = "Could not start: " + err.message;
  }
}

function onUpdate(u) {
  status.done = u.done; if (u.total) status.total = u.total;
  if (u.finding && u.finding.flow) {
    findings.unshift(u.finding);
    status.counts = status.counts || {};
    status.counts[u.finding.outcome] = (status.counts[u.finding.outcome] || 0) + 1;
    updatePip();
  }
  if (!$("page-overview").hidden) renderOverview();
  if (!$("page-findings").hidden) renderFindings();
}

function updatePip() {
  const pip = $("pip");
  if (findings.length) { pip.hidden = false; pip.textContent = findings.length; } else pip.hidden = true;
}

async function syncStatus() {
  try {
    status = await api("/api/v1/status");
    if (status.ran) { findings = await api("/api/v1/findings"); updatePip(); }
  } catch (err) { $("subject").textContent = "unreachable"; return; }
  if (!status.running) { $("run").removeAttribute("aria-busy"); $("run").textContent = status.ran ? "Run again" : "Run campaign"; }
  route();
}

function openStream() {
  const es = new EventSource(withToken("/api/v1/stream"));
  es.onmessage = (m) => {
    let u; try { u = JSON.parse(m.data); } catch { return; }
    const wasRunning = status.running;
    onUpdate(u);
    // the engine sends a final flush with done==total; refresh status then
    if (status.total && u.done >= status.total) setTimeout(syncStatus, 150);
  };
}

async function init() {
  window.addEventListener("hashchange", route);
  window.addEventListener("resize", () => { if (modelData && !$("page-overview").hidden) renderOverview(); if (modelData && !$("page-model").hidden) renderModel(); });
  $("run").addEventListener("click", startRun);
  document.querySelectorAll("#out-seg button").forEach((b) => b.addEventListener("click", () => {
    outFilter = b.dataset.out;
    document.querySelectorAll("#out-seg button").forEach((x) => x.classList.toggle("on", x === b));
    renderFindings();
  }));
  const pick = (ev) => {
    const tr = ev.target.closest("tr[data-i]");
    if (tr && (ev.type === "click" || ev.key === "Enter")) { selected = findings[Number(tr.dataset.i)]; renderFindings(); renderDetail(); }
  };
  $("rows").addEventListener("click", pick);
  $("rows").addEventListener("keydown", pick);
  try {
    const [st, mdl] = await Promise.all([api("/api/v1/status"), api("/api/v1/model")]);
    status = st; modelData = mdl;
    window.frayMutators = await api("/api/v1/mutators");
    $("subject").textContent = st.target;
    $("build").textContent = `Fray ${st.version} · ${st.platform}`;
    if (st.ran) { findings = await api("/api/v1/findings"); updatePip(); }
  } catch (err) {
    $("lede-sub").textContent = "Could not reach the panel API: " + err.message;
  }
  openStream();
  route();
  if (status.running) { $("run").setAttribute("aria-busy", "true"); $("run").textContent = "Running"; }
}

init();
